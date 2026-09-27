package slack

// Retry tests per outcome (Issue #192). They pin what the retry of the Web API
// calls and of Download does for each outcome of an attempt — network error,
// 429 with and without Retry-After, 5xx, unexpected status, failing body,
// cancellation — and that every response body is closed. A scripted transport
// answers the requests, so network errors and failing bodies can be injected
// without a server. The httptest-based tests in client_test.go cover the same
// policy over real connections.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	slackFileURL  = "https://files.slack.com/files-pri/T0EXAMPLE-F0EXAMPLE/download/report.pdf"
	publicURL     = "https://example.com/preview.png"
	userInfoAlice = `{"ok":true,"user":{"id":"U1","name":"alice"}}`
)

// scriptStep is one scripted answer: a transport error, or a response whose
// body can fail with readErr after its content instead of ending.
type scriptStep struct {
	err     error
	status  int
	header  http.Header
	body    string
	readErr error
	// onServe runs when the step answers a request (to cancel the context).
	onServe func()
}

// scriptedRequest is what one request carried.
type scriptedRequest struct {
	method string
	url    string
	auth   string
	body   string
}

// scriptTransport answers the n-th request with steps[n] and records the
// requests and the response bodies it handed out.
type scriptTransport struct {
	mu     sync.Mutex
	steps  []scriptStep
	reqs   []scriptedRequest
	bodies []*trackedBody
}

func (s *scriptTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var sent []byte
	if req.Body != nil {
		sent, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.reqs)
	s.reqs = append(s.reqs, scriptedRequest{
		method: req.Method,
		url:    req.URL.String(),
		auth:   req.Header.Get("Authorization"),
		body:   string(sent),
	})
	if n >= len(s.steps) {
		return nil, fmt.Errorf("unscripted request %d", n+1)
	}
	step := s.steps[n]
	if step.onServe != nil {
		step.onServe()
	}
	if step.err != nil {
		return nil, step.err
	}
	body := &trackedBody{r: strings.NewReader(step.body), readErr: step.readErr}
	s.bodies = append(s.bodies, body)
	header := step.header
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{StatusCode: step.status, Header: header, Body: body, Request: req}, nil
}

func (s *scriptTransport) requests() []scriptedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.reqs)
}

// openBodies counts the response bodies nobody closed.
func (s *scriptTransport) openBodies() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	open := 0
	for _, b := range s.bodies {
		if !b.isClosed() {
			open++
		}
	}
	return open
}

// trackedBody is a response body that records Close and returns readErr in
// place of io.EOF when set.
type trackedBody struct {
	r       io.Reader
	readErr error

	mu     sync.Mutex
	closed bool
}

func (b *trackedBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if err == io.EOF && b.readErr != nil {
		err = b.readErr
	}
	return n, err
}

func (b *trackedBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *trackedBody) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

// logRecorder collects the lines the client reports through Logf.
type logRecorder struct {
	mu    sync.Mutex
	lines []string
}

func (r *logRecorder) logf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

func (r *logRecorder) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.lines)
}

// newScriptClient returns a Client whose requests steps answer, recording its
// waits and log lines.
func newScriptClient(steps ...scriptStep) (*Client, *scriptTransport, *sleepRecorder, *logRecorder) {
	tr := &scriptTransport{steps: steps}
	rec := &sleepRecorder{}
	logs := &logRecorder{}
	c := New(testToken, WithSleeper(rec.sleep), WithTransport(tr))
	c.Logf = logs.logf
	return c, tr, rec, logs
}

// retryCaller is one of the two request paths that retry.
type retryCaller struct {
	name string
	// what names the request in the retry log lines.
	what string
	// errPrefix is what the path puts before a retry error.
	errPrefix string
	run       func(ctx context.Context, c *Client) error
}

var retryCallers = []retryCaller{
	{
		name:      "api",
		what:      "api auth.test",
		errPrefix: "slack api auth.test: ",
		run: func(ctx context.Context, c *Client) error {
			_, err := c.AuthTest(ctx)
			return err
		},
	},
	{
		name: "download",
		what: "download",
		run: func(ctx context.Context, c *Client) error {
			_, _, err := c.Download(ctx, slackFileURL, 0, io.Discard)
			return err
		},
	},
}

func retryAfterHeader(secs string) http.Header {
	return http.Header{"Retry-After": {secs}}
}

func TestRetryOutcomes(t *testing.T) {
	t.Parallel()

	ok := scriptStep{status: http.StatusOK, body: authTestOK}
	cases := []struct {
		name  string
		steps []scriptStep
		// wantErr is the error after the caller's prefix; "" means success.
		wantErr string
		// waits are the bases of the recorded waits (plus up to 1s of jitter).
		waits []time.Duration
		// logs are patterns of the log lines; %s is the caller's what.
		logs []string
	}{
		{
			name:  "network error then success",
			steps: []scriptStep{{err: errors.New("connection refused")}, ok},
			waits: []time.Duration{time.Second},
			logs:  []string{`^retrying %s in [12]s \(.*connection refused\)$`},
		},
		{
			name:  "429 without Retry-After then success",
			steps: []scriptStep{{status: http.StatusTooManyRequests}, ok},
			waits: []time.Duration{time.Second},
			logs:  []string{`^retrying %s in [12]s \(rate limited \(429\)\)$`},
		},
		{
			// The backoff after a Retry-After wait counts the attempt (2s before
			// the 2nd retry), not the backoffs so far.
			name: "429 with Retry-After, 5xx, then success",
			steps: []scriptStep{
				{status: http.StatusTooManyRequests, header: retryAfterHeader("7")},
				{status: http.StatusInternalServerError},
				ok,
			},
			waits: []time.Duration{7 * time.Second, 2 * time.Second},
			logs: []string{
				`^rate limited on %s, waiting [78]s as instructed by Slack$`,
				`^retrying %s in [23]s \(server error: HTTP 500\)$`,
			},
		},
		{
			name:    "unexpected status is not retried",
			steps:   []scriptStep{{status: http.StatusNotFound, body: "not found"}},
			wantErr: "unexpected HTTP 404",
		},
		{
			name:    "5xx until the retries run out",
			steps:   slices.Repeat([]scriptStep{{status: http.StatusServiceUnavailable}}, maxRetries+1),
			wantErr: "giving up after 5 retries: server error: HTTP 503",
			waits:   []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second},
			logs:    slices.Repeat([]string{`^retrying %s in \d+s \(server error: HTTP 503\)$`}, maxRetries),
		},
		{
			// The final 429 still waits out its Retry-After.
			name: "429 with Retry-After until the retries run out",
			steps: slices.Repeat([]scriptStep{
				{status: http.StatusTooManyRequests, header: retryAfterHeader("1")},
			}, maxRetries+1),
			wantErr: "giving up after 5 retries: rate limited (429)",
			waits:   slices.Repeat([]time.Duration{time.Second}, maxRetries+1),
			logs: slices.Repeat([]string{`^rate limited on %s, waiting [12]s as instructed by Slack$`},
				maxRetries+1),
		},
	}
	for _, caller := range retryCallers {
		for _, tt := range cases {
			t.Run(caller.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				c, tr, rec, logs := newScriptClient(tt.steps...)
				err := caller.run(context.Background(), c)
				if tt.wantErr == "" {
					if err != nil {
						t.Fatalf("err = %v, want success", err)
					}
				} else if want := caller.errPrefix + tt.wantErr; err == nil || err.Error() != want {
					t.Fatalf("err = %v, want %q", err, want)
				}
				if n := len(tr.requests()); n != len(tt.steps) {
					t.Errorf("requests = %d, want %d", n, len(tt.steps))
				}
				waits := rec.recorded()
				if len(waits) != len(tt.waits) {
					t.Fatalf("waits = %v, want %d waits", waits, len(tt.waits))
				}
				for i, base := range tt.waits {
					assertWaitIn(t, waits[i], base)
				}
				lines := logs.recorded()
				if len(lines) != len(tt.logs) {
					t.Fatalf("log lines = %q, want %d lines", lines, len(tt.logs))
				}
				for i, pattern := range tt.logs {
					re := regexp.MustCompile(fmt.Sprintf(pattern, regexp.QuoteMeta(caller.what)))
					if !re.MatchString(lines[i]) {
						t.Errorf("log line %d = %q, want it to match %s", i, lines[i], re)
					}
				}
				if n := tr.openBodies(); n != 0 {
					t.Errorf("%d response bodies left open", n)
				}
			})
		}
	}
}

// TestCallRetriesBodyReadError: the Web API reads the whole body before
// decoding it, so a read that fails part way is retried like a network error.
func TestCallRetriesBodyReadError(t *testing.T) {
	t.Parallel()

	c, tr, rec, logs := newScriptClient(
		scriptStep{status: http.StatusOK, body: `{"ok":tr`, readErr: io.ErrUnexpectedEOF},
		scriptStep{status: http.StatusOK, body: authTestOK},
	)
	if _, err := c.AuthTest(context.Background()); err != nil {
		t.Fatalf("AuthTest: %v", err)
	}
	if n := len(tr.requests()); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
	waits := rec.recorded()
	if len(waits) != 1 {
		t.Fatalf("waits = %v, want one backoff wait", waits)
	}
	assertWaitIn(t, waits[0], time.Second)
	re := regexp.MustCompile(`^retrying api auth\.test in [12]s \(unexpected EOF\)$`)
	if lines := logs.recorded(); len(lines) != 1 || !re.MatchString(lines[0]) {
		t.Errorf("log lines = %q, want one matching %s", lines, re)
	}
	if n := tr.openBodies(); n != 0 {
		t.Errorf("%d response bodies left open", n)
	}
}

// TestDownloadStreamErrorIsNotRetried: Download streams the body to w, so a
// read that fails part way is returned, not retried: part of the file has
// already been written.
func TestDownloadStreamErrorIsNotRetried(t *testing.T) {
	t.Parallel()

	c, tr, rec, logs := newScriptClient(
		scriptStep{status: http.StatusOK, body: "partial", readErr: io.ErrUnexpectedEOF},
	)
	var buf bytes.Buffer
	written, _, err := c.Download(context.Background(), slackFileURL, 0, &buf)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, want io.ErrUnexpectedEOF", err)
	}
	if written != int64(len("partial")) || buf.String() != "partial" {
		t.Errorf("written = %d body = %q, want %d / %q", written, buf.String(), len("partial"), "partial")
	}
	if n := len(tr.requests()); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
	if waits := rec.recorded(); len(waits) != 0 {
		t.Errorf("waits = %v, want none", waits)
	}
	if lines := logs.recorded(); len(lines) != 0 {
		t.Errorf("log lines = %q, want none", lines)
	}
	if n := tr.openBodies(); n != 0 {
		t.Errorf("%d response bodies left open", n)
	}
}

func TestDownloadSizeLimit(t *testing.T) {
	t.Parallel()

	const content = "0123456789"
	cases := []struct {
		name    string
		limit   int64
		written int64
		wantErr error
	}{
		{name: "no limit", limit: 0, written: 10},
		{name: "at the limit", limit: 10, written: 10},
		// Download reads one byte past the limit to tell that it is exceeded.
		{name: "over the limit", limit: 9, written: 10, wantErr: ErrTooLarge},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, tr, _, _ := newScriptClient(scriptStep{
				status: http.StatusOK,
				header: http.Header{"Content-Type": {"application/pdf"}},
				body:   content,
			})
			written, contentType, err := c.Download(context.Background(), slackFileURL, tt.limit, io.Discard)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if written != tt.written {
				t.Errorf("written = %d, want %d", written, tt.written)
			}
			if contentType != "application/pdf" {
				t.Errorf("contentType = %q, want application/pdf", contentType)
			}
			if n := len(tr.requests()); n != 1 {
				t.Errorf("requests = %d, want 1", n)
			}
			if n := tr.openBodies(); n != 0 {
				t.Errorf("%d response bodies left open", n)
			}
		})
	}
}

// TestRetryStopsWhenCanceled: a wait cut short by cancellation ends the
// request with the context's error, without the "giving up" wrapping.
func TestRetryStopsWhenCanceled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		step scriptStep
		wait time.Duration
	}{
		{name: "during backoff", step: scriptStep{status: http.StatusInternalServerError}, wait: time.Second},
		{
			name: "during Retry-After",
			step: scriptStep{status: http.StatusTooManyRequests, header: retryAfterHeader("3")},
			wait: 3 * time.Second,
		},
	}
	for _, caller := range retryCallers {
		for _, tt := range cases {
			t.Run(caller.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				step := tt.step
				step.onServe = cancel
				c, tr, rec, _ := newScriptClient(step)
				// Like the real sleeper, give up at once on a done context.
				c.sleep = func(ctx context.Context, d time.Duration) error {
					if err := rec.sleep(ctx, d); err != nil {
						return err
					}
					return ctx.Err()
				}
				err := caller.run(ctx, c)
				if want := caller.errPrefix + "context canceled"; err == nil || err.Error() != want {
					t.Fatalf("err = %v, want %q", err, want)
				}
				if !errors.Is(err, context.Canceled) {
					t.Errorf("err = %v, want it to wrap context.Canceled", err)
				}
				if n := len(tr.requests()); n != 1 {
					t.Errorf("requests = %d, want 1", n)
				}
				waits := rec.recorded()
				if len(waits) != 1 {
					t.Fatalf("waits = %v, want the one canceled wait", waits)
				}
				assertWaitIn(t, waits[0], tt.wait)
				if n := tr.openBodies(); n != 0 {
					t.Errorf("%d response bodies left open", n)
				}
			})
		}
	}
}

// TestRetriedRequestsResendTheRequest: every attempt sends the same request,
// the Web API form body and token included, and a download sends the token
// only to files.slack.com.
func TestRetriedRequestsResendTheRequest(t *testing.T) {
	t.Parallel()

	bearer := "Bearer " + testToken
	cases := []struct {
		name string
		body string
		run  func(ctx context.Context, c *Client) error
		want scriptedRequest
	}{
		{
			name: "api",
			body: userInfoAlice,
			run: func(ctx context.Context, c *Client) error {
				_, err := c.UserInfo(ctx, "U1")
				return err
			},
			want: scriptedRequest{method: http.MethodPost, url: apiBase + "users.info", auth: bearer, body: "user=U1"},
		},
		{
			name: "download from files.slack.com",
			body: "file body",
			run: func(ctx context.Context, c *Client) error {
				_, _, err := c.Download(ctx, slackFileURL, 0, io.Discard)
				return err
			},
			want: scriptedRequest{method: http.MethodGet, url: slackFileURL, auth: bearer},
		},
		{
			name: "download of a public asset",
			body: "image body",
			run: func(ctx context.Context, c *Client) error {
				_, _, err := c.Download(ctx, publicURL, 0, io.Discard)
				return err
			},
			want: scriptedRequest{method: http.MethodGet, url: publicURL},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, tr, _, _ := newScriptClient(
				scriptStep{status: http.StatusInternalServerError},
				scriptStep{status: http.StatusTooManyRequests, header: retryAfterHeader("1")},
				scriptStep{status: http.StatusOK, body: tt.body},
			)
			if err := tt.run(context.Background(), c); err != nil {
				t.Fatalf("request: %v", err)
			}
			reqs := tr.requests()
			if len(reqs) != 3 {
				t.Fatalf("requests = %d, want 3", len(reqs))
			}
			for i, got := range reqs {
				if got != tt.want {
					t.Errorf("request %d = %+v, want %+v", i+1, got, tt.want)
				}
			}
		})
	}
}
