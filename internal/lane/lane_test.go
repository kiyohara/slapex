package lane

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrigin(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ url, want string }{
		{"https://files.slack.com/files-pri/T1-F1/a.png", "https://files.slack.com:443"},
		{"HTTPS://Files.Slack.com:443/b.png", "https://files.slack.com:443"},
		{"http://example.com/a.png", "http://example.com:80"},
		{"https://example.com:8443/a.png", "https://example.com:8443"},
		{"https://[::1]:8443/a.png", "https://[::1]:8443"},
		{"not a url", "not a url"},
		{"", ""},
	} {
		if got := origin(tc.url); got != tc.want {
			t.Errorf("origin(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

// fakeConn stands for a connection whose TLS handshake negotiated proto.
type fakeConn struct {
	net.Conn
	proto string
}

func (c fakeConn) ConnectionState() tls.ConnectionState {
	return tls.ConnectionState{NegotiatedProtocol: c.proto}
}

// gotConn reports a connection that negotiated proto to the hook in ctx, as
// net/http does when a request has its connection.
func gotConn(ctx context.Context, proto string) {
	if trace := httptrace.ContextClientTrace(ctx); trace != nil && trace.GotConn != nil {
		trace.GotConn(httptrace.GotConnInfo{Conn: fakeConn{proto: proto}})
	}
}

// tracker records what the jobs of a Run do: the order they start in and the
// most that run at a time, per origin and over all.
type tracker struct {
	jobs []Job

	mu       sync.Mutex
	events   []string // "start <i>", and the markers the test adds
	running  map[string]int
	most     map[string]int
	total    int
	mostOver int
}

func newTracker(jobs []Job) *tracker {
	return &tracker{jobs: jobs, running: map[string]int{}, most: map[string]int{}}
}

func (tr *tracker) start(i int) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	o := origin(tr.jobs[i].URL)
	tr.events = append(tr.events, fmt.Sprintf("start %d", i))
	tr.running[o]++
	tr.most[o] = max(tr.most[o], tr.running[o])
	tr.total++
	tr.mostOver = max(tr.mostOver, tr.total)
}

func (tr *tracker) end(i int) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.running[origin(tr.jobs[i].URL)]--
	tr.total--
}

func (tr *tracker) mark(event string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.events = append(tr.events, event)
}

func (tr *tracker) runningOn(o string) int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.running[origin(o)]
}

func (tr *tracker) mostOn(o string) int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.most[origin(o)]
}

func (tr *tracker) log() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.events)
}

// waitFor polls cond until it holds, and fails the test after a while.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Errorf("timed out waiting for %s", what)
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func jobsOn(url string, sizes ...int64) []Job {
	var jobs []Job
	for i, size := range sizes {
		jobs = append(jobs, Job{URL: fmt.Sprintf("%s/%d", url, i), Size: size})
	}
	return jobs
}

// TestRunStartOrder: a lane starts its jobs of known size largest first, and
// those of unknown size after them in the order given.
func TestRunStartOrder(t *testing.T) {
	t.Parallel()

	jobs := jobsOn("https://files.example.com", 0, 5, 0, 9, 5, 2)
	tr := newTracker(jobs)
	Run(context.Background(), jobs, Limits{HTTP2: 1, HTTP1: 1, Total: 1}, func(_ context.Context, i int) {
		tr.start(i)
		tr.end(i)
	})
	want := []string{"start 3", "start 1", "start 4", "start 5", "start 0", "start 2"}
	if got := tr.log(); !slices.Equal(got, want) {
		t.Errorf("starts = %q, want %q", got, want)
	}
}

// TestRunLeadsEachOrigin: a lane runs its first job alone until the job has a
// connection, then up to the limit of the protocol the connection negotiated,
// while the first job still runs. Lanes do not wait for each other.
func TestRunLeadsEachOrigin(t *testing.T) {
	t.Parallel()

	const h2, h1 = "https://h2.example.com", "https://h1.example.com"
	jobs := append(jobsOn(h2, 0, 0, 0, 0, 0, 0), jobsOn(h1, 0, 0, 0, 0)...)
	limits := Limits{HTTP2: 4, HTTP1: 2, Total: 64}
	tr := newTracker(jobs)
	release := map[string]chan struct{}{h2: make(chan struct{}), h1: make(chan struct{})}
	Run(context.Background(), jobs, limits, func(ctx context.Context, i int) {
		tr.start(i)
		defer tr.end(i)
		o, proto, limit := h2, "h2", limits.HTTP2
		if i >= 6 {
			o, proto, limit = h1, "http/1.1", limits.HTTP1
		}
		if i != 0 && i != 6 {
			<-release[o]
			return
		}
		// The first job of its lane runs alone until it has a connection.
		if n := tr.runningOn(o); n != 1 {
			t.Errorf("%s: %d jobs run with the first before its connection, want it alone", o, n)
		}
		tr.mark("connected " + o)
		gotConn(ctx, proto)
		waitFor(t, o+" to run its limit", func() bool { return tr.runningOn(o) == limit })
		close(release[o])
	})

	for _, tc := range []struct {
		origin      string
		first, last int
		limit       int
	}{{h2, 0, 5, limits.HTTP2}, {h1, 6, 9, limits.HTTP1}} {
		if got := tr.mostOn(tc.origin); got != tc.limit {
			t.Errorf("%s ran at most %d jobs at a time, want %d", tc.origin, got, tc.limit)
		}
		log := tr.log()
		connected := slices.Index(log, "connected "+tc.origin)
		for i := tc.first + 1; i <= tc.last; i++ {
			if at := slices.Index(log, fmt.Sprintf("start %d", i)); at < connected {
				t.Errorf("job %d of %s started before the first had a connection: %q", i, tc.origin, log)
			}
		}
	}
}

// TestRunLeaderWithoutConnection: a first job that ends without a connection
// opens its lane at the HTTP/1.1 limit.
func TestRunLeaderWithoutConnection(t *testing.T) {
	t.Parallel()

	jobs := jobsOn("https://down.example.com", 0, 0, 0, 0, 0, 0)
	tr := newTracker(jobs)
	var after atomic.Int32 // jobs started after the first
	both := make(chan struct{})
	Run(context.Background(), jobs, Limits{HTTP2: 4, HTTP1: 2, Total: 64}, func(_ context.Context, i int) {
		tr.start(i)
		defer tr.end(i)
		if i == 0 {
			time.Sleep(10 * time.Millisecond)
			tr.mark("first ended")
			return
		}
		// The two that start when the first ends wait for each other.
		switch after.Add(1) {
		case 1:
			select {
			case <-both:
			case <-time.After(5 * time.Second):
				t.Error("timed out waiting for a second job to run")
			}
		case 2:
			close(both)
		}
	})
	log := tr.log()
	if at := slices.Index(log, "first ended"); at != 1 {
		t.Errorf("events = %q, want the first job to end before the others start", log)
	}
	if got := tr.mostOn(jobs[0].URL); got != 2 {
		t.Errorf("at most %d jobs at a time, want the HTTP/1.1 limit 2", got)
	}
}

// TestRunTotalLimit: all the lanes together run at most Total jobs, and each
// lane gets a turn.
func TestRunTotalLimit(t *testing.T) {
	t.Parallel()

	var jobs []Job
	for o := range 8 {
		jobs = append(jobs, jobsOn(fmt.Sprintf("https://o%d.example.com", o), 0, 0, 0)...)
	}
	tr := newTracker(jobs)
	Run(context.Background(), jobs, Limits{HTTP2: 4, HTTP1: 4, Total: 5}, func(ctx context.Context, i int) {
		tr.start(i)
		defer tr.end(i)
		gotConn(ctx, "h2")
		time.Sleep(20 * time.Millisecond)
	})
	if tr.mostOver > 5 {
		t.Errorf("%d jobs ran at a time, want at most 5", tr.mostOver)
	}
	if tr.mostOver < 2 {
		t.Errorf("at most %d job ran at a time, want the lanes to run side by side", tr.mostOver)
	}
	if n := len(tr.log()); n != len(jobs) {
		t.Errorf("%d jobs started, want %d", n, len(jobs))
	}
}

// TestRunLargeJobs: a lane runs at most Large jobs of LargeSize or more at a
// time, and the small ones take the rest of its places.
func TestRunLargeJobs(t *testing.T) {
	t.Parallel()

	// Jobs 0-3 are large, 4-9 small (size unknown or under 100).
	jobs := jobsOn("https://files.example.com", 400, 300, 200, 100, 0, 99, 0, 0, 0, 0)
	limits := Limits{HTTP2: 4, HTTP1: 4, Total: 64, Large: 2, LargeSize: 100}
	var mu sync.Mutex
	large, mostLarge := 0, 0
	tr := newTracker(jobs)
	release := make(chan struct{})
	Run(context.Background(), jobs, limits, func(ctx context.Context, i int) {
		tr.start(i)
		defer tr.end(i)
		isLarge := jobs[i].Size >= 100
		mu.Lock()
		if isLarge {
			large++
			mostLarge = max(mostLarge, large)
		}
		mu.Unlock()
		defer func() {
			mu.Lock()
			if isLarge {
				large--
			}
			mu.Unlock()
		}()
		if i == 0 {
			gotConn(ctx, "h2")
			waitFor(t, "the lane to run its limit", func() bool { return tr.runningOn(jobs[0].URL) == 4 })
			close(release)
			return
		}
		<-release
	})
	if mostLarge != 2 {
		t.Errorf("at most %d large jobs at a time, want 2", mostLarge)
	}
	// Once the lane opens, the second large job and two small ones join the
	// first: size 99 is small, and the unknown sizes come after it.
	first := tr.log()[:4]
	slices.Sort(first)
	if want := []string{"start 0", "start 1", "start 4", "start 5"}; !slices.Equal(first, want) {
		t.Errorf("first starts = %q, want %q", first, want)
	}
	if got := tr.mostOn(jobs[0].URL); got != 4 {
		t.Errorf("at most %d jobs at a time, want 4", got)
	}
}

// TestRunCanceled: once the context is done, Run starts no more jobs, and
// returns when the running ones have.
func TestRunCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobs := jobsOn("https://files.example.com", 0, 0, 0, 0, 0, 0, 0, 0)
	var started atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, jobs, Limits{HTTP2: 3, HTTP1: 3, Total: 64}, func(ctx context.Context, i int) {
			started.Add(1)
			if i == 0 {
				gotConn(ctx, "h2")
			}
			<-ctx.Done()
		})
	}()
	waitFor(t, "three jobs to start", func() bool { return started.Load() == 3 })
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the cancel")
	}
	if n := started.Load(); n != 3 {
		t.Errorf("%d jobs started, want the 3 running at the cancel", n)
	}
}

// TestRunOverHTTP runs the lanes against real servers: an HTTP/2 origin gets
// one connection, which its jobs share up to the HTTP/2 limit, and an
// HTTP/1.1 origin runs up to the HTTP/1.1 limit, a connection each. The
// connections of the HTTP/1.1 origin are not bounded here: net/http completes
// a dial that a connection coming free overtook, and keeps the connection.
func TestRunOverHTTP(t *testing.T) {
	t.Parallel()

	limits := Limits{HTTP2: 5, HTTP1: 3, Total: 64}
	for _, tc := range []struct {
		name  string
		http2 bool
		limit int
	}{{"HTTP/2", true, limits.HTTP2}, {"HTTP/1.1", false, limits.HTTP1}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var conns, inflight, most atomic.Int32
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := inflight.Add(1)
				defer inflight.Add(-1)
				for m := most.Load(); n > m && !most.CompareAndSwap(m, n); m = most.Load() {
				}
				time.Sleep(20 * time.Millisecond)
				fmt.Fprint(w, r.Proto)
			}))
			srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateNew {
					conns.Add(1)
				}
			}
			srv.EnableHTTP2 = tc.http2
			srv.StartTLS()
			defer srv.Close()
			client := srv.Client()

			jobs := jobsOn(srv.URL, slices.Repeat([]int64{0}, 20)...)
			var protos sync.Map
			Run(context.Background(), jobs, limits, func(ctx context.Context, i int) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, jobs[i].URL, nil)
				if err != nil {
					t.Error(err)
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Error(err)
					return
				}
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				protos.Store(string(body), true)
			})
			protos.Range(func(proto, _ any) bool {
				if want := map[bool]string{true: "HTTP/2.0", false: "HTTP/1.1"}[tc.http2]; proto != want {
					t.Errorf("served over %s, want %s", proto, want)
				}
				return true
			})
			if n := most.Load(); n > int32(tc.limit) || n < 2 {
				t.Errorf("%d requests at a time, want 2 to %d", n, tc.limit)
			}
			if n := conns.Load(); tc.http2 && n > 1 {
				t.Errorf("%d connections, want 1", n)
			}
		})
	}
}
