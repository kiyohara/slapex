package main

// Tests of the rate-limited origins (Issue #276): the limits they keep, the
// 429s they answer with, the presets that have them, and the runs that meet
// them.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/kiyohara/slapex/internal/lane"
	"github.com/kiyohara/slapex/internal/output"
)

// limiterStep is a step of TestLimiter: after waiting for after, and
// resetting the limiter when reset is set, ask it once for each of want.
type limiterStep struct {
	after time.Duration
	reset bool
	want  []bool
}

// TestLimiter: a rate limit serves its burst at once, then as many requests a
// second as it allows; a passing refusal refuses every request in its window
// and none outside it; and a reset starts either afresh.
func TestLimiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			cfg   Origin
			steps []limiterStep
		}{
			{
				name: "token bucket",
				cfg:  Origin{LimitPerSec: 4, LimitBurst: 2},
				steps: []limiterStep{
					{want: []bool{true, true, false}},                                             // the burst
					{after: 250 * time.Millisecond, want: []bool{true, false}},                    // a request a quarter second
					{after: 2 * time.Second, want: []bool{true, true, false}},                     // never more than the burst
					{after: 100 * time.Millisecond, reset: true, want: []bool{true, true, false}}, // a full burst again
				},
			},
			{
				name: "passing refusal",
				cfg:  Origin{RefuseAfterMS: 100, RefuseForMS: 200},
				steps: []limiterStep{
					{want: []bool{true}},
					{after: 100 * time.Millisecond, want: []bool{false, false}},
					{after: 199 * time.Millisecond, want: []bool{false}},
					{after: time.Millisecond, want: []bool{true, true}},
					{reset: true, want: []bool{true}},
					{after: 100 * time.Millisecond, want: []bool{false}},
				},
			},
		} {
			l := &limiter{cfg: tc.cfg}
			l.reset()
			for i, step := range tc.steps {
				time.Sleep(step.after)
				if step.reset {
					l.reset()
				}
				var got []bool
				for range step.want {
					got = append(got, l.allow())
				}
				if !slices.Equal(got, step.want) {
					t.Errorf("%s, step %d: allowed %v, want %v", tc.name, i, got, step.want)
				}
			}
		}
	})
}

// TestRateLimitedOrigin: an origin answers a request it refuses with 429 and
// its Retry-After after its first-byte delay, and serves the asset once its
// refusal has passed.
func TestRateLimitedOrigin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg := Origin{Class: "other", HTTP2: true, FirstByteMS: 50, RefuseForMS: 1000, RetryAfterS: 2}
		o := &fakeOrigin{cfg: cfg, link: &link{}, limiter: &limiter{cfg: cfg}}
		o.limiter.reset()
		serve := func() (*httptest.ResponseRecorder, time.Duration) {
			start := time.Now()
			rec := httptest.NewRecorder()
			o.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/7/10", nil))
			return rec, time.Since(start)
		}

		rec, took := serve()
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "2" || took != 50*time.Millisecond {
			t.Errorf("in the refusal: %d, Retry-After %q after %s; want 429, Retry-After 2 after 50ms",
				rec.Code, rec.Header().Get("Retry-After"), took)
		}
		time.Sleep(time.Second)
		rec, took = serve()
		if rec.Code != http.StatusOK || rec.Body.Len() != 10 || took != 50*time.Millisecond {
			t.Errorf("after the refusal: %d, %d bytes after %s; want 200, 10 bytes after 50ms", rec.Code, rec.Body.Len(), took)
		}

		// Without a Retry-After to give, the 429 has none.
		cfg.RetryAfterS = 0
		o = &fakeOrigin{cfg: cfg, link: &link{}, limiter: &limiter{cfg: cfg}}
		o.limiter.reset()
		if rec, _ := serve(); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "" {
			t.Errorf("without Retry-After: %d, Retry-After %q; want 429 and none", rec.Code, rec.Header().Get("Retry-After"))
		}
	})
}

// TestRateLimitedWorkloads: the limited and spike presets are 160 thumbnails
// from the traced workload's files.slack.com, which keeps a rate limit that
// lasts and one that passes.
func TestRateLimitedWorkloads(t *testing.T) {
	for name, limit := range map[string]func(*Origin){
		"limited": func(o *Origin) { o.LimitPerSec, o.LimitBurst, o.RetryAfterS = 8, 8, 1 },
		"spike":   func(o *Origin) { o.RefuseAfterMS, o.RefuseForMS, o.RetryAfterS = 1500, 2000, 2 },
	} {
		w, err := loadWorkload(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := w.validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		want := tracedWorkload().Origins[0]
		limit(&want)
		if w.Name != name || len(w.Origins) != 1 || w.Origins[0] != want || !want.rateLimited() {
			t.Errorf("%s = %q with origins %+v, want its name and one origin %+v", name, w.Name, w.Origins, want)
		}
		if len(w.Assets) != 160 || slices.ContainsFunc(w.Assets, func(a Asset) bool {
			return a.Kind != output.KindUploadThumb || a.Bytes < 20*kib || a.Bytes > 76*kib
		}) {
			t.Errorf("%s has %d assets, want 160 thumbnails of 20 to 76 KiB", name, len(w.Assets))
		}
	}
}

// TestRateLimitedRun: the parallel strategies download every asset from an
// origin that refuses the first requests of a run, after the 429s it answers
// them with, and the report counts the 429s and tells how parallel restores
// the lane's limit.
func TestRateLimitedRun(t *testing.T) {
	t.Parallel()

	// The origin refuses the requests of the first second: the first ones
	// of both strategies get a 429, and the retries after its Retry-After of
	// a second do not.
	w := Workload{
		Name:    "refusing",
		Origins: []Origin{{Class: "other", HTTP2: true, FirstByteMS: 20, RefuseForMS: 1000, RetryAfterS: 1}},
	}
	for i := range 6 {
		w.Assets = append(w.Assets, Asset{Origin: 0, Kind: output.KindAvatar, Bytes: int64(i+1) * kib})
	}
	b := startBench(w)
	defer b.close()
	var results []result
	for _, name := range []string{"parallel", "parallel-275"} {
		res, _, err := b.run(context.Background(), strategyNamed(t, name), 1, lane.Defaults)
		if err != nil {
			t.Fatalf("%s: run: %v", name, err)
		}
		if res.saved != 6 || res.notSaved != 0 || res.limited == 0 || res.requests != 6+res.limited {
			t.Errorf("%s: result = %+v, want 6 assets saved, each after the 429s it got", name, res)
		}
		results = append(results, res)
	}

	report := func(limits lane.Limits, results ...result) string {
		var out bytes.Buffer
		writeReport(&out, b.w, limits, results)
		return out.String()
	}
	out := report(lane.Defaults, results...)
	for _, want := range []string{
		"; HTTP/2 origins 1, HTTP/1.1 origins 0; rate-limited origins 1.",
		" With parallel, a 429 halves its origin's limit, and every 4 responses after it give a place back.",
		"| Requests | 429s | New conns |",
		"| parallel (origin lanes that wait out 429s, from #276) | 1 |",
		"| parallel-275 (origin lanes, 429s per download, #275) | 1 |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report misses %q:\n%s", want, out)
		}
	}
	limits := lane.Defaults
	limits.RestoreAfter = 0
	if out := report(limits, results...); !strings.Contains(out, " With parallel, a 429 halves its origin's limit for good.") {
		t.Errorf("report with no restore misses that the limit stays halved:\n%s", out)
	}
	// The lanes of parallel-275 leave the limit alone.
	if out := report(lane.Defaults, results[1]); strings.Contains(out, "halves") {
		t.Errorf("report of parallel-275 alone tells how parallel restores the limit:\n%s", out)
	}
}
