package output

// Rate limit tests of the parallel fetch (Issue #276, PF-04 of #272): a 429
// holds back the downloads of its origin, and only those, for the time its
// Retry-After asks, and a third-party host that asks for longer than a
// download may wait fails its downloads at once. The downloads go through the
// Slack client and net/http to a fake server in a synctest bubble, over
// in-memory connections, so that the requests and the waits run on the
// bubble's clock.

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// pipeServer serves an HTTP handler over in-memory connections (net.Pipe),
// on which a goroutine in a synctest bubble waits durably, unlike on a
// socket. Its transport dials every host to the server, an https one too:
// the transport takes the plain connection it dials for TLS as past its
// handshake, and speaks HTTP/1.1 over it.
type pipeServer struct {
	srv   *http.Server
	conns chan net.Conn
	done  chan struct{}
	close sync.Once
}

func startPipeServer(h http.Handler) *pipeServer {
	s := &pipeServer{
		srv:   &http.Server{Handler: h, ErrorLog: log.New(io.Discard, "", 0)},
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}
	go s.srv.Serve(pipeListener{s})
	return s
}

func (s *pipeServer) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	select {
	case s.conns <- server:
		return client, nil
	case <-s.done:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *pipeServer) transport() *http.Transport {
	return &http.Transport{DialContext: s.dial, DialTLSContext: s.dial}
}

// stop closes the server and its connections, which ends the goroutines of
// the server and of the transports that dialed it.
func (s *pipeServer) stop() { s.srv.Close() }

type pipeListener struct{ s *pipeServer }

func (l pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.s.conns:
		return c, nil
	case <-l.s.done:
		return nil, net.ErrClosed
	}
}

func (l pipeListener) Close() error {
	l.s.close.Do(func() { close(l.s.done) })
	return nil
}

func (l pipeListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// timedRequest is one request the server got: when, since the test started,
// and for which host.
type timedRequest struct {
	at   time.Duration
	host string
}

// rateLimitedHandler answers a request that limit picks with a 429 that asks
// to wait retryAfter seconds, after half a second, and any other request with
// a body, after a second, so that a 429 comes before the responses to the
// requests sent with it. It records the requests.
type rateLimitedHandler struct {
	start      time.Time
	retryAfter string
	limit      func(host string, at time.Duration) bool

	mu   sync.Mutex
	reqs []timedRequest
}

func (h *rateLimitedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	at := time.Since(h.start)
	h.mu.Lock()
	h.reqs = append(h.reqs, timedRequest{at: at, host: r.Host})
	limit := h.limit(r.Host, at)
	h.mu.Unlock()
	if limit {
		time.Sleep(time.Second / 2)
		w.Header().Set("Retry-After", h.retryAfter)
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	time.Sleep(time.Second)
	w.Header().Set("Content-Type", "image/png")
	io.WriteString(w, "body of "+r.URL.Path)
}

// requests are the requests so far, by host, in the order they came.
func (h *rateLimitedHandler) requests() map[string][]time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	byHost := map[string][]time.Duration{}
	for _, r := range h.reqs {
		byHost[r.host] = append(byHost[r.host], r.at)
	}
	return byHost
}

// fetchRateLimited fetches plan through a Slack client to a server of h, and
// returns what the fetch passed on, and when.
func fetchRateLimited(t *testing.T, h *rateLimitedHandler, plan []PlannedAsset) (*Assets, *timedLines) {
	t.Helper()
	srv := startPipeServer(h)
	defer srv.stop()
	tr := srv.transport()
	defer tr.CloseIdleConnections()
	a := NewAssets(context.Background(), slack.New(fetchTestToken, slack.WithTransport(tr)), t.TempDir(), 0)
	out := &timedLines{start: h.start}
	a.Notef = out.logf("notice: ")
	a.Logf = out.logf("warning: ")
	a.Fetch(plan)
	return a, out
}

// timedLines records the lines passed on, with when they were.
type timedLines struct {
	start time.Time

	mu    sync.Mutex
	lines []string
	at    []time.Duration
}

func (l *timedLines) logf(prefix string) func(string, ...any) {
	return func(format string, args ...any) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.lines = append(l.lines, prefix+fmt.Sprintf(format, args...))
		l.at = append(l.at, time.Since(l.start))
	}
}

func plannedOn(kind, prefix string, n int) []PlannedAsset {
	var plan []PlannedAsset
	for i := range n {
		plan = append(plan, PlannedAsset{Kind: kind, SourceURL: fmt.Sprintf("%s/%d.png", prefix, i)})
	}
	return plan
}

// TestAssetsFetchWaitsOutRateLimitByOrigin: once one download of an origin
// gets a 429, no download of that origin sends a request for the time its
// Retry-After asks, while the downloads of another origin go on. Every asset
// is saved in the end.
func TestAssetsFetchWaitsOutRateLimitByOrigin(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// The first request to a.example gets the 429. The lane of each
		// origin runs 6 at a time once its first request has a connection
		// (HTTP/1.1).
		limitedAt := time.Duration(-1)
		h := &rateLimitedHandler{start: time.Now(), retryAfter: "10", limit: func(host string, at time.Duration) bool {
			if host != "a.example" || limitedAt >= 0 {
				return false
			}
			limitedAt = at
			return true
		}}
		plan := append(plannedOn(KindAvatar, "https://a.example", 8), plannedOn(KindEmoji, "https://b.example", 20)...)
		a, out := fetchRateLimited(t, h, plan)

		if limitedAt != 0 {
			t.Fatalf("the 429 went to a request sent at %s, want the first one, at 0s", limitedAt)
		}
		// The 429 came half a second after its request.
		answered := limitedAt + time.Second/2
		until := answered + 10*time.Second
		requests := h.requests()
		for _, at := range requests["a.example"] {
			if at >= answered && at < until {
				t.Errorf("a request to a.example at %s, while it waits out the 429 answered at %s", at, answered)
			}
		}
		toBWhileWaiting := 0
		for _, at := range requests["b.example"] {
			if at > answered && at < until {
				toBWhileWaiting++
			}
		}
		if toA, toB := len(requests["a.example"]), len(requests["b.example"]); toA != 9 || toB != 20 {
			t.Errorf("requests = %d to a.example and %d to b.example, want 9 (a retry) and 20", toA, toB)
		}
		if toBWhileWaiting == 0 {
			t.Errorf("no request to b.example while a.example waited: %v", requests)
		}
		re := regexp.MustCompile(`^notice: rate limited on download, waiting 1[01]s as instructed by Slack$`)
		if len(out.lines) != 1 || !re.MatchString(out.lines[0]) {
			t.Errorf("passed on = %q, want one line matching %s", out.lines, re)
		}
		for _, p := range plan {
			if _, ok := a.Save(p.Kind, p.SourceURL, AssetMeta{}); !ok {
				t.Errorf("Save(%s) ok = false", p.SourceURL)
			}
		}
	})
}

// TestAssetsFetchWaitsUpToTheLimitOfThirdParty: the limit on how long a
// download of a third-party host may wait holds the seconds its Retry-After
// asks for, not the jitter the wait adds to them. A 429 that asks for 60s is
// waited out, by the download that got it and by the downloads of its origin
// that start while the lane waits; one that asks for 61s fails them all.
func TestAssetsFetchWaitsUpToTheLimitOfThirdParty(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		retryAfter string
		waited     bool
	}{
		{retryAfter: "60", waited: true},
		{retryAfter: "61", waited: false},
	} {
		t.Run(tt.retryAfter, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// The first request to c.example gets the 429. The lane sends
				// 6 requests at 0s (HTTP/1.1); its other 2 downloads start
				// while it waits, once the 5 other requests are answered, at 1s.
				limited := false
				h := &rateLimitedHandler{start: time.Now(), retryAfter: tt.retryAfter, limit: func(string, time.Duration) bool {
					first := !limited
					limited = true
					return first
				}}
				plan := plannedOn(KindOGImage, "https://c.example", 8)
				a, out := fetchRateLimited(t, h, plan)

				saved := 0
				for _, p := range plan {
					if _, ok := a.Save(p.Kind, p.SourceURL, AssetMeta{}); ok {
						saved++
					}
				}
				requests := h.requests()["c.example"]
				if tt.waited {
					// The 429 came at 0.5s and asked to wait until 60.5s.
					if len(requests) != 9 || saved != 8 {
						t.Errorf("requests at %v and %d assets saved, want 9 requests (a retry) and all 8 saved", requests, saved)
					}
					for _, at := range requests {
						if at > 0 && at < 60*time.Second+time.Second/2 {
							t.Errorf("a request at %s, while the lane waits out the 429", at)
						}
					}
					re := regexp.MustCompile(`^notice: rate limited on download, waiting 1m[01]s as instructed by Slack$`)
					if len(out.lines) != 1 || !re.MatchString(out.lines[0]) {
						t.Errorf("passed on = %q, want one line matching %s", out.lines, re)
					}
					return
				}
				if len(requests) != 6 || slices.Max(requests) != 0 || saved != 5 {
					t.Errorf("requests at %v and %d assets saved, want the lane's 6 at 0s and 5 saved", requests, saved)
				}
				re := regexp.MustCompile(`^warning: asset failed \(og_image\): rate limited \(429\): the server asks to wait 1m1s, over the 1m0s limit$`)
				if len(out.lines) != 3 {
					t.Fatalf("passed on = %q, want 3 warnings", out.lines)
				}
				for i, line := range out.lines {
					if !re.MatchString(line) {
						t.Errorf("line %d = %q, want it to match %s", i, line, re)
					}
				}
			})
		})
	}
}

// TestAssetsFetchFailsLongRateLimitOfThirdParty: a third-party host whose 429
// asks to wait longer than a download of it may wait fails that download at
// once, and the rest of its downloads too, for as long as its wait lasts,
// without a request. The Slack file host is waited out, however long it asks.
func TestAssetsFetchFailsLongRateLimitOfThirdParty(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// c.example answers every request with the 429, and files.slack.com
		// the first one. The lane of c.example sends 6 requests before the
		// first 429 comes; its 4 other downloads start while it waits.
		limited := map[string]bool{}
		h := &rateLimitedHandler{start: time.Now(), retryAfter: "120", limit: func(host string, _ time.Duration) bool {
			first := !limited[host]
			limited[host] = true
			return first || host == "c.example"
		}}
		public := plannedOn(KindOGImage, "https://c.example", 10)
		slackFiles := plannedOn(KindAttachment, "https://files.slack.com/files-pri/T0-F1", 2)
		a, out := fetchRateLimited(t, h, append(slices.Clone(public), slackFiles...))

		requests := h.requests()
		if got := requests["c.example"]; len(got) != 6 || slices.Max(got) != 0 {
			t.Errorf("requests to c.example at %v, want the lane's 6 at 0s only", got)
		}
		// The Slack file host is retried once its wait is over.
		if got := requests["files.slack.com"]; len(got) != 3 || got[2] < 120*time.Second+time.Second/2 {
			t.Errorf("requests to files.slack.com at %v, want 3, the retry after the 120s wait", got)
		}

		failure := regexp.MustCompile(`^warning: asset failed \(og_image\): rate limited \(429\): the server asks to wait 2m0s, over the 1m0s limit$`)
		waited := regexp.MustCompile(`^notice: rate limited on download, waiting 2m[01]s as instructed by Slack$`)
		if len(out.lines) != 11 {
			t.Fatalf("passed on = %q, want 10 warnings and a notice", out.lines)
		}
		for i, line := range out.lines[:10] {
			if !failure.MatchString(line) {
				t.Errorf("line %d = %q, want it to match %s", i, line, failure)
			}
			// The failures come at once, when the first 429s come.
			if out.at[i] != time.Second/2 {
				t.Errorf("line %d passed on at %s, want 500ms", i, out.at[i])
			}
		}
		if !waited.MatchString(out.lines[10]) {
			t.Errorf("line 10 = %q, want it to match %s", out.lines[10], waited)
		}
		for _, p := range public {
			if _, ok := a.Save(p.Kind, p.SourceURL, AssetMeta{}); ok {
				t.Errorf("Save(%s) ok = true, want the download failed", p.SourceURL)
			}
		}
		for _, p := range slackFiles {
			if _, ok := a.Save(p.Kind, p.SourceURL, AssetMeta{}); !ok {
				t.Errorf("Save(%s) ok = false", p.SourceURL)
			}
		}
	})
}
