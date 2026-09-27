package slack

// HTTP trace tests (Issue #273): the trace is off unless WithTrace turns it
// on, it writes one JSON line per HTTP request (every retry attempt and every
// redirect hop) with the waits around it, and it keeps no URL, header, token
// or body.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// traceWriter collects a trace and checks that each Write is one whole line.
type traceWriter struct {
	mu     sync.Mutex
	writes []string
}

func (w *traceWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes = append(w.writes, string(p))
	return len(p), nil
}

func (w *traceWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.writes, "")
}

// records decodes the trace, one record per Write.
func (w *traceWriter) records(t *testing.T) []TraceRecord {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	var recs []TraceRecord
	for _, line := range w.writes {
		if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
			t.Fatalf("trace write %q is not one line", line)
		}
		var rec TraceRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("trace line %q: %v", line, err)
		}
		recs = append(recs, rec)
	}
	return recs
}

// shortSleep sleeps a thousandth of each wait, so the trace measures real
// waits that the tests can tell apart without taking seconds.
func shortSleep(ctx context.Context, d time.Duration) error {
	return sleepCtx(ctx, d/1000)
}

func TestTraceIsOffByDefault(t *testing.T) {
	t.Parallel()

	c := New(testToken)
	if c.trace != nil {
		t.Fatal("trace is on without WithTrace")
	}
	if c.httpClient.Transport != nil {
		t.Fatalf("transport = %T, want the default transport", c.httpClient.Transport)
	}
	c = New(testToken, WithTransport(&scriptTransport{}))
	if _, ok := c.httpClient.Transport.(*scriptTransport); !ok {
		t.Fatalf("transport = %T, want the one WithTransport gave", c.httpClient.Transport)
	}
}

// TestTraceWritesOneLinePerAttempt: every attempt of a call or a download is
// one line, with the retry wait its failure caused and the pacing wait before
// the first attempt.
func TestTraceWritesOneLinePerAttempt(t *testing.T) {
	t.Parallel()

	tr := &scriptTransport{steps: []scriptStep{
		// auth.test: 503, 429 with Retry-After, a body that fails, then OK.
		{status: http.StatusServiceUnavailable},
		{status: http.StatusTooManyRequests, header: retryAfterHeader("2")},
		{status: http.StatusOK, body: `{"ok":tr`, readErr: io.ErrUnexpectedEOF},
		{status: http.StatusOK, body: authTestOK},
		// auth.test again, right after: paced.
		{status: http.StatusOK, body: authTestOK},
		// A download: the connection fails, then OK.
		{err: errors.New("connection reset by peer")},
		{status: http.StatusOK, body: "image-bytes"},
	}}
	w := &traceWriter{}
	c := New(testToken, WithSleeper(shortSleep), WithTransport(tr), WithTrace(w))
	ctx := context.Background()
	for range 2 {
		if _, err := c.AuthTest(ctx); err != nil {
			t.Fatalf("AuthTest: %v", err)
		}
	}
	if _, _, err := c.Download(WithAssetKind(ctx, "og_image"), publicURL, 0, io.Discard); err != nil {
		t.Fatalf("Download: %v", err)
	}

	recs := w.records(t)
	if len(recs) != len(tr.steps) {
		t.Fatalf("trace has %d lines, want one per request (%d):\n%s", len(recs), len(tr.steps), w)
	}
	type want struct {
		typ, method, kind, host string
		attempt, status         int
		err                     string
		bytes                   int64
		// Waits are a thousandth of what the client asked for (shortSleep):
		// backoff 1s, 2s, 4s ... and Retry-After plus up to 1s of jitter.
		minRetryWaitUS int64
		paced          bool
	}
	wants := []want{
		{typ: TraceAPI, method: "auth.test", host: "slack.com", attempt: 0, status: 503, minRetryWaitUS: 1000},
		{typ: TraceAPI, method: "auth.test", host: "slack.com", attempt: 1, status: 429, minRetryWaitUS: 2000},
		{typ: TraceAPI, method: "auth.test", host: "slack.com", attempt: 2, status: 200, err: "network", bytes: 8, minRetryWaitUS: 4000},
		{typ: TraceAPI, method: "auth.test", host: "slack.com", attempt: 3, status: 200, bytes: int64(len(authTestOK))},
		{typ: TraceAPI, method: "auth.test", host: "slack.com", attempt: 0, status: 200, bytes: int64(len(authTestOK)), paced: true},
		{typ: TraceDownload, kind: "og_image", host: "example.com", attempt: 0, err: "network", minRetryWaitUS: 1000},
		{typ: TraceDownload, kind: "og_image", host: "example.com", attempt: 1, status: 200, bytes: 11},
	}
	for i, got := range recs {
		want := wants[i]
		if got.Type != want.typ || got.Method != want.method || got.Kind != want.kind || got.Scheme != "https" || got.Host != want.host {
			t.Errorf("line %d labels = %s %q %q %s://%s, want %s %q %q https://%s",
				i, got.Type, got.Method, got.Kind, got.Scheme, got.Host, want.typ, want.method, want.kind, want.host)
		}
		if got.Attempt != want.attempt || got.Redirect != 0 || got.Status != want.status || got.Error != want.err || got.Bytes != want.bytes {
			t.Errorf("line %d = attempt %d redirect %d status %d error %q bytes %d, want attempt %d redirect 0 status %d error %q bytes %d",
				i, got.Attempt, got.Redirect, got.Status, got.Error, got.Bytes, want.attempt, want.status, want.err, want.bytes)
		}
		if got.DoneUS == nil {
			t.Errorf("line %d has no done time", i)
		}
		if (want.minRetryWaitUS == 0 && got.RetryWaitUS != 0) || got.RetryWaitUS < want.minRetryWaitUS {
			t.Errorf("line %d retry wait = %dµs, want at least %dµs (0 when none)", i, got.RetryWaitUS, want.minRetryWaitUS)
		}
		if want.paced != (got.PacingWaitUS > 0) {
			t.Errorf("line %d pacing wait = %dµs, want paced %v", i, got.PacingWaitUS, want.paced)
		}
		if i > 0 && got.Start.Before(recs[i-1].Start) {
			t.Errorf("line %d starts before line %d", i, i-1)
		}
	}
}

// TestTraceRedirectHops: each redirect hop is its own line within its attempt.
func TestTraceRedirectHops(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	served := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/avatar.png":
			http.Redirect(w, r, "/cdn/avatar.png", http.StatusFound)
		case "/cdn/avatar.png":
			mu.Lock()
			served++
			first := served == 1
			mu.Unlock()
			if first {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			fmt.Fprint(w, "avatar")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	w := &traceWriter{}
	c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(srv.Client().Transport), WithTrace(w))
	if _, _, err := c.Download(WithAssetKind(context.Background(), "avatar"), srv.URL+"/avatar.png", 0, io.Discard); err != nil {
		t.Fatalf("Download: %v", err)
	}

	recs := w.records(t)
	wants := []struct{ attempt, redirect, status int }{
		{0, 0, http.StatusFound},
		{0, 1, http.StatusServiceUnavailable},
		{1, 0, http.StatusFound},
		{1, 1, http.StatusOK},
	}
	if len(recs) != len(wants) {
		t.Fatalf("trace has %d lines, want %d:\n%s", len(recs), len(wants), w)
	}
	for i, want := range wants {
		got := recs[i]
		if got.Attempt != want.attempt || got.Redirect != want.redirect || got.Status != want.status || got.Kind != "avatar" {
			t.Errorf("line %d = attempt %d redirect %d status %d kind %q, want attempt %d redirect %d status %d kind avatar",
				i, got.Attempt, got.Redirect, got.Status, got.Kind, want.attempt, want.redirect, want.status)
		}
	}
	if recs[0].URLHash == recs[1].URLHash || recs[0].URLHash != recs[2].URLHash || recs[1].URLHash != recs[3].URLHash {
		t.Errorf("url hashes = %s %s %s %s, want one per URL", recs[0].URLHash, recs[1].URLHash, recs[2].URLHash, recs[3].URLHash)
	}
	if recs[3].Bytes != int64(len("avatar")) {
		t.Errorf("last line bytes = %d, want %d", recs[3].Bytes, len("avatar"))
	}
}

// TestTraceLeavesOutSecrets is the negative test of the trace: a private file
// URL with a query, the token in the Authorization header, and the workspace
// and channel in the API traffic never reach the trace.
func TestTraceLeavesOutSecrets(t *testing.T) {
	t.Parallel()

	const secretFileURL = "https://files.slack.com/files-pri/T0SECRET-F0SECRET/download/secret-report.pdf?pub_secret=QUERYSECRET"
	tr := &scriptTransport{steps: []scriptStep{
		{status: http.StatusOK, body: `{"ok":true,"url":"https://secret-workspace.slack.com/","team":"Secret Workspace","team_id":"T0SECRET","user":"secret-user"}`},
		{status: http.StatusOK, body: `{"ok":true,"messages":[],"channel":"C0SECRET"}`},
		{err: fmt.Errorf("dial tcp: lookup files.slack.com: no such host (%s)", secretFileURL)},
		{status: http.StatusOK, body: "secret report body"},
	}}
	w := &traceWriter{}
	c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(tr), WithTrace(w))
	ctx := context.Background()
	if _, err := c.AuthTest(ctx); err != nil {
		t.Fatalf("AuthTest: %v", err)
	}
	if _, err := c.call(ctx, "conversations.history", url.Values{"channel": {"C0SECRET"}}, nil); err != nil {
		t.Fatalf("conversations.history: %v", err)
	}
	if _, _, err := c.Download(WithAssetKind(ctx, "attachment"), secretFileURL, 0, io.Discard); err != nil {
		t.Fatalf("Download: %v", err)
	}

	// The requests did carry what the trace must leave out.
	reqs := tr.requests()
	if len(reqs) != 4 || reqs[3].auth != "Bearer "+testToken || reqs[3].url != secretFileURL {
		t.Fatalf("requests = %+v, want the download sent with the token", reqs)
	}
	trace := w.String()
	for _, secret := range []string{
		testToken, "Bearer", "Authorization", "authorization",
		"files-pri", "T0SECRET", "F0SECRET", "secret-report", "pub_secret", "QUERYSECRET",
		"Secret Workspace", "secret-workspace", "secret-user", "C0SECRET", "no such host",
	} {
		if strings.Contains(trace, secret) {
			t.Errorf("trace contains %q:\n%s", secret, trace)
		}
	}
	recs := w.records(t)
	if len(recs) != 4 || recs[3].Host != "files.slack.com" || recs[2].Error != "network" {
		t.Errorf("trace = %+v, want 4 lines ending with the download's two attempts", recs)
	}
}

// TestTracePhases: over a new TLS connection, the line has the connection's
// phases in order; the next request over HTTP/2 reuses the connection.
func TestTracePhases(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		http2 bool
		proto string
	}{
		{name: "http2", http2: true, proto: "HTTP/2.0"},
		{name: "http1", http2: false, proto: "HTTP/1.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "asset-body")
			}))
			srv.EnableHTTP2 = tc.http2
			srv.StartTLS()
			defer srv.Close()

			w := &traceWriter{}
			c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(srv.Client().Transport), WithTrace(w))
			for range 2 {
				if _, _, err := c.Download(context.Background(), srv.URL+"/asset.png", 0, io.Discard); err != nil {
					t.Fatalf("Download: %v", err)
				}
			}

			recs := w.records(t)
			if len(recs) != 2 {
				t.Fatalf("trace has %d lines, want 2:\n%s", len(recs), w)
			}
			first := recs[0]
			if first.Proto != tc.proto || first.Status != http.StatusOK || first.Bytes != int64(len("asset-body")) || first.ConnReused {
				t.Errorf("first line = %s %d, %d bytes, reused %v; want %s 200, %d bytes on a new connection",
					first.Proto, first.Status, first.Bytes, first.ConnReused, tc.proto, len("asset-body"))
			}
			// The URL has an IP address: no DNS lookup.
			if first.DNSStartUS != nil || first.DNSDoneUS != nil {
				t.Errorf("first line has DNS times for an IP address")
			}
			phases := []*int64{first.ConnectStartUS, first.ConnectDoneUS, first.TLSStartUS, first.TLSDoneUS, first.GotConnUS, first.FirstByteUS, first.DoneUS}
			for i, p := range phases {
				if p == nil {
					t.Fatalf("first line misses phase %d: %s", i, w)
				}
				if i > 0 && *p < *phases[i-1] {
					t.Errorf("first line phase %d at %dµs comes before phase %d at %dµs", i, *p, i-1, *phases[i-1])
				}
			}
			if recs[1].Proto != tc.proto {
				t.Errorf("second line proto = %s, want %s", recs[1].Proto, tc.proto)
			}
			// HTTP/1.1 may dial again when the first connection is not idle
			// yet; HTTP/2 always shares its connection.
			if tc.http2 && !recs[1].ConnReused {
				t.Errorf("second line did not reuse the HTTP/2 connection")
			}
		})
	}
}

func TestTraceURLHash(t *testing.T) {
	t.Parallel()

	a, b := newTracer(io.Discard), newTracer(io.Discard)
	h := a.urlHash(slackFileURL)
	if _, err := hex.DecodeString(h); err != nil || len(h) != 16 {
		t.Fatalf("urlHash = %q, want 16 hex digits", h)
	}
	if again := a.urlHash(slackFileURL); again != h {
		t.Errorf("urlHash changed for the same URL: %s then %s", h, again)
	}
	if other := a.urlHash(publicURL); other == h {
		t.Errorf("urlHash is %s for two URLs", h)
	}
	// Keyed per client: another client, or a plain hash of the URL, gives
	// another value, so a known URL cannot be looked up in the trace.
	if other := b.urlHash(slackFileURL); other == h {
		t.Errorf("urlHash is %s in two clients", h)
	}
	sum := sha256.Sum256([]byte(slackFileURL))
	if strings.HasPrefix(hex.EncodeToString(sum[:]), h) {
		t.Errorf("urlHash %s is the URL's plain SHA-256", h)
	}
}

// TestTraceRecordsUnlabeledRequest: a request that no call or download
// labeled (none does today) still gets its line, when its body is closed.
func TestTraceRecordsUnlabeledRequest(t *testing.T) {
	t.Parallel()

	tr := &scriptTransport{steps: []scriptStep{{status: http.StatusOK, body: "ok"}}}
	w := &traceWriter{}
	c := New(testToken, WithTransport(tr), WithTrace(w))
	req, err := http.NewRequest(http.MethodGet, publicURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	if w.String() != "" {
		t.Fatalf("trace written before the body was closed: %s", w)
	}
	resp.Body.Close()
	recs := w.records(t)
	if len(recs) != 1 || recs[0].Type != "" || recs[0].Host != "example.com" || recs[0].Bytes != 2 {
		t.Fatalf("trace = %+v, want one unlabeled line", recs)
	}
}

// TestTraceRun: the run line holds when the export started, in UTC, and how
// long it ran, and nothing else. Without a trace TraceRun writes nothing.
func TestTraceRun(t *testing.T) {
	t.Parallel()

	New(testToken).TraceRun(time.Now())

	w := &traceWriter{}
	c := New(testToken, WithTrace(w))
	start := time.Now().Add(-1500 * time.Millisecond)
	c.TraceRun(start)
	recs := w.records(t)
	if len(recs) != 1 {
		t.Fatalf("trace = %s, want one line", w)
	}
	if rec := recs[0]; rec.Type != TraceRun || !rec.Start.Equal(start) || rec.DoneUS == nil ||
		*rec.DoneUS < 1_500_000 || *rec.DoneUS > 60_000_000 {
		t.Fatalf("run line = %+v, want the start and about 1.5 s", rec)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(w.String()), &fields); err != nil || len(fields) != 3 ||
		!strings.HasSuffix(fmt.Sprint(fields["start"]), "Z") {
		t.Fatalf("run line %s: fields %v (%v), want start in UTC, type and done_us only", w, fields, err)
	}
}

// TestTraceTimeoutClass: a request that runs out the client's timeout is a
// timeout, whether it was waiting for the headers or reading the body. With
// the trace on, http.Client cancels a request both through its context and
// through Request.Cancel (it does not know traceTransport), and the transport
// reports whichever it notices first.
func TestTraceTimeoutClass(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stalled-body" {
			w.Header().Set("Content-Length", "10")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)

	w := &traceWriter{}
	c := New(testToken, WithSleeper(shortSleep), WithTransport(srv.Client().Transport), WithTrace(w))
	c.httpClient.Timeout = 20 * time.Millisecond
	// Each download waiting for the headers makes 6 attempts; a download
	// whose body stalls makes 1.
	for _, path := range []string{"/stalled", "/stalled", "/stalled", "/stalled-body", "/stalled-body", "/stalled-body",
		"/stalled-body", "/stalled-body", "/stalled-body", "/stalled-body", "/stalled-body", "/stalled-body"} {
		if _, _, err := c.Download(context.Background(), srv.URL+path, 0, io.Discard); err == nil {
			t.Fatalf("Download(%s) succeeded, want a timeout", path)
		}
	}
	recs := w.records(t)
	if len(recs) != 3*(maxRetries+1)+9 {
		t.Fatalf("trace has %d lines, want %d", len(recs), 3*(maxRetries+1)+9)
	}
	for i, rec := range recs {
		if rec.Error != "timeout" {
			t.Errorf("line %d (status %d) error = %q, want timeout", i, rec.Status, rec.Error)
		}
	}
}

func TestTraceRecordTimes(t *testing.T) {
	t.Parallel()

	us := func(v int64) *int64 { return &v }
	for _, tc := range []struct {
		name string
		rec  TraceRecord
		want TraceTimes
	}{
		{
			name: "complete",
			rec: TraceRecord{PacingWaitUS: 50, RetryWaitUS: 70,
				GotConnUS: us(100), FirstByteUS: us(300), DoneUS: us(1000)},
			want: TraceTimes{PacingWait: 50 * time.Microsecond, RetryWait: 70 * time.Microsecond,
				Connect: 100 * time.Microsecond, FirstByte: 200 * time.Microsecond, Transfer: 700 * time.Microsecond},
		},
		{
			name: "failed before the connection",
			rec:  TraceRecord{ConnectStartUS: us(10), DoneUS: us(400)},
			want: TraceTimes{Connect: 400 * time.Microsecond},
		},
		{
			name: "failed waiting for the response",
			rec:  TraceRecord{GotConnUS: us(100), DoneUS: us(900)},
			want: TraceTimes{Connect: 100 * time.Microsecond, FirstByte: 800 * time.Microsecond},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.rec.Times()
			if got != tc.want {
				t.Errorf("Times() = %+v, want %+v", got, tc.want)
			}
		})
	}

	a := TraceTimes{PacingWait: 1, RetryWait: 2, Connect: 3, FirstByte: 4, Transfer: 5}
	if sum := a.Add(a); sum != (TraceTimes{2, 4, 6, 8, 10}) || sum.Total() != 30 {
		t.Errorf("Add = %+v total %v, want each part doubled, total 30", sum, sum.Total())
	}
}

// TestTraceKeepsTheResult: with the trace on, the client returns what it
// returns without it.
func TestTraceKeepsTheResult(t *testing.T) {
	t.Parallel()

	const content = "asset body"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(srv.Client().Transport), WithTrace(io.Discard))
	var buf bytes.Buffer
	written, ct, err := c.Download(context.Background(), srv.URL+"/a.png", 0, &buf)
	if err != nil || written != int64(len(content)) || ct != "image/png" || buf.String() != content {
		t.Fatalf("Download = %d %q %v body %q, want %d image/png <nil> %q", written, ct, err, buf.String(), len(content), content)
	}
	written, _, err = c.Download(context.Background(), srv.URL+"/a.png", 4, io.Discard)
	if !errors.Is(err, ErrTooLarge) || written != 5 {
		t.Fatalf("Download over the limit = %d %v, want 5 and ErrTooLarge", written, err)
	}
}
