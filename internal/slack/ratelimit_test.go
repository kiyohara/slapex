package slack

// Tests of what the downloads add to the retry for their lanes (Issue #276,
// PF-04 of #272): a download of an asset that is not a Slack file fails at
// once when a 429 asks it to wait longer than downloadPublicMaxWait, while a
// Slack file and the Web API wait as long as they are asked, and the HTTP
// trace counts a download's wait for its lane among its waits. internal/lane
// and internal/output test the lanes themselves.

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"testing"
	"time"
)

// TestRetryAfterOverTheLimit: a 429 that asks to wait longer than
// downloadPublicMaxWait fails a download of an asset that is not a Slack file
// at once, without the wait or a notice; within the limit, or for a Slack
// file or a Web API call, the wait is waited out and the request retried. The
// limit holds the seconds the Retry-After asks for, not the jitter the wait
// adds to them: 60s is within it, and 61s over it.
func TestRetryAfterOverTheLimit(t *testing.T) {
	t.Parallel()

	download := func(url string) func(ctx context.Context, c *Client) error {
		return func(ctx context.Context, c *Client) error {
			_, _, err := c.Download(ctx, url, 0, io.Discard)
			return err
		}
	}
	cases := []struct {
		name       string
		run        func(ctx context.Context, c *Client) error
		ok         string // the body of the response after the 429
		retryAfter string
		// wantErr is a pattern of the error; "" for success after a wait of
		// wait (plus up to 1s of jitter).
		wantErr string
		wait    time.Duration
	}{
		{
			name:       "public asset over the limit",
			run:        download(publicURL),
			retryAfter: "120",
			wantErr:    `^rate limited \(429\): the server asks to wait 2m0s, over the 1m0s limit$`,
		},
		{
			name:       "public asset just over the limit",
			run:        download(publicURL),
			retryAfter: "61",
			wantErr:    `^rate limited \(429\): the server asks to wait 1m1s, over the 1m0s limit$`,
		},
		{
			name:       "public asset at the limit",
			run:        download(publicURL),
			retryAfter: "60",
			wait:       60 * time.Second,
		},
		{
			name:       "public asset within the limit",
			run:        download(publicURL),
			retryAfter: "59",
			wait:       59 * time.Second,
		},
		{
			name:       "Slack file",
			run:        download(slackFileURL),
			retryAfter: "120",
			wait:       120 * time.Second,
		},
		{
			name: "Web API",
			run: func(ctx context.Context, c *Client) error {
				_, err := c.AuthTest(ctx)
				return err
			},
			ok:         authTestOK,
			retryAfter: "120",
			wait:       120 * time.Second,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, tr, rec, logs := newScriptClient(
				scriptStep{status: http.StatusTooManyRequests, header: retryAfterHeader(tt.retryAfter)},
				scriptStep{status: http.StatusOK, body: tt.ok},
			)
			err := tt.run(context.Background(), c)
			requests, waits, lines := len(tr.requests()), rec.recorded(), logs.recorded()
			if tt.wantErr != "" {
				if re := regexp.MustCompile(tt.wantErr); err == nil || !re.MatchString(err.Error()) {
					t.Fatalf("err = %v, want it to match %s", err, re)
				}
				if requests != 1 || len(waits) != 0 || len(lines) != 0 {
					t.Errorf("requests = %d, waits = %v, log lines = %q, want 1 request and no wait or line", requests, waits, lines)
				}
			} else {
				if err != nil {
					t.Fatalf("err = %v, want success", err)
				}
				if requests != 2 || len(waits) != 1 || len(lines) != 1 {
					t.Fatalf("requests = %d, waits = %v, log lines = %q, want 2 requests, a wait and its notice", requests, waits, lines)
				}
				assertWaitIn(t, waits[0], tt.wait)
			}
			if n := tr.openBodies(); n != 0 {
				t.Errorf("%d response bodies left open", n)
			}
		})
	}
}

// TestTraceCountsLaneWaits: a download's wait for its lane is its pacing wait
// before its first request, and a part of its retry wait after a failed one.
func TestTraceCountsLaneWaits(t *testing.T) {
	t.Parallel()

	tr := &scriptTransport{steps: []scriptStep{
		{status: http.StatusServiceUnavailable},
		{status: http.StatusOK, body: "image-bytes"},
	}}
	w := &traceWriter{}
	rec := &sleepRecorder{}
	laneWaits := []time.Duration{3 * time.Second, 2 * time.Second}
	fakeLane := func(c *Client) {
		c.waitLane = func(context.Context, time.Duration) (time.Duration, error) {
			d := laneWaits[0]
			laneWaits = laneWaits[1:]
			return d, nil
		}
	}
	c := New(testToken, WithSleeper(rec.sleep), WithTransport(tr), WithTrace(w), fakeLane)
	if _, _, err := c.Download(context.Background(), publicURL, 0, io.Discard); err != nil {
		t.Fatalf("Download: %v", err)
	}

	recs := w.records(t)
	if len(recs) != 2 {
		t.Fatalf("trace has %d lines, want one per request (2):\n%s", len(recs), w)
	}
	// The recorder does not sleep: the backoff adds next to nothing.
	if got := recs[0].PacingWaitUS; got != (3 * time.Second).Microseconds() {
		t.Errorf("first request's pacing wait = %dus, want the lane's 3s", got)
	}
	if got := recs[0].RetryWaitUS; got < (2*time.Second).Microseconds() || got >= (3*time.Second).Microseconds() {
		t.Errorf("first request's retry wait = %dus, want the lane's 2s and the backoff's next to nothing", got)
	}
	if recs[1].PacingWaitUS != 0 || recs[1].RetryWaitUS != 0 {
		t.Errorf("second request's waits = %d/%dus, want none", recs[1].PacingWaitUS, recs[1].RetryWaitUS)
	}
}
