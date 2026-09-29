package slack

// Download client tests (Issue #275, PF-03 of #272): the downloads go through
// a transport of their own and are not paced, a watchdog bounds the wait for
// the response headers and for each byte of the body in place of an overall
// timeout, and WithNotices takes a download's progress notices.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/lane"
)

func TestNewDownloadTransport(t *testing.T) {
	t.Parallel()

	tr := NewDownloadTransport()
	// A request that waits for a free stream can stall for good
	// (golang/go#70809): TestAssetsFetchOverHTTP in internal/output shows it.
	if tr.HTTP2 != nil && tr.HTTP2.StrictMaxConcurrentRequests {
		t.Errorf("HTTP2 = %+v, want no StrictMaxConcurrentRequests", tr.HTTP2)
	}
	if tr.MaxIdleConnsPerHost < lane.Defaults.PerOrigin() {
		t.Errorf("MaxIdleConnsPerHost = %d, want at least the lanes' %d", tr.MaxIdleConnsPerHost, lane.Defaults.PerOrigin())
	}
	// Otherwise it is http.DefaultTransport's: the proxy from the
	// environment, HTTP/2, and the dial and TLS handshake timeouts.
	def := http.DefaultTransport.(*http.Transport)
	if tr == def || tr.Proxy == nil || !tr.ForceAttemptHTTP2 || tr.TLSHandshakeTimeout != def.TLSHandshakeTimeout ||
		tr.IdleConnTimeout != def.IdleConnTimeout || tr.DialContext == nil {
		t.Errorf("transport = %+v, want a copy of http.DefaultTransport", tr)
	}

	c := New(testToken)
	if got, ok := c.dlClient.Transport.(*http.Transport); !ok || got == def || got.MaxIdleConnsPerHost != tr.MaxIdleConnsPerHost {
		t.Errorf("download transport = %#v, want NewDownloadTransport's", c.dlClient.Transport)
	}
	if c.httpClient.Transport != nil {
		t.Errorf("Web API transport = %#v, want http.DefaultTransport", c.httpClient.Transport)
	}
}

// TestDownloadsAreNotPaced: downloads one after another wait for nothing,
// while the Web API calls of one method keep their pacing.
func TestDownloadsAreNotPaced(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			fmt.Fprint(w, authTestOK)
			return
		}
		fmt.Fprint(w, "asset")
	}))
	defer srv.Close()

	c, rec := newTestClient(srv)
	for i := range 3 {
		if _, _, err := c.Download(context.Background(), fmt.Sprintf("%s/files/%d.png", srv.URL, i), 0, io.Discard); err != nil {
			t.Fatalf("Download: %v", err)
		}
	}
	if waits := rec.recorded(); len(waits) != 0 {
		t.Fatalf("waits after the downloads = %v, want none", waits)
	}
	for range 2 {
		if _, err := c.AuthTest(context.Background()); err != nil {
			t.Fatalf("AuthTest: %v", err)
		}
	}
	if waits := rec.recorded(); len(waits) != 1 || waits[0] <= 0 || waits[0] > methodPace {
		t.Fatalf("waits after two auth.test calls = %v, want one pacing wait up to %v", waits, methodPace)
	}
}

// watchServer serves /headers-never, which never answers, /interim-only,
// which sends an interim response (103 Early Hints) and then nothing,
// /headers-cut, which sends the start of its headers and then nothing
// (HTTP/1.1 only), /body-stalls, which sends its headers and part of its body
// and then nothing, and /trickle, which sends its 20 bytes one every 10ms. It
// counts the requests.
func watchServer(t *testing.T, http2 bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/headers-never":
			<-r.Context().Done()
		case "/interim-only":
			w.Header().Set("Link", "</style.css>; rel=preload; as=style")
			w.WriteHeader(http.StatusEarlyHints)
			<-r.Context().Done()
		case "/headers-cut":
			conn, buf, err := http.NewResponseController(w).Hijack()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer conn.Close()
			buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: image/png\r\n")
			buf.Flush()
			// Until the client closes the connection.
			io.Copy(io.Discard, conn)
		case "/body-stalls":
			w.Header().Set("Content-Length", "10")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "part")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/trickle":
			w.Header().Set("Content-Length", "20")
			w.WriteHeader(http.StatusOK)
			for range 20 {
				fmt.Fprint(w, "x")
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	srv.EnableHTTP2 = http2
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv, &requests
}

// TestDownloadHeaderWait: an attempt whose response headers do not come
// within the header wait fails, and is retried like a network error. The
// error names the wait, not the URL, and is a timeout.
func TestDownloadHeaderWait(t *testing.T) {
	t.Parallel()

	for _, http2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("http2=%v", http2), func(t *testing.T) {
			t.Parallel()

			srv, _ := watchServer(t, http2)
			// The attempts are counted as they are sent: the server may take
			// in the last one only after the client has given up on it.
			var attempts atomic.Int32
			tr := srv.Client().Transport
			rec := &sleepRecorder{}
			c := New(testToken, WithSleeper(rec.sleep), WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
				attempts.Add(1)
				return tr.RoundTrip(req)
			})))
			c.headerWait = 20 * time.Millisecond
			_, _, err := c.Download(context.Background(), srv.URL+"/headers-never", 0, io.Discard)
			if want := "giving up after 5 retries: Get: no response headers within 20ms"; err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Timeout() {
				t.Errorf("err = %#v, want a timeout", err)
			}
			if n := attempts.Load(); n != maxRetries+1 {
				t.Errorf("attempts = %d, want %d", n, maxRetries+1)
			}
			if waits := rec.recorded(); len(waits) != maxRetries {
				t.Errorf("waits = %v, want a backoff before each retry", waits)
			}
		})
	}
}

// TestDownloadHeaderWaitPastFirstByte: the wait for response headers lasts
// until the final response's headers are in. An interim response, or the
// start of the headers, does not end it.
func TestDownloadHeaderWaitPastFirstByte(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		path  string
		http2 bool
	}{
		{"/interim-only", false},
		{"/interim-only", true},
		{"/headers-cut", false},
	} {
		t.Run(fmt.Sprintf("%s,http2=%v", strings.TrimPrefix(tc.path, "/"), tc.http2), func(t *testing.T) {
			t.Parallel()

			srv, _ := watchServer(t, tc.http2)
			c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(srv.Client().Transport))
			c.headerWait = 20 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _, err := c.Download(ctx, srv.URL+tc.path, 0, io.Discard)
			if want := "giving up after 5 retries: Get: no response headers within 20ms"; err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}

// TestDownloadHeaderWaitSkipsRedirectConnect: the time a redirect's next
// request takes to get a connection does not count toward the wait for
// response headers (the transport's dial timeout bounds it), and that request
// has a wait of its own once written.
func TestDownloadHeaderWaitSkipsRedirectConnect(t *testing.T) {
	t.Parallel()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "asset")
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/asset.png", http.StatusFound)
	}))
	defer origin.Close()

	var dialer net.Dialer
	slow := target.Listener.Addr().String()
	tr := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == slow {
			time.Sleep(time.Second)
		}
		return dialer.DialContext(ctx, network, addr)
	}}
	defer tr.CloseIdleConnections()
	rec := &sleepRecorder{}
	c := New(testToken, WithSleeper(rec.sleep), WithTransport(tr))
	c.headerWait = 500 * time.Millisecond
	written, _, err := c.Download(context.Background(), origin.URL+"/asset.png", 0, io.Discard)
	if err != nil || written != 5 {
		t.Fatalf("Download = %d, %v; want the 5 bytes", written, err)
	}
	if waits := rec.recorded(); len(waits) != 0 {
		t.Errorf("waits = %v, want no retry", waits)
	}
}

// TestDownloadHeaderWaitBeatsLateResponse: a response that comes after the
// header wait canceled the attempt does not save it, and the HTTP trace
// records the attempt as a timeout. A server may answer the client's
// cancellation itself (a handler that returns once its request is canceled
// sends an empty 200), and net/http passes on a response that raced the
// cancellation.
func TestDownloadHeaderWaitBeatsLateResponse(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	w := &traceWriter{}
	c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTrace(w), WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts.Add(1)
		if trace := httptrace.ContextClientTrace(req.Context()); trace != nil && trace.WroteRequest != nil {
			trace.WroteRequest(httptrace.WroteRequestInfo{})
		}
		<-req.Context().Done()
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})))
	c.headerWait = 20 * time.Millisecond
	_, _, err := c.Download(context.Background(), publicURL, 0, io.Discard)
	if want := "giving up after 5 retries: Get: no response headers within 20ms"; err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if n := attempts.Load(); n != maxRetries+1 {
		t.Errorf("attempts = %d, want %d", n, maxRetries+1)
	}
	recs := w.records(t)
	if len(recs) != maxRetries+1 {
		t.Fatalf("trace has %d lines, want %d", len(recs), maxRetries+1)
	}
	for i, rec := range recs {
		if rec.Error != "timeout" {
			t.Errorf("line %d (status %d) error = %q, want timeout", i, rec.Status, rec.Error)
		}
	}
}

// TestDownloadStallWait: a body that goes the stall wait without a byte fails
// the download, which is not retried: part of it is written already.
func TestDownloadStallWait(t *testing.T) {
	t.Parallel()

	for _, http2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("http2=%v", http2), func(t *testing.T) {
			t.Parallel()

			srv, requests := watchServer(t, http2)
			c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(srv.Client().Transport))
			c.stallWait = 20 * time.Millisecond
			var buf strings.Builder
			written, _, err := c.Download(context.Background(), srv.URL+"/body-stalls", 0, &buf)
			if want := "download stalled: no data for 20ms"; err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
			if written != 4 || buf.String() != "part" {
				t.Errorf("written = %d %q, want the 4 bytes before the stall", written, buf.String())
			}
			if n := requests.Load(); n != 1 {
				t.Errorf("requests = %d, want 1", n)
			}
		})
	}
}

// TestDownloadStallWaitAfterLateWroteRequest: the HTTP/2 transport may report
// a request written only after its response came (the WroteRequest trace).
// That report does not put the wait for response headers back in place of
// the body's.
func TestDownloadStallWaitAfterLateWroteRequest(t *testing.T) {
	t.Parallel()

	c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		trace := httptrace.ContextClientTrace(req.Context())
		body := &lateReportBody{ctx: req.Context(), report: func() { trace.WroteRequest(httptrace.WroteRequestInfo{}) }}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, ContentLength: -1, Body: body, Request: req}, nil
	})))
	c.headerWait, c.stallWait = 5*time.Second, 20*time.Millisecond
	var buf strings.Builder
	written, _, err := c.Download(context.Background(), publicURL, 0, &buf)
	if want := "download stalled: no data for 20ms"; err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if written != 4 || buf.String() != "part" {
		t.Errorf("written = %d %q, want the 4 bytes before the stall", written, buf.String())
	}
}

// lateReportBody is a body whose first read reports the request written
// (report) and brings "part", and whose next read waits for the request's
// cancellation.
type lateReportBody struct {
	ctx    context.Context
	report func()
	read   bool
}

func (b *lateReportBody) Read(p []byte) (int, error) {
	if !b.read {
		b.read = true
		b.report()
		return copy(p, "part"), nil
	}
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (b *lateReportBody) Close() error { return nil }

// TestDownloadSlowBodyIsNotStalled: a body that keeps coming is not cut off,
// however long it takes: each byte starts the stall wait again.
func TestDownloadSlowBodyIsNotStalled(t *testing.T) {
	t.Parallel()

	for _, http2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("http2=%v", http2), func(t *testing.T) {
			t.Parallel()

			srv, _ := watchServer(t, http2)
			c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(srv.Client().Transport))
			c.headerWait, c.stallWait = 150*time.Millisecond, 150*time.Millisecond
			start := time.Now()
			written, _, err := c.Download(context.Background(), srv.URL+"/trickle", 0, io.Discard)
			if err != nil || written != 20 {
				t.Fatalf("Download = %d, %v; want 20 bytes", written, err)
			}
			if took := time.Since(start); took < 150*time.Millisecond {
				t.Errorf("the body took %v, want longer than the stall wait for the test to mean anything", took)
			}
		})
	}
}

// TestDownloadPublicTimeout: an attempt at an asset that is not a Slack file
// fails once it has taken the public timeout, however its body keeps coming,
// and is not retried: part of it is written already. An attempt at a Slack
// file has no such limit.
func TestDownloadPublicTimeout(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, url string
		wantErr   string
	}{
		{"public asset", publicURL, "download took longer than 50ms"},
		{"Slack file", slackFileURL, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var attempts atomic.Int32
			c := New(testToken, WithSleeper((&sleepRecorder{}).sleep), WithTransport(roundTripFunc(func(req *http.Request) (*http.Response, error) {
				attempts.Add(1)
				body := &trickleBody{ctx: req.Context(), n: 50, every: 10 * time.Millisecond}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, ContentLength: -1, Body: body, Request: req}, nil
			})))
			c.stallWait, c.publicTimeout = 5*time.Second, 50*time.Millisecond
			written, _, err := c.Download(context.Background(), tc.url, 0, io.Discard)
			if tc.wantErr == "" {
				if err != nil || written != 50 {
					t.Fatalf("Download = %d, %v; want all 50 bytes", written, err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Timeout() {
				t.Errorf("err = %#v, want a timeout", err)
			}
			if written >= 50 {
				t.Errorf("written = %d, want less than the body", written)
			}
			if n := attempts.Load(); n != 1 {
				t.Errorf("attempts = %d, want 1", n)
			}
		})
	}
}

// trickleBody brings its n bytes one at a time, one every every, until its
// request is canceled.
type trickleBody struct {
	ctx   context.Context
	n     int
	every time.Duration
}

func (b *trickleBody) Read(p []byte) (int, error) {
	if b.n == 0 {
		return 0, io.EOF
	}
	select {
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	case <-time.After(b.every):
	}
	b.n--
	return copy(p, "x"), nil
}

func (b *trickleBody) Close() error { return nil }

// TestWatchdog covers the watchdog's own rules: progress moves the deadline
// on, a disarmed watchdog does not fire, stop reports whether it fired, the
// body's wait stays once armed, and the limit comes whatever the progress.
func TestWatchdog(t *testing.T) {
	t.Parallel()

	timeout := &downloadTimeout{"timed out"}
	newWatchdog := func() (*watchdog, context.Context) {
		ctx, cancel := context.WithCancelCause(context.Background())
		return &watchdog{cancel: cancel}, ctx
	}

	wd, ctx := newWatchdog()
	wd.arm(200*time.Millisecond, timeout)
	for range 30 {
		time.Sleep(10 * time.Millisecond)
		wd.progress()
	}
	if ctx.Err() != nil {
		t.Fatalf("fired despite progress: %v", context.Cause(ctx))
	}
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("did not fire once the progress stopped")
	}
	if cause := context.Cause(ctx); cause != timeout || wd.stop() != timeout {
		t.Errorf("cause = %v, stop = %v; want the timeout it was armed with", cause, wd.stop())
	}

	wd, ctx = newWatchdog()
	wd.arm(10*time.Millisecond, timeout)
	wd.disarm()
	time.Sleep(30 * time.Millisecond)
	if ctx.Err() != nil {
		t.Errorf("a disarmed watchdog fired: %v", context.Cause(ctx))
	}
	if err := wd.stop(); err != nil {
		t.Errorf("stop = %v, want nil for a watchdog that did not fire", err)
	}
	wd.arm(10*time.Millisecond, timeout)
	time.Sleep(30 * time.Millisecond)
	if ctx.Err() != nil {
		t.Errorf("a stopped watchdog fired: %v", context.Cause(ctx))
	}

	// Once the body's wait is armed, arm and disarm leave it alone.
	stalled := &downloadTimeout{"stalled"}
	wd, ctx = newWatchdog()
	wd.armBody(10*time.Millisecond, stalled)
	wd.arm(time.Hour, timeout)
	wd.disarm()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the body's wait did not fire after arm and disarm")
	}
	if cause := context.Cause(ctx); cause != stalled {
		t.Errorf("cause = %v, want the body's", cause)
	}

	// The limit fires despite progress and a disarm, and stop ends it.
	tooLong := &downloadTimeout{"took too long"}
	wd, ctx = newWatchdog()
	wd.limit(50*time.Millisecond, tooLong)
	wd.armBody(time.Hour, stalled)
	wd.disarm()
	giveUp := time.After(5 * time.Second)
	for ctx.Err() == nil {
		wd.progress()
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Millisecond):
		case <-giveUp:
			t.Fatal("the limit did not fire")
		}
	}
	if cause := context.Cause(ctx); cause != tooLong || wd.stop() != tooLong {
		t.Errorf("cause = %v, stop = %v; want the limit's", cause, wd.stop())
	}
	wd, ctx = newWatchdog()
	wd.limit(50*time.Millisecond, tooLong)
	if err := wd.stop(); err != nil {
		t.Errorf("stop = %v, want nil before the limit", err)
	}
	time.Sleep(100 * time.Millisecond)
	if ctx.Err() != nil {
		t.Errorf("a stopped watchdog's limit fired: %v", context.Cause(ctx))
	}
}

// TestWithNoticesTakesTheNotices: the notices of a download made with a
// WithNotices context go to its function, and not to Logf.
func TestWithNoticesTakesTheNotices(t *testing.T) {
	t.Parallel()

	c, _, _, logs := newScriptClient(
		scriptStep{status: http.StatusServiceUnavailable},
		scriptStep{status: http.StatusOK, body: "asset"},
		scriptStep{status: http.StatusTooManyRequests, header: retryAfterHeader("3")},
		scriptStep{status: http.StatusOK, body: "asset"},
	)
	var mu sync.Mutex
	var held []string
	ctx := WithNotices(context.Background(), func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		held = append(held, fmt.Sprintf(format, args...))
	})
	if _, _, err := c.Download(ctx, publicURL, 0, io.Discard); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if len(held) != 1 || !strings.HasPrefix(held[0], "retrying download in ") || len(logs.recorded()) != 0 {
		t.Fatalf("held = %q, Logf = %q; want the retry notice held", held, logs.recorded())
	}
	if _, _, err := c.Download(context.Background(), publicURL, 0, io.Discard); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if lines := logs.recorded(); len(held) != 1 || len(lines) != 1 || !strings.HasPrefix(lines[0], "rate limited on download") {
		t.Fatalf("held = %q, Logf = %q; want the wait notice in Logf", held, lines)
	}
}
