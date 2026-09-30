package lane

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

// starts records the calls of the jobs a Scheduler runs: the order they start
// in, by name, and the most that run at a time.
type starts struct {
	mu      sync.Mutex
	names   []string
	running int
	most    int
}

// job returns a do for Add that records the start of job i under
// prefix+i, runs body, and records its end.
func (st *starts) job(prefix string, body func(ctx context.Context, i int)) func(context.Context, int) {
	return func(ctx context.Context, i int) {
		st.mu.Lock()
		st.names = append(st.names, fmt.Sprintf("%s%d", prefix, i))
		st.running++
		st.most = max(st.most, st.running)
		st.mu.Unlock()
		defer func() {
			st.mu.Lock()
			st.running--
			st.mu.Unlock()
		}()
		if body != nil {
			body(ctx, i)
		}
	}
}

func (st *starts) log() []string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return slices.Clone(st.names)
}

func (st *starts) now() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.running
}

// waitDone fails the test when g does not end within a while.
func waitDone(t *testing.T, what string, g *Group) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		g.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// TestSchedulerAddOrder: the jobs Add adds wait in their lane with the jobs
// added before them, in order of size: a larger job added later starts before
// the smaller ones waiting, and the jobs of unknown size start last, in the
// order they came.
func TestSchedulerAddOrder(t *testing.T) {
	t.Parallel()

	const o = "https://files.example.com"
	s := NewScheduler(context.Background(), Limits{HTTP2: 1, HTTP1: 1, Total: 1})
	defer s.Close()
	var st starts
	release := make(chan struct{})
	a := s.Add(jobsOn(o, 0, 0, 0), st.job("a", func(_ context.Context, i int) {
		if i == 0 {
			<-release
		}
	}))
	waitFor(t, "the first job to start", func() bool { return st.now() == 1 })
	b := s.Add(jobsOn(o, 5, 0), st.job("b", nil))
	c := s.Add(jobsOn(o, 9), st.job("c", nil))
	close(release)
	for _, g := range []*Group{a, b, c} {
		waitDone(t, "the jobs to end", g)
	}
	want := []string{"a0", "c0", "b0", "a1", "a2", "b1"}
	if got := st.log(); !slices.Equal(got, want) {
		t.Errorf("starts = %q, want %q", got, want)
	}
}

// TestSchedulerSharesLimits: the jobs of later Adds share the limits of the
// jobs under way, and a lane whose first job has a connection runs the jobs
// added after at the limit that connection allows, without a first job alone
// again.
func TestSchedulerSharesLimits(t *testing.T) {
	t.Parallel()

	const o = "https://files.example.com"
	limits := Limits{HTTP2: 3, HTTP1: 1, Total: 64}
	s := NewScheduler(context.Background(), limits)
	defer s.Close()
	var st starts
	first := s.Add(jobsOn(o, 0), st.job("a", func(ctx context.Context, _ int) { gotConn(ctx, "h2") }))
	waitDone(t, "the first job", first)

	release := make(chan struct{})
	later := s.Add(jobsOn(o, 0, 0, 0, 0), st.job("b", func(context.Context, int) { <-release }))
	waitFor(t, "the lane to run its HTTP/2 limit", func() bool { return st.now() == limits.HTTP2 })
	// The limit holds: the fourth job waits for a place.
	time.Sleep(10 * time.Millisecond)
	if n := st.now(); n != limits.HTTP2 {
		t.Errorf("%d jobs at a time, want the HTTP/2 limit %d", n, limits.HTTP2)
	}
	close(release)
	waitDone(t, "the later jobs", later)
	if n := len(st.log()); n != 5 {
		t.Errorf("%d jobs started, want 5", n)
	}
	st.mu.Lock()
	most := st.most
	st.mu.Unlock()
	if most != limits.HTTP2 {
		t.Errorf("at most %d jobs at a time, want %d", most, limits.HTTP2)
	}
}

// TestGroupStop: Stop drops the jobs of the group that wait to start, which
// get no call, and cancels the context of those under way; the other groups
// run on.
func TestGroupStop(t *testing.T) {
	t.Parallel()

	const o = "https://files.example.com"
	s := NewScheduler(context.Background(), Limits{HTTP2: 1, HTTP1: 1, Total: 1})
	defer s.Close()
	var st starts
	a := s.Add(jobsOn(o, 0), st.job("a", func(ctx context.Context, _ int) {
		<-ctx.Done()
	}))
	waitFor(t, "the first job to start", func() bool { return st.now() == 1 })
	b := s.Add(jobsOn(o, 0, 0), st.job("b", nil))
	c := s.Add(jobsOn(o, 0), st.job("c", nil))

	b.Stop()
	waitDone(t, "the stopped group that waits to start", b)
	a.Stop()
	waitDone(t, "the stopped group under way", a)
	waitDone(t, "the group after them", c)
	if got, want := st.log(), []string{"a0", "c0"}; !slices.Equal(got, want) {
		t.Errorf("starts = %q, want %q", got, want)
	}
}

// TestSchedulerClose: Close cancels the calls under way, drops the jobs that
// wait to start and returns once the calls have; an Add after it runs
// nothing.
func TestSchedulerClose(t *testing.T) {
	t.Parallel()

	const o = "https://files.example.com"
	s := NewScheduler(context.Background(), Limits{HTTP2: 2, HTTP1: 2, Total: 2})
	var st starts
	var mu sync.Mutex
	returned := 0
	g := s.Add(jobsOn(o, 0, 0, 0, 0), st.job("a", func(ctx context.Context, i int) {
		if i == 0 {
			gotConn(ctx, "h2")
		}
		<-ctx.Done()
		mu.Lock()
		returned++
		mu.Unlock()
	}))
	waitFor(t, "two jobs to start", func() bool { return st.now() == 2 })
	s.Close()
	mu.Lock()
	if returned != 2 {
		t.Errorf("Close returned with %d of the 2 calls under way returned", returned)
	}
	mu.Unlock()
	waitDone(t, "the group", g)
	after := s.Add(jobsOn(o, 0), st.job("b", nil))
	waitDone(t, "the group added after Close", after)
	if got, want := len(st.log()), 2; got != want {
		t.Errorf("%d jobs started (%q), want %d", got, st.log(), want)
	}
}

// TestSchedulerCanceled: once the scheduler's context is done, the jobs that
// wait to start are dropped, the groups end once their calls have returned,
// and an Add runs nothing.
func TestSchedulerCanceled(t *testing.T) {
	t.Parallel()

	const o = "https://files.example.com"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewScheduler(ctx, Limits{HTTP2: 1, HTTP1: 1, Total: 1})
	defer s.Close()
	var st starts
	a := s.Add(jobsOn(o, 0), st.job("a", func(ctx context.Context, _ int) { <-ctx.Done() }))
	waitFor(t, "the first job to start", func() bool { return st.now() == 1 })
	b := s.Add(jobsOn(o, 0, 0), st.job("b", nil))
	cancel()
	waitDone(t, "the group under way", a)
	waitDone(t, "the group waiting to start", b)
	waitDone(t, "a group added after the cancel", s.Add(jobsOn(o, 0), st.job("c", nil)))
	if got, want := st.log(), []string{"a0"}; !slices.Equal(got, want) {
		t.Errorf("starts = %q, want %q", got, want)
	}
}
