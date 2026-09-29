package output

// Parallel fetch tests (Issue #275, PF-03 of #272): Fetch downloads through
// the Slack client in lanes by origin (internal/lane), passes the downloads'
// notices and warnings on in the plan's order whatever order they end in,
// stops when its context is canceled without leaving a temporary file, sends
// the token to Slack's file host only, and shares one connection per HTTP/2
// origin.

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/lane"
	"github.com/kiyohara/slapex/internal/slack"
)

// fetchTestToken is a fake token: these tests send their requests to fake
// transports and local servers only.
const fetchTestToken = "xoxb-fetch-test-token"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// fakeResponse is a response of status with body.
func fakeResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"image/png"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

// newFetchClient is a Slack client that sends every request through rt and
// retries without waiting, unless ctx is done.
func newFetchClient(rt http.RoundTripper) *slack.Client {
	return slack.New(fetchTestToken, slack.WithTransport(rt),
		slack.WithSleeper(func(ctx context.Context, _ time.Duration) error { return ctx.Err() }))
}

// endedDownloader tells ended the URL of each download that has returned.
type endedDownloader struct {
	Downloader
	ended chan<- string
}

func (d endedDownloader) Download(ctx context.Context, srcURL string, limit int64, w io.Writer) (int64, string, error) {
	n, contentType, err := d.Downloader.Download(ctx, srcURL, limit, w)
	d.ended <- srcURL
	return n, contentType, err
}

// TestHeldNoticesPassInPlanOrder: what a download reports is held until the
// downloads before it in the plan have passed theirs on, and is then passed on
// in the order it was reported; once the context is done, it is dropped.
func TestHeldNoticesPassInPlanOrder(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := NewAssets(ctx, &fakeDownloader{}, t.TempDir(), 0)
	var out []string
	a.Logf = func(format string, args ...any) { out = append(out, "warning: "+fmt.Sprintf(format, args...)) }
	a.Notef = func(format string, args ...any) { out = append(out, "notice: "+fmt.Sprintf(format, args...)) }

	h := newHeldNotices(a, 4)
	h.notef(2)("retry %d", 2)
	h.warnf(2)("failed %d", 2)
	h.done(2)
	h.notef(1)("retry %d", 1)
	h.done(1)
	if len(out) != 0 {
		t.Fatalf("passed on before the first download ended: %q", out)
	}
	h.warnf(0)("failed %d", 0)
	h.done(0)
	want := []string{"warning: failed 0", "notice: retry 1", "notice: retry 2", "warning: failed 2"}
	if !slices.Equal(out, want) {
		t.Fatalf("passed on = %q, want %q", out, want)
	}

	cancel()
	h.notef(3)("retry %d", 3)
	h.done(3)
	if !slices.Equal(out, want) {
		t.Fatalf("passed on after the context ended = %q, want nothing more than %q", out, want)
	}
}

// retryWait matches the wait of a retry notice, which has jitter.
var retryWait = regexp.MustCompile(`in [0-9]+s `)

// TestAssetsFetchPassesNoticesInPlanOrder: four downloads, one per origin, run
// at once through the Slack client and end in the reverse of the plan's order.
// Their retry notices and warnings come in the plan's order, each download's
// notices before its warning, as a serial fetch gives them, and each result
// is the one its download ended with.
func TestAssetsFetchPassesNoticesInPlanOrder(t *testing.T) {
	t.Parallel()

	// Each download is retried once, after a 5xx of its own, and the retry
	// gets the status the download is listed with.
	downloads := []struct {
		url           string
		kind          string
		first, status int
	}{
		{"https://a.example/1.png", KindAvatar, http.StatusServiceUnavailable, http.StatusOK},
		{"https://b.example/2.png", KindEmoji, http.StatusInternalServerError, http.StatusNotFound},
		{"https://c.example/3.png", KindAvatar, http.StatusBadGateway, http.StatusOK},
		{"https://d.example/4.png", KindAttachment, http.StatusGatewayTimeout, http.StatusForbidden},
	}
	release := map[string]chan struct{}{}
	for _, d := range downloads {
		release[d.url] = make(chan struct{})
	}
	var releaseMu sync.Mutex
	releaseOne := func(u string) {
		releaseMu.Lock()
		defer releaseMu.Unlock()
		select {
		case <-release[u]:
		default:
			close(release[u])
		}
	}
	// A failing test must not leave the downloads waiting.
	t.Cleanup(func() {
		for _, d := range downloads {
			releaseOne(d.url)
		}
	})

	retried := make(chan string, len(downloads))
	var mu sync.Mutex
	attempts := map[string]int{}
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		u := req.URL.String()
		mu.Lock()
		attempts[u]++
		n := attempts[u]
		mu.Unlock()
		for _, d := range downloads {
			if d.url != u {
				continue
			}
			if n == 1 {
				return fakeResponse(req, d.first, ""), nil
			}
			retried <- u
			<-release[u]
			return fakeResponse(req, d.status, "body of "+u), nil
		}
		return nil, fmt.Errorf("unexpected request")
	})
	ended := make(chan string, len(downloads))
	a := NewAssets(context.Background(), endedDownloader{Downloader: newFetchClient(rt), ended: ended}, t.TempDir(), 0)
	var outMu sync.Mutex
	var out []string
	a.Logf = func(format string, args ...any) {
		outMu.Lock()
		defer outMu.Unlock()
		out = append(out, "warning: "+fmt.Sprintf(format, args...))
	}
	a.Notef = func(format string, args ...any) {
		outMu.Lock()
		defer outMu.Unlock()
		out = append(out, "notice: "+retryWait.ReplaceAllString(fmt.Sprintf(format, args...), "in <d> "))
	}
	passedOn := func() []string {
		outMu.Lock()
		defer outMu.Unlock()
		return slices.Clone(out)
	}

	var plan []PlannedAsset
	for _, d := range downloads {
		plan = append(plan, PlannedAsset{Kind: d.kind, SourceURL: d.url})
	}
	fetched := make(chan struct{})
	go func() {
		defer close(fetched)
		a.Fetch(plan)
	}()
	wait := func(ch <-chan string, what string) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for %s", what)
		}
	}

	// The four retries are under way at once: the lanes run side by side.
	for range downloads {
		wait(retried, "the four downloads to run at once")
	}
	// They end last first. Nothing is passed on until the first has ended.
	for i, d := range slices.Backward(downloads) {
		releaseOne(d.url)
		wait(ended, d.url+" to end")
		if i > 0 {
			if got := passedOn(); len(got) != 0 {
				t.Fatalf("passed on before the plan's first download ended: %q", got)
			}
		}
	}
	select {
	case <-fetched:
	case <-time.After(10 * time.Second):
		t.Fatal("Fetch did not return")
	}

	want := []string{
		"notice: retrying download in <d> (server error: HTTP 503)",
		"notice: retrying download in <d> (server error: HTTP 500)",
		"warning: asset failed (emoji): unexpected HTTP 404",
		"notice: retrying download in <d> (server error: HTTP 502)",
		"notice: retrying download in <d> (server error: HTTP 504)",
		"warning: asset failed (attachment): unexpected HTTP 403",
	}
	if got := passedOn(); !slices.Equal(got, want) {
		t.Fatalf("passed on = %q\nwant %q", got, want)
	}
	for _, d := range downloads {
		if _, ok := a.Save(d.kind, d.url, AssetMeta{}); ok != (d.status == http.StatusOK) {
			t.Errorf("Save(%s) ok = %v, want %v", d.url, ok, d.status == http.StatusOK)
		}
		mu.Lock()
		n := attempts[d.url]
		mu.Unlock()
		if n != 2 {
			t.Errorf("%s was requested %d times, want 2", d.url, n)
		}
	}
}

// stalledBody sends part of a body, then tells streaming and sends nothing
// more until ctx ends.
type stalledBody struct {
	ctx       context.Context
	streaming chan<- struct{}
	sent      bool
}

func (b *stalledBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		return copy(p, "part"), nil
	}
	// The part is in the temporary file by now: the reader asks for more
	// once it has written what it got.
	b.streaming <- struct{}{}
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (b *stalledBody) Close() error { return nil }

// TestAssetsFetchStopsWhenCanceled: once the context is canceled, the
// downloads under way stop and remove their temporary files, the downloads not
// started get no request, and what the downloads reported is dropped.
func TestAssetsFetchStopsWhenCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests atomic.Int32
	streaming := make(chan struct{}, 8)
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		resp := fakeResponse(req, http.StatusOK, "")
		resp.Body = &stalledBody{ctx: req.Context(), streaming: streaming}
		return resp, nil
	})
	dir := t.TempDir()
	a := NewAssets(ctx, newFetchClient(rt), dir, 0)
	a.Lanes = lane.Limits{HTTP2: 1, HTTP1: 1, Total: 2, Large: 1}
	var reported atomic.Int32
	a.Logf = func(string, ...any) { reported.Add(1) }
	a.Notef = func(string, ...any) { reported.Add(1) }
	plan := []PlannedAsset{
		{Kind: KindAvatar, SourceURL: "https://a.example/1.png"},
		{Kind: KindAvatar, SourceURL: "https://b.example/2.png"},
		{Kind: KindAvatar, SourceURL: "https://c.example/3.png"},
	}
	fetched := make(chan struct{})
	go func() {
		defer close(fetched)
		a.Fetch(plan)
	}()
	for range 2 {
		select {
		case <-streaming:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for two downloads to stream")
		}
	}
	if tmp := tempFiles(t, dir); len(tmp) != 2 {
		t.Fatalf("temporary files while two downloads stream = %q, want two", tmp)
	}

	cancel()
	select {
	case <-fetched:
	case <-time.After(10 * time.Second):
		t.Fatal("Fetch did not return after the cancel")
	}
	if n := requests.Load(); n != 2 {
		t.Errorf("requests = %d, want 2: none for the download not started", n)
	}
	if files := collectFiles(t, dir); len(files) != 0 {
		t.Errorf("files left = %q, want none", files)
	}
	if n := reported.Load(); n != 0 {
		t.Errorf("%d notices and warnings passed on, want them dropped", n)
	}
}

// tempFiles are the names of the temporary files of the downloads in dir.
func tempFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "asset-") {
			names = append(names, e.Name())
		}
	}
	return names
}

// TestAssetsFetchSendsTokenOnlyToSlackFiles: the parallel fetch sends the
// token to Slack's file host, and to no other host
// (doc/guidelines/credential-scope-guidelines.md): not to Slack's public asset
// hosts, not to a third party, not to a host whose name only starts like
// Slack's, and not to the host a Slack file redirects to.
func TestAssetsFetchSendsTokenOnlyToSlackFiles(t *testing.T) {
	t.Parallel()

	const moved = "https://files.slack.com/files-pri/T0-F3/moved.png"
	const redirected = "https://cdn.example.net/moved.png"
	withToken := []string{
		"https://files.slack.com/files-pri/T0-F1/report.pdf",
		"https://files.slack.com/files-tmb/T0-F2/image_480.png",
		moved,
	}
	withoutToken := []string{
		"https://avatars.slack-edge.com/2026-01-01/1_abc_72.png",
		"https://emoji.slack-edge.com/T0/party/abc.gif",
		"https://secure.gravatar.com/avatar/0123456789abcdef.jpg?s=72",
		"https://example.com/unfurl.png",
		"https://files.slack.com.example.net/files-pri/T0-F4/lookalike.png",
	}
	var mu sync.Mutex
	auth := map[string][]string{} // the Authorization header of each request, by URL
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		auth[req.URL.String()] = append(auth[req.URL.String()], req.Header.Get("Authorization"))
		mu.Unlock()
		if req.URL.String() == moved {
			resp := fakeResponse(req, http.StatusFound, "")
			resp.Header.Set("Location", redirected)
			return resp, nil
		}
		return fakeResponse(req, http.StatusOK, "body of "+req.URL.String()), nil
	})
	a := NewAssets(context.Background(), newFetchClient(rt), t.TempDir(), 0)
	var plan []PlannedAsset
	for _, u := range append(slices.Clone(withToken), withoutToken...) {
		plan = append(plan, PlannedAsset{Kind: KindAttachment, SourceURL: u})
	}
	a.Fetch(plan)

	for _, u := range withToken {
		if got := auth[u]; len(got) != 1 || got[0] != "Bearer "+fetchTestToken {
			t.Errorf("Authorization to %s = %q, want the token once", u, got)
		}
	}
	for _, u := range append(withoutToken, redirected) {
		if got := auth[u]; len(got) != 1 || got[0] != "" {
			t.Errorf("Authorization to %s = %q, want one request without it", u, got)
		}
	}
	for _, p := range plan {
		if _, ok := a.Save(p.Kind, p.SourceURL, AssetMeta{}); !ok {
			t.Errorf("Save(%s) ok = false", p.SourceURL)
		}
	}
}

// TestAssetsFetchOverHTTP runs Fetch through the Slack client and the
// transport it downloads with (slack.NewDownloadTransport) against real
// servers, and every asset is saved:
//
//   - An HTTP/2 origin that allows as many streams as the lane runs downloads
//     gets one connection, which the downloads share up to the lane's limit.
//   - An HTTP/2 origin that allows one stream at a time gets more
//     connections, and does not stall the downloads (golang/go#70809: a
//     transport that waits for a free stream would).
//   - An HTTP/1.1 origin gets a connection per download, and runs up to the
//     lane's limit at a time. Its connections are not bounded here: net/http
//     completes a dial that a connection coming free overtook, and keeps the
//     connection.
func TestAssetsFetchOverHTTP(t *testing.T) {
	t.Parallel()

	limits := lane.Limits{HTTP2: 8, HTTP1: 3, Total: 64, Large: 4, LargeSize: 1 << 20}
	for _, tc := range []struct {
		name    string
		http2   bool
		streams int // the server's MaxConcurrentStreams; 0 for its default
		// wantConns bounds the connections, 0 leaving them unbounded, and
		// wantMost the downloads at a time.
		wantConns, wantMost int32
	}{
		{"HTTP/2", true, 0, 1, int32(limits.HTTP2)},
		{"HTTP/2 with one stream", true, 1, 0, int32(limits.HTTP2)},
		{"HTTP/1.1", false, 0, 0, int32(limits.HTTP1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var conns, inflight, most atomic.Int32
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := inflight.Add(1)
				defer inflight.Add(-1)
				for m := most.Load(); n > m && !most.CompareAndSwap(m, n); m = most.Load() {
				}
				time.Sleep(10 * time.Millisecond)
				w.Header().Set("Content-Type", "image/png")
				fmt.Fprintf(w, "%s %s", r.Proto, r.URL.Path)
			}))
			srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateNew {
					conns.Add(1)
				}
			}
			srv.Config.HTTP2 = &http.HTTP2Config{MaxConcurrentStreams: tc.streams}
			srv.EnableHTTP2 = tc.http2
			srv.StartTLS()
			defer srv.Close()

			tr := slack.NewDownloadTransport()
			// Trust the server's certificate, as the server's own client does.
			tr.TLSClientConfig = srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			defer tr.CloseIdleConnections()
			// A stalled transport fails the downloads here instead of the
			// test run.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			dir := t.TempDir()
			a := NewAssets(ctx, slack.New(fetchTestToken, slack.WithTransport(tr)), dir, 0)
			a.Lanes = limits
			var plan []PlannedAsset
			for i := range 24 {
				plan = append(plan, PlannedAsset{Kind: KindAvatar, SourceURL: fmt.Sprintf("%s/avatars/%d.png", srv.URL, i)})
			}
			a.Fetch(plan)

			if err := ctx.Err(); err != nil {
				t.Fatalf("the downloads stalled: %v", err)
			}
			for _, p := range plan {
				if _, ok := a.Save(p.Kind, p.SourceURL, AssetMeta{}); !ok {
					t.Errorf("Save(%s) ok = false", p.SourceURL)
				}
			}
			proto := "HTTP/1.1"
			if tc.http2 {
				proto = "HTTP/2.0"
			}
			for rel, body := range collectFiles(t, dir) {
				if !strings.HasPrefix(body, proto+" ") {
					t.Errorf("%s = %q, want it served over %s", rel, body, proto)
				}
			}
			if n := conns.Load(); tc.wantConns > 0 && n > tc.wantConns {
				t.Errorf("%d connections, want at most %d", n, tc.wantConns)
			}
			if n := most.Load(); n < 2 || n > tc.wantMost {
				t.Errorf("%d downloads at a time, want 2 to %d", n, tc.wantMost)
			}
		})
	}
}
