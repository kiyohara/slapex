package slack

// Tests of the method lanes (Issue #278, PF-06 of #272; decision log 0069):
// the calls of one Web API method run one at a time, in the order they come,
// each once the previous one has ended and at least methodPace after it
// started; the calls of different methods run side by side; a 429's
// Retry-After holds back its method only; a call whose context ends stops
// waiting; and the HTTP trace counts as a call's pacing wait only the wait
// for methodPace once the call has its lane. The calls go through net/http to
// a fake server in a synctest bubble, over in-memory connections, so that the
// requests and the waits run on the bubble's clock.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path"
	"regexp"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// pipeServer serves an HTTP handler over in-memory connections (net.Pipe),
// on which a goroutine in a synctest bubble waits durably, unlike on a
// socket, as the one of internal/output/ratelimit_test.go does. Its transport
// dials every host to the server.
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

// laneHandler answers users.info and bots.info after delay, with a 429 that
// asks to wait retryAfter seconds for the requests limit picks, and records
// the requests and how many of each method were under way at a time.
type laneHandler struct {
	start      time.Time
	delay      time.Duration
	retryAfter string
	limit      func(arg string, attempt int) bool

	mu       sync.Mutex
	reqs     []laneRequest
	attempts map[string]int
	underWay map[string]int
	most     map[string]int // the most requests of a method under way at a time
}

// laneRequest is one request the server got: when it came and when it was
// answered, since the test started.
type laneRequest struct {
	method, arg string
	start, end  time.Duration
	status      int
}

func newLaneHandler(delay time.Duration) *laneHandler {
	return &laneHandler{start: time.Now(), delay: delay,
		attempts: map[string]int{}, underWay: map[string]int{}, most: map[string]int{}}
}

func (h *laneHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	method := path.Base(r.URL.Path)
	arg := r.PostForm.Get("user") + r.PostForm.Get("bot")
	h.mu.Lock()
	req := laneRequest{method: method, arg: arg, start: time.Since(h.start), status: http.StatusOK}
	attempt := h.attempts[arg]
	h.attempts[arg]++
	h.underWay[method]++
	h.most[method] = max(h.most[method], h.underWay[method])
	if h.limit != nil && h.limit(arg, attempt) {
		req.status = http.StatusTooManyRequests
	}
	h.mu.Unlock()

	time.Sleep(h.delay)

	h.mu.Lock()
	h.underWay[method]--
	req.end = time.Since(h.start)
	h.reqs = append(h.reqs, req)
	h.mu.Unlock()
	if req.status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", h.retryAfter)
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	switch method {
	case "users.info":
		fmt.Fprintf(w, `{"ok":true,"user":{"id":%q}}`, arg)
	case "bots.info":
		fmt.Fprintf(w, `{"ok":true,"bot":{"id":%q}}`, arg)
	default:
		http.NotFound(w, r)
	}
}

// requests are the requests of method, in the order they came.
func (h *laneHandler) requests(method string) []laneRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	var reqs []laneRequest
	for _, r := range h.reqs {
		if r.method == method {
			reqs = append(reqs, r)
		}
	}
	slices.SortFunc(reqs, func(a, b laneRequest) int { return cmp.Compare(a.start, b.start) })
	return reqs
}

func (h *laneHandler) mostUnderWay(method string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.most[method]
}

// newLaneClient returns a client of the server of h, which paces and waits on
// the bubble's clock, and the function that stops the server.
func newLaneClient(h http.Handler, opts ...Option) (*Client, func()) {
	srv := startPipeServer(h)
	tr := srv.transport()
	c := New(testToken, append([]Option{WithBaseURL("http://slack.test/api/"), WithTransport(tr)}, opts...)...)
	return c, func() {
		tr.CloseIdleConnections()
		srv.stop()
	}
}

// callInOrder runs each call on a goroutine of its own, starting the next once
// the ones before wait (for their lane, a response or a pacing wait), so that
// the calls come in the order given. It returns their errors once all have
// returned.
func callInOrder(calls ...func() error) []error {
	errs := make([]error, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Go(func() { errs[i] = call() })
		synctest.Wait()
	}
	wg.Wait()
	return errs
}

func userInfo(ctx context.Context, c *Client, id string) func() error {
	return func() error {
		_, err := c.UserInfo(ctx, id)
		return err
	}
}

func botInfo(ctx context.Context, c *Client, id string) func() error {
	return func() error {
		_, err := c.BotInfo(ctx, id)
		return err
	}
}

func assertNoErrors(t *testing.T, errs []error) {
	t.Helper()
	for i, err := range errs {
		if err != nil {
			t.Errorf("call %d: %v", i, err)
		}
	}
}

// assertStarts checks that the requests went out for args, in that order, at
// the times want gives.
func assertStarts(t *testing.T, method string, reqs []laneRequest, args []string, want []time.Duration) {
	t.Helper()
	var gotArgs []string
	var got []time.Duration
	for _, r := range reqs {
		gotArgs = append(gotArgs, r.arg)
		got = append(got, r.start)
	}
	if !slices.Equal(gotArgs, args) || !slices.Equal(got, want) {
		t.Errorf("%s requests = %v at %v, want %v at %v", method, gotArgs, got, args, want)
	}
}

// TestMethodLaneRunsCallsOneAtATime: calls of one method that come together
// go out one at a time, in the order they came, each methodPace after the one
// before started, or once it ended when it took longer.
func TestMethodLaneRunsCallsOneAtATime(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		delay time.Duration
		want  []time.Duration
	}{
		{name: "paced", delay: 300 * time.Millisecond, want: []time.Duration{0, time.Second, 2 * time.Second}},
		{name: "after the one before", delay: 1500 * time.Millisecond, want: []time.Duration{0, 1500 * time.Millisecond, 3 * time.Second}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				h := newLaneHandler(tt.delay)
				c, stop := newLaneClient(h)
				defer stop()
				ctx := context.Background()
				assertNoErrors(t, callInOrder(userInfo(ctx, c, "U3"), userInfo(ctx, c, "U1"), userInfo(ctx, c, "U2")))

				assertStarts(t, "users.info", h.requests("users.info"), []string{"U3", "U1", "U2"}, tt.want)
				if n := h.mostUnderWay("users.info"); n != 1 {
					t.Errorf("%d users.info requests under way at a time, want 1", n)
				}
			})
		})
	}
}

// TestMethodLanesRunSideBySide: the calls of different methods do not wait
// for each other.
func TestMethodLanesRunSideBySide(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		h := newLaneHandler(300 * time.Millisecond)
		c, stop := newLaneClient(h)
		defer stop()
		ctx := context.Background()
		assertNoErrors(t, callInOrder(userInfo(ctx, c, "U1"), botInfo(ctx, c, "B1"), userInfo(ctx, c, "U2"), botInfo(ctx, c, "B2")))

		assertStarts(t, "users.info", h.requests("users.info"), []string{"U1", "U2"}, []time.Duration{0, time.Second})
		assertStarts(t, "bots.info", h.requests("bots.info"), []string{"B1", "B2"}, []time.Duration{0, time.Second})
	})
}

// TestMethodLaneRetryAfterHoldsItsMethodOnly: a call that gets a 429 keeps its
// lane while it waits out the Retry-After, so the next call of its method
// waits too, while the calls of another method go on.
func TestMethodLaneRetryAfterHoldsItsMethodOnly(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		h := newLaneHandler(100 * time.Millisecond)
		h.retryAfter = "3"
		h.limit = func(arg string, attempt int) bool { return arg == "U1" && attempt == 0 }
		c, stop := newLaneClient(h)
		defer stop()
		var notices []string
		c.Logf = func(format string, args ...any) { notices = append(notices, fmt.Sprintf(format, args...)) }
		ctx := context.Background()
		assertNoErrors(t, callInOrder(userInfo(ctx, c, "U1"), userInfo(ctx, c, "U2"), botInfo(ctx, c, "B1"), botInfo(ctx, c, "B2"), botInfo(ctx, c, "B3")))

		users := h.requests("users.info")
		if len(users) != 3 || users[0].arg != "U1" || users[0].status != http.StatusTooManyRequests ||
			users[1].arg != "U1" || users[2].arg != "U2" {
			t.Fatalf("users.info requests = %+v, want U1's 429, its retry, then U2", users)
		}
		// The retry waits the 3s asked for from the 429, plus up to 1s of
		// jitter; U2 goes out once U1's retry has ended.
		if wait := users[1].start - users[0].end; wait < 3*time.Second || wait >= 4*time.Second {
			t.Errorf("U1 retried %s after its 429, want the 3s asked for and up to 1s of jitter", wait)
		}
		if users[2].start != users[1].end {
			t.Errorf("U2 went out at %s, want once U1's retry ended at %s", users[2].start, users[1].end)
		}
		assertStarts(t, "bots.info", h.requests("bots.info"), []string{"B1", "B2", "B3"},
			[]time.Duration{0, time.Second, 2 * time.Second})
		re := regexp.MustCompile(`^rate limited on api users\.info, waiting [34]s as instructed by Slack$`)
		if len(notices) != 1 || !re.MatchString(notices[0]) {
			t.Errorf("notices = %q, want one matching %s", notices, re)
		}
	})
}

// TestMethodLaneCanceledCallStopsWaiting: a call whose context ends while it
// waits for its lane, or for its pacing, returns at once without a request,
// and the next call of its method goes out as if the canceled one had not
// come.
func TestMethodLaneCanceledCallStopsWaiting(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		delay    time.Duration // of each response
		cancelAt time.Duration // U2's cancellation
		// U3 goes out once U1 has ended and methodPace after U1 started.
		wantU3 time.Duration
	}{
		{name: "waiting for the lane", delay: 5 * time.Second, cancelAt: 2 * time.Second, wantU3: 5 * time.Second},
		{name: "waiting for the pacing", delay: 100 * time.Millisecond, cancelAt: 500 * time.Millisecond, wantU3: time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				h := newLaneHandler(tt.delay)
				c, stop := newLaneClient(h)
				defer stop()
				ctx := context.Background()
				canceled, cancel := context.WithCancel(ctx)
				var returned time.Duration
				u2 := func() error {
					_, err := c.UserInfo(canceled, "U2")
					returned = time.Since(h.start)
					return err
				}
				go func() {
					time.Sleep(tt.cancelAt)
					cancel()
				}()
				errs := callInOrder(userInfo(ctx, c, "U1"), u2, userInfo(ctx, c, "U3"))

				if errs[0] != nil || errs[2] != nil {
					t.Errorf("U1, U3 errors = %v, %v, want none", errs[0], errs[2])
				}
				if err := errs[1]; err == nil || err.Error() != "slack api users.info: context canceled" || !errors.Is(err, context.Canceled) {
					t.Errorf("U2 error = %v, want the cancellation", err)
				}
				if returned != tt.cancelAt {
					t.Errorf("U2 returned at %s, want at its cancellation, %s", returned, tt.cancelAt)
				}
				assertStarts(t, "users.info", h.requests("users.info"), []string{"U1", "U3"}, []time.Duration{0, tt.wantU3})
			})
		})
	}
}

// TestMethodLanesPassOnACanceledTurn: a call whose context ends as the lane
// passes to it passes the lane on to the next call, and a call whose context
// has ended does not take a free lane.
func TestMethodLanesPassOnACanceledTurn(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var m methodLanes
		ctx := context.Background()
		first, err := m.enter(ctx, "users.info")
		if err != nil {
			t.Fatalf("enter: %v", err)
		}
		canceled, cancel := context.WithCancel(ctx)
		results := make(chan string, 2)
		go func() {
			if _, err := m.enter(canceled, "users.info"); err != nil {
				results <- "second: " + err.Error()
				return
			}
			results <- "second got the lane"
		}()
		synctest.Wait()
		go func() {
			l, err := m.enter(ctx, "users.info")
			if err != nil {
				results <- "third: " + err.Error()
				return
			}
			results <- "third got the lane"
			m.leave(l)
		}()
		synctest.Wait()
		// The second call's context ends as the lane passes to it: with the
		// lanes locked, the call can only get as far as taking itself back,
		// and finds the lane passed to it by then.
		m.mu.Lock()
		cancel()
		m.leaveLocked(first)
		m.mu.Unlock()
		got := []string{<-results, <-results}
		slices.Sort(got)
		if want := []string{"second: context canceled", "third got the lane"}; !slices.Equal(got, want) {
			t.Errorf("results = %q, want %q", got, want)
		}

		// The lane is free again: a canceled call does not take it, and the
		// next call does.
		if _, err := m.enter(canceled, "users.info"); !errors.Is(err, context.Canceled) {
			t.Errorf("enter with a canceled context = %v, want the cancellation", err)
		}
		l, err := m.enter(ctx, "users.info")
		if err != nil {
			t.Fatalf("enter after the canceled one: %v", err)
		}
		m.leave(l)
	})
}

// TestTraceCountsOnlyThePacingInItsLane: the pacing wait of a call is the wait
// for methodPace once the call has its lane, not the wait for the lane: of
// three calls that come together, the second and third wait for the lane
// while the one before them runs, and only then for the rest of methodPace,
// if any.
func TestTraceCountsOnlyThePacingInItsLane(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		delay time.Duration
		want  []int64 // pacing waits in microseconds
	}{
		{name: "paced", delay: 300 * time.Millisecond, want: []int64{0, 700_000, 700_000}},
		{name: "after the one before", delay: 1500 * time.Millisecond, want: []int64{0, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				w := &traceWriter{}
				c, stop := newLaneClient(newLaneHandler(tt.delay), WithTrace(w))
				defer stop()
				ctx := context.Background()
				assertNoErrors(t, callInOrder(userInfo(ctx, c, "U1"), userInfo(ctx, c, "U2"), userInfo(ctx, c, "U3")))

				recs := w.records(t)
				slices.SortFunc(recs, func(a, b TraceRecord) int { return a.Start.Compare(b.Start) })
				var got []int64
				for _, rec := range recs {
					got = append(got, rec.PacingWaitUS)
				}
				if !slices.Equal(got, tt.want) {
					t.Errorf("pacing waits = %v us, want %v", got, tt.want)
				}
			})
		})
	}
}
