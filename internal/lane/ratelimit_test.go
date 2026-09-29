package lane

// Rate limit tests (Issue #276, PF-04 of #272): a job gives its place up while
// it waits to retry, a 429 halves its lane's limit and, for the time its
// Retry-After asks, gives no job of the lane a place, and the responses after
// it give the limit back. The jobs are puppets that the test moves one step at
// a time in a synctest bubble, so the steps and the waits are exact.

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// puppets are the jobs of a Run, which take the steps the test tells them in
// the order told. A step that returns true ends its job.
type puppets struct {
	steps []chan step
	done  chan struct{}

	mu      sync.Mutex
	started []int
	ended   int
	sent    []int                 // the jobs whose Wait let them send
	waited  map[int]time.Duration // the latest wait of each job that sent
	errs    map[int]error         // the latest error of each job's Wait
}

type step func(ctx context.Context, p *puppets, i int) (end bool)

// runPuppets starts Run on jobs, each job a puppet, and waits until the jobs
// it starts are ready for their steps.
func runPuppets(ctx context.Context, jobs []Job, limits Limits) *puppets {
	p := &puppets{
		steps:  make([]chan step, len(jobs)),
		done:   make(chan struct{}),
		waited: map[int]time.Duration{},
		errs:   map[int]error{},
	}
	for i := range p.steps {
		p.steps[i] = make(chan step, 16)
	}
	go func() {
		defer close(p.done)
		Run(ctx, jobs, limits, func(ctx context.Context, i int) {
			p.mu.Lock()
			p.started = append(p.started, i)
			p.mu.Unlock()
			defer func() {
				p.mu.Lock()
				p.ended++
				p.mu.Unlock()
			}()
			for s := range p.steps[i] {
				if s(ctx, p, i) {
					return
				}
			}
		})
	}()
	synctest.Wait()
	return p
}

// tell has job i take steps, and waits until every other goroutine of the
// bubble is blocked again.
func (p *puppets) tell(i int, steps ...step) {
	for _, s := range steps {
		p.steps[i] <- s
	}
	synctest.Wait()
}

// finish ends every job, those not started too, and waits for Run to return.
func (p *puppets) finish() {
	for _, steps := range p.steps {
		steps <- end
	}
	<-p.done
}

func (p *puppets) startedJobs() []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.started)
}

// running counts the jobs started and not ended.
func (p *puppets) running() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.started) - p.ended
}

func (p *puppets) sentJobs() []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.sent)
}

func (p *puppets) waitOf(i int) (time.Duration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waited[i], p.errs[i]
}

// connect reports the job's connection, which negotiated proto (gotConn).
func connect(proto string) step {
	return func(ctx context.Context, _ *puppets, _ int) bool {
		gotConn(ctx, proto)
		return false
	}
}

// send waits until the lane lets the job send (Wait with maxWait), and records
// that it sent, or the error.
func send(maxWait time.Duration) step {
	return func(ctx context.Context, p *puppets, i int) bool {
		waited, err := Wait(ctx, maxWait)
		p.mu.Lock()
		defer p.mu.Unlock()
		if err != nil {
			p.errs[i] = err
			return false
		}
		p.sent = append(p.sent, i)
		p.waited[i] = waited
		return false
	}
}

// limited is a 429 that asks to wait wait: the job tells its lane, and gives
// its place up to wait before it retries.
func limited(wait time.Duration) step {
	return func(ctx context.Context, _ *puppets, _ int) bool {
		RateLimited(ctx, wait)
		Yield(ctx)
		return false
	}
}

// failed is a failure of another kind, which the job retries: it gives its
// place up for the backoff.
func failed(ctx context.Context, _ *puppets, _ int) bool {
	Yield(ctx)
	return false
}

// succeed is a 200 response: the job tells its lane, and ends.
func succeed(ctx context.Context, _ *puppets, _ int) bool {
	Succeeded(ctx)
	return true
}

func end(context.Context, *puppets, int) bool { return true }

// detached takes s with a context that Detach returned.
func detached(s step) step {
	return func(ctx context.Context, p *puppets, i int) bool {
		return s(Detach(ctx), p, i)
	}
}

// checkJobs compares the jobs got with want, in any order: the jobs that one
// step lets go on do so at once.
func checkJobs(t *testing.T, what string, got, want []int) {
	t.Helper()
	got, want = slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// TestRateLimitedPausesTheLane: for the time a 429 asks, the jobs of its lane
// send nothing, while those of another lane do. The lane then gives half its
// limit of places, first to the jobs that waited, in the order they started.
func TestRateLimitedPausesTheLane(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Jobs 0-3 are on origin a, 4 and 5 on origin b.
		jobs := append(jobsOn("https://a.example.com", 0, 0, 0, 0), jobsOn("https://b.example.com", 0, 0)...)
		p := runPuppets(context.Background(), jobs, Limits{HTTP2: 4, HTTP1: 4, Total: 64})
		defer p.finish()

		p.tell(0, connect("h2"), send(0))
		p.tell(4, connect("h2"), send(0))
		checkJobs(t, "started", p.startedJobs(), []int{0, 1, 2, 3, 4, 5})
		p.tell(0, limited(10*time.Second))
		for _, i := range []int{1, 2, 3, 5} {
			p.tell(i, send(0))
		}
		checkJobs(t, "sent while origin a waits", p.sentJobs(), []int{0, 4, 5})

		time.Sleep(10*time.Second - time.Nanosecond)
		synctest.Wait()
		checkJobs(t, "sent until the wait ends", p.sentJobs(), []int{0, 4, 5})
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		// The 429 halved the limit to 2: job 3 waits for a place.
		checkJobs(t, "sent once the wait ends", p.sentJobs(), []int{0, 4, 5, 1, 2})
		if waited, _ := p.waitOf(1); waited != 10*time.Second {
			t.Errorf("job 1 waited %s, want 10s", waited)
		}

		// Job 0 comes back to retry. It started before job 3, so it gets the
		// next place first.
		p.tell(0, send(0))
		p.tell(1, succeed)
		checkJobs(t, "sent once a job ends", p.sentJobs(), []int{0, 4, 5, 1, 2, 0})
		p.tell(2, succeed)
		checkJobs(t, "sent once another job ends", p.sentJobs(), []int{0, 4, 5, 1, 2, 0, 3})
	})
}

// TestRateLimitedHalvesOncePerRound: the 429s to the requests sent before the
// latest halving do not halve the limit again; a 429 to a request sent after
// it does. A lane with no job under way starts as many as its limit, which
// the test counts.
func TestRateLimitedHalvesOncePerRound(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := jobsOn("https://a.example.com", make([]int64, 20)...)
		p := runPuppets(context.Background(), jobs, Limits{HTTP2: 8, HTTP1: 8, Total: 64})
		defer p.finish()

		p.tell(0, connect("h2"))
		for i := range 8 {
			p.tell(i, send(0))
		}
		// Three 429s to the requests sent at once halve the limit once, to 4.
		for _, i := range []int{0, 1, 2} {
			p.tell(i, limited(0), end)
		}
		for i := 3; i < 8; i++ {
			p.tell(i, succeed)
		}
		if n := p.running(); n != 4 {
			t.Fatalf("%d jobs run, want the halved limit 4", n)
		}
		// Jobs 8-11 send once the limit is halved. A 429 to one of them halves
		// it again, to 2; a 429 to another, sent before that halving, does
		// not.
		for i := 8; i < 12; i++ {
			p.tell(i, send(0))
		}
		p.tell(8, limited(0), end)
		p.tell(9, limited(0), end)
		p.tell(10, succeed)
		p.tell(11, succeed)
		if n := p.running(); n != 2 {
			t.Errorf("%d jobs run, want the limit halved again, 2", n)
		}
	})
}

// TestSucceededRestoresTheLimit: every RestoreAfter responses to the requests
// sent since the latest halving give the lane one place back, up to the limit
// it opened at. The responses to the requests sent before it give nothing
// back.
func TestSucceededRestoresTheLimit(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := jobsOn("https://a.example.com", make([]int64, 30)...)
		p := runPuppets(context.Background(), jobs, Limits{HTTP2: 4, HTTP1: 4, Total: 64, RestoreAfter: 2})
		defer p.finish()

		p.tell(0, connect("h2"))
		for i := range 4 {
			p.tell(i, send(0))
		}
		p.tell(0, limited(0), end)
		for _, i := range []int{1, 2, 3} {
			p.tell(i, succeed)
		}
		if n := p.running(); n != 2 {
			t.Fatalf("%d jobs run, want the halved limit 2", n)
		}
		// Jobs 4 and 5 run, and their two responses give a place back.
		p.tell(4, send(0), succeed)
		p.tell(5, send(0), succeed)
		if n := p.running(); n != 3 {
			t.Errorf("%d jobs run after 2 responses, want 3", n)
		}
		// Jobs 6-8 run; two more responses give the last place back.
		p.tell(6, send(0), succeed)
		p.tell(7, send(0), succeed)
		if n := p.running(); n != 4 {
			t.Errorf("%d jobs run after 4 responses, want 4", n)
		}
		// Jobs 8-11 run; their responses give nothing more.
		for i := 8; i < 12; i++ {
			p.tell(i, send(0), succeed)
		}
		if n := p.running(); n != 4 {
			t.Errorf("%d jobs run after 8 responses, want the limit the lane opened at, 4", n)
		}
	})
}

// TestWaitTooLong: a Wait that the lane would keep longer than its maxWait
// fails at once with a TooLongError, and so does a Wait under way once a
// later 429 makes the lane wait that long. A Wait with a longer maxWait, or
// with none, waits.
func TestWaitTooLong(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		jobs := jobsOn("https://a.example.com", make([]int64, 5)...)
		p := runPuppets(context.Background(), jobs, Limits{HTTP2: 8, HTTP1: 8, Total: 64})
		defer p.finish()

		checkTooLong := func(i int, want time.Duration) {
			t.Helper()
			var tooLong *TooLongError
			if _, err := p.waitOf(i); !errors.As(err, &tooLong) || tooLong.Wait != want {
				t.Errorf("job %d: Wait = %v, want a TooLongError of %s", i, err, want)
			}
		}
		p.tell(0, connect("h2"), send(0))
		p.tell(4, send(0))
		p.tell(0, limited(90*time.Second))
		p.tell(1, send(60*time.Second))
		checkTooLong(1, 90*time.Second)
		p.tell(2, send(120*time.Second))
		p.tell(3, send(0))
		if _, err := p.waitOf(2); err != nil {
			t.Errorf("job 2: Wait = %v, want it to wait", err)
		}
		p.tell(4, limited(150*time.Second))
		checkTooLong(2, 150*time.Second)
		checkJobs(t, "sent before the wait ends", p.sentJobs(), []int{0, 4})

		time.Sleep(150 * time.Second)
		synctest.Wait()
		checkJobs(t, "sent once the wait ends", p.sentJobs(), []int{0, 4, 3})
		if waited, err := p.waitOf(3); waited != 150*time.Second || err != nil {
			t.Errorf("job 3: Wait = %s, %v, want 150s, nil", waited, err)
		}
	})
}

// TestYieldLendsThePlace: a job that gives its place up to wait lets a job of
// another lane take it, and gets a place back before a job of its own lane
// starts.
func TestYieldLendsThePlace(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Jobs 0-2 are on origin a, 3 on origin b, with two places in all.
		jobs := append(jobsOn("https://a.example.com", 0, 0, 0), jobsOn("https://b.example.com", 0)...)
		p := runPuppets(context.Background(), jobs, Limits{HTTP2: 3, HTTP1: 3, Total: 2})
		defer p.finish()

		p.tell(0, connect("h2"), send(0))
		checkJobs(t, "started", p.startedJobs(), []int{0, 3})
		p.tell(0, failed)
		checkJobs(t, "started once job 0 gave its place up", p.startedJobs(), []int{0, 3, 1})
		p.tell(0, send(0))
		checkJobs(t, "sent while every place is taken", p.sentJobs(), []int{0})
		p.tell(3, succeed)
		checkJobs(t, "sent once a place came free", p.sentJobs(), []int{0, 0})
		checkJobs(t, "started once a place came free", p.startedJobs(), []int{0, 3, 1})
	})
}

// TestDetach: a job with a context that Detach returned keeps its place while
// it waits to retry, and its 429 leaves its lane alone.
func TestDetach(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Job 0 is on origin a, job 1 on origin b, with one place in all.
		jobs := append(jobsOn("https://a.example.com", 0), jobsOn("https://b.example.com", 0)...)
		p := runPuppets(context.Background(), jobs, Limits{HTTP2: 2, HTTP1: 2, Total: 1})
		defer p.finish()

		p.tell(0, detached(send(0)), detached(limited(time.Hour)), detached(failed))
		checkJobs(t, "started", p.startedJobs(), []int{0})
		p.tell(0, send(0))
		checkJobs(t, "sent", p.sentJobs(), []int{0, 0})
		// Without Detach, the job gives its place up.
		p.tell(0, failed)
		checkJobs(t, "started once job 0 gave its place up", p.startedJobs(), []int{0, 1})
	})
}

// TestRunCanceledWhileWaiting: once the context is done, a job that waits out
// a 429 stops waiting, and Run returns without starting the jobs left, and
// without waiting the 429 out.
func TestRunCanceledWhileWaiting(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		jobs := jobsOn("https://a.example.com", 0, 0, 0, 0)
		p := runPuppets(ctx, jobs, Limits{HTTP2: 2, HTTP1: 2, Total: 64})
		start := time.Now()

		p.tell(0, connect("h2"), send(0), limited(time.Hour), end)
		p.tell(1, send(0))
		cancel()
		synctest.Wait()
		if _, err := p.waitOf(1); !errors.Is(err, context.Canceled) {
			t.Errorf("Wait = %v, want context.Canceled", err)
		}
		p.finish()
		checkJobs(t, "started", p.startedJobs(), []int{0, 1})
		if d := time.Since(start); d != 0 {
			t.Errorf("Run returned after %s, want at once", d)
		}
	})
}

// TestOutsideRun: a context that Run did not give waits for nothing, and tells
// no lane anything.
func TestOutsideRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	if waited, err := Wait(ctx, time.Second); waited != 0 || err != nil {
		t.Errorf("Wait = %s, %v, want 0, nil", waited, err)
	}
	Yield(ctx)
	RateLimited(ctx, time.Hour)
	Succeeded(ctx)
}
