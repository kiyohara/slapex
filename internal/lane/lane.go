// Package lane runs the asset downloads of an export in parallel, in one lane
// per origin (Issue #275, decision log 0067). An origin — scheme, host and
// port — is what an HTTP client keeps its connections by. A lane runs its
// first download alone until that download has a connection, so the ones
// after it share the connection instead of each opening their own. The lane
// then runs up to its limit at a time: higher for an origin whose connection
// speaks HTTP/2, where the downloads share it, than for one that speaks
// HTTP/1.1, where each needs a connection of its own. The lanes run side by
// side, within a limit over all of them.
//
// A lane also answers for its origin's rate limit (Issue #276, decision log
// 0068). A download holds a place in its lane while it has a request out or
// reads a body, and gives the place up while it waits to retry, so that it
// neither keeps the other origins waiting nor comes back to a lane that has
// to send less. When the origin answers 429, the lane halves its limit, and
// for the time the answer's Retry-After asks, sends no request: a download of
// the lane waits for the lane before each request (Wait). The client that
// sends the requests tells the lane what they got (Yield, RateLimited,
// Succeeded) through the context Run gives each download.
//
// A Scheduler runs the lanes of a whole export (Issue #279, decision log
// 0069): it takes downloads as they become known — those sent ahead of the
// Assets phase, then that phase's own — and runs them all within one set of
// limits. Run runs one set of downloads through a Scheduler of its own.
package lane

import (
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http/httptrace"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// Job is one download.
type Job struct {
	// URL is what the job downloads. Its origin picks the job's lane.
	URL string
	// Size is the job's size in bytes when it is known before the download
	// (a Slack file), and 0 when it is not.
	Size int64
}

// Limits bound how many jobs run at a time. A limit below 1 counts as 1.
type Limits struct {
	// HTTP2 bounds the jobs of an origin whose first connection negotiated
	// HTTP/2, and HTTP1 those of any other origin.
	HTTP2, HTTP1 int
	// Total bounds the jobs of all the lanes together that hold a place.
	Total int
	// Large bounds the jobs of one lane whose Size is LargeSize or more; the
	// smaller jobs take the rest of the lane's limit. A LargeSize of 0 counts
	// no job as large.
	Large     int
	LargeSize int64
	// RestoreAfter is how many responses (Succeeded) give a lane back one of
	// the places its 429s took (RateLimited), up to the limit it opened at.
	// Only the responses to requests sent since the latest 429 that halved
	// the limit count. 0 gives no place back.
	RestoreAfter int
}

// Defaults are the limits slapex downloads with. Decision logs 0067 and 0068
// have the benchmarks they come from (tools/assetbench).
var Defaults = Limits{HTTP2: 16, HTTP1: 6, Total: 64, Large: 4, LargeSize: 4 << 20, RestoreAfter: 4}

// PerOrigin is the most jobs one lane runs at a time.
func (l Limits) PerOrigin() int { return max(l.HTTP2, l.HTTP1, 1) }

// Run calls do(ctx, i) for each job i, each on a goroutine of its own, and
// returns once every call it made has returned. The jobs of a lane start in
// order of size, the largest first, and those of unknown size after them in
// the order given. A job starts with a place, and holds it until it returns
// or gives it up to wait (Yield, Wait). Run keeps to limits:
//
//   - A lane runs its first job alone until that job has a connection, which
//     the context Run gives do reports (httptrace GotConn). The lane's limit
//     is then HTTP2 or HTTP1, by the protocol the connection negotiated. A
//     first job that ends without a connection — it could not connect, or it
//     sent no request — leaves the lane at HTTP1.
//   - A lane starts a job while it has fewer jobs under way than its limit,
//     counting those that wait for a place, and gives a place while fewer
//     than its limit hold one. A 429 halves the limit, and the responses
//     after it give it back (RateLimited, Succeeded, RestoreAfter).
//   - A lane gives at most Large jobs of LargeSize or more a place at a time.
//   - All the lanes together give at most Total jobs a place at a time, and
//     take turns at the places that come free. In a lane, a job that waits
//     for its place back gets it before a job starts, and the jobs that wait
//     get theirs in the order they started.
//   - A lane that waits out a 429 gives no job a place: a job that starts in
//     it starts without one, and waits for one (Wait).
//
// Once ctx is done, Run starts no more jobs: a job it has not started gets no
// call. do should return soon after ctx is done.
func Run(ctx context.Context, jobs []Job, limits Limits, do func(ctx context.Context, i int)) {
	s := NewScheduler(ctx, limits)
	defer s.Close()
	s.Add(jobs, do).Wait()
}

// Scheduler runs jobs the way Run does, and takes them over time: the jobs
// that Add adds wait in their lanes with the jobs added before them, within
// the same limits. A lane keeps what its first job found out — the limit its
// connection allows, a 429's wait and halved limit — for the jobs added
// after.
type Scheduler struct {
	ctx    context.Context // done once the scheduler's context is, or Close
	cancel context.CancelFunc
	limits Limits
	wg     sync.WaitGroup // the calls under way

	mu       sync.Mutex
	closed   bool
	byOrigin map[string]*lane
	lanes    []*lane // in the order of their first job
	next     int     // the lane that is offered a free place first
	holding  int     // jobs that hold a place, over all the lanes
	started  int     // jobs started, which numbers them in start order
}

// NewScheduler returns a Scheduler that runs its jobs within limits, until
// ctx is done or Close. Close releases it.
func NewScheduler(ctx context.Context, limits Limits) *Scheduler {
	s := &Scheduler{limits: limits.normalized(), byOrigin: map[string]*lane{}}
	s.ctx, s.cancel = context.WithCancel(ctx)
	context.AfterFunc(s.ctx, s.drop)
	return s
}

// Group is the jobs of one Add.
type Group struct {
	s      *Scheduler
	ctx    context.Context // the context of the group's calls, which Stop cancels
	cancel context.CancelFunc
	do     func(context.Context, int)
	done   chan struct{} // closed once no job of the group is under way or waits to start

	pending int // guarded by s.mu: the jobs under way or waiting to start
}

// queued is a job that waits in its lane to start.
type queued struct {
	job   Job
	g     *Group
	i     int  // the job's index in the jobs its Add took
	large bool // Size is LargeSize or more
}

// Add adds jobs to their lanes and returns them as a Group. Each job i is run
// the way Run runs it, by a call of do(ctx, i) on a goroutine of its own, and
// starts in its lane after the jobs added before it of the same size or
// larger, before those smaller or of unknown size. Once the scheduler's
// context is done or Close has been called, Add adds nothing.
func (s *Scheduler) Add(jobs []Job, do func(ctx context.Context, i int)) *Group {
	g := &Group{s: s, do: do, done: make(chan struct{})}
	g.ctx, g.cancel = context.WithCancel(s.ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil || len(jobs) == 0 {
		g.endLocked(0)
		return g
	}
	g.pending = len(jobs)
	for i, j := range jobs {
		key := origin(j.URL)
		l := s.byOrigin[key]
		if l == nil {
			l = &lane{limit: 1, probing: true}
			s.byOrigin[key] = l
			s.lanes = append(s.lanes, l)
		}
		// The queue is in order of size, the largest first: the job goes
		// after those of its size, and the jobs of unknown size (0) stay
		// last in the order they came.
		at := slices.IndexFunc(l.queue, func(q *queued) bool { return q.job.Size < j.Size })
		if at < 0 {
			at = len(l.queue)
		}
		l.queue = slices.Insert(l.queue, at, &queued{job: j, g: g, i: i, large: s.isLarge(j.Size)})
	}
	s.dispatchLocked()
	return g
}

// Wait waits until every job of g has ended: each call it made has returned,
// and the jobs it did not start never will (Stop, or the scheduler's context
// is done).
func (g *Group) Wait() { <-g.done }

// Stop drops the jobs of g that have not started, and cancels the context of
// the calls under way, which should return soon.
func (g *Group) Stop() {
	s := g.s
	s.mu.Lock()
	for _, l := range s.lanes {
		n := len(l.queue)
		l.queue = slices.DeleteFunc(l.queue, func(q *queued) bool { return q.g == g })
		g.endLocked(n - len(l.queue))
	}
	s.mu.Unlock()
	g.cancel()
}

// endLocked counts n more jobs of g ended, and marks g done once none is
// left.
func (g *Group) endLocked(n int) {
	g.pending -= n
	if g.pending == 0 && !g.ended() {
		close(g.done)
		g.cancel()
	}
}

func (g *Group) ended() bool {
	select {
	case <-g.done:
		return true
	default:
		return false
	}
}

// drop drops the jobs that wait to start, once the scheduler's context is
// done: they will not start.
func (s *Scheduler) drop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.lanes {
		for _, q := range l.queue {
			q.g.endLocked(1)
		}
		l.queue = nil
	}
}

// Close cancels the calls under way, drops the jobs that wait to start, and
// returns once every call the scheduler made has returned. The scheduler
// takes no jobs after it.
func (s *Scheduler) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	s.drop()
	s.wg.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.lanes {
		if l.timer != nil {
			l.timer.Stop()
		}
	}
}

func (l Limits) normalized() Limits {
	l.HTTP2 = max(l.HTTP2, 1)
	l.HTTP1 = max(l.HTTP1, 1)
	l.Total = max(l.Total, 1)
	l.Large = max(l.Large, 1)
	l.RestoreAfter = max(l.RestoreAfter, 0)
	return l
}

// lane is the jobs of one origin.
type lane struct {
	queue   []*queued // the jobs not started, in the order they start
	holding int       // jobs under way that hold a place
	waiting int       // jobs under way that do not
	ready   []*job    // the waiting jobs that want their place back (Wait), in start order
	large   int       // jobs of LargeSize or more that hold a place
	limit   int
	opened  int // the limit the lane opened at, which RestoreAfter gives back
	// probing is set while the lane runs its first job alone, until that job
	// has a connection or ends.
	probing bool
	// paused is set while the lane waits out a 429, until resume, which the
	// timer calls at until. asked is when the 429s asked to wait until,
	// without the jitter that until adds, which Wait holds a job's maxWait
	// to.
	paused bool
	until  time.Time
	asked  time.Time
	timer  *time.Timer
	// gen counts the 429s that halved limit: a job's 429 halves it only when
	// no other 429 has halved it since the job got its place, and a job's
	// response counts towards RestoreAfter under the same condition.
	gen int
	// succeeded counts the responses towards the next place RestoreAfter
	// gives back.
	succeeded int
}

// job is one job under way. The context Run gives do carries it (jobKey), for
// Wait, Yield, RateLimited and Succeeded.
type job struct {
	s      *Scheduler
	l      *lane
	q      *queued
	order  int  // its place in the start order
	large  bool // Size is LargeSize or more
	leader bool // started while its lane was probing

	// Guarded by s.mu.
	holds bool
	gen   int // the lane's gen when the job got its place
	ended bool
	// While the job is in l.ready: closed when it gets its place, or when
	// err says why it will not.
	wake    chan struct{}
	maxWait time.Duration
	err     error
}

type jobKey struct{}

// jobOf returns the job ctx is for, or nil.
func jobOf(ctx context.Context) *job {
	j, _ := ctx.Value(jobKey{}).(*job)
	return j
}

// Detach returns ctx for a job that does not answer to its lane: Wait, Yield,
// RateLimited and Succeeded do nothing with it, so the job keeps its place
// from its start to its end and its 429s leave the lane alone, as in the
// lanes of #275. tools/assetbench compares the two.
func Detach(ctx context.Context) context.Context {
	return context.WithValue(ctx, jobKey{}, (*job)(nil))
}

// dispatchLocked lets the jobs go on that the limits allow, one per lane in
// turn.
func (s *Scheduler) dispatchLocked() {
	if s.ctx.Err() != nil {
		return
	}
	for {
		admitted := false
		first := s.next
		for k := range len(s.lanes) {
			idx := (first + k) % len(s.lanes)
			if s.admitLocked(s.lanes[idx]) {
				admitted = true
				s.next = (idx + 1) % len(s.lanes)
			}
		}
		if !admitted {
			return
		}
	}
}

// admitLocked lets the next job of l go on, when its limits let it — a job
// that waits for its place back gets it before a job starts — and reports
// whether it did.
func (s *Scheduler) admitLocked(l *lane) bool {
	if !l.paused && l.holding < l.limit && s.holding < s.limits.Total {
		for k, j := range l.ready {
			if j.large && l.large >= s.limits.Large {
				continue
			}
			l.ready = slices.Delete(l.ready, k, k+1)
			l.waiting--
			s.holdLocked(j)
			close(j.wake)
			return true
		}
	}
	return s.startLocked(l)
}

// startLocked starts the next job of l, when its limits let it, and reports
// whether it did. A job that starts in a lane that waits out a 429 starts
// without a place, and waits for one (Wait).
func (s *Scheduler) startLocked(l *lane) bool {
	if len(l.queue) == 0 || l.holding+l.waiting >= l.limit {
		return false
	}
	k := 0
	if !l.paused {
		if s.holding >= s.limits.Total {
			return false
		}
		// The large jobs come first in the queue: when l runs as many as it
		// may, the first small one takes the place.
		if l.queue[0].large && l.large >= s.limits.Large {
			k = slices.IndexFunc(l.queue, func(q *queued) bool { return !q.large })
			if k < 0 {
				return false
			}
		}
	}
	q := l.queue[k]
	l.queue = slices.Delete(l.queue, k, k+1)
	j := &job{s: s, l: l, q: q, order: s.started, large: q.large, leader: l.probing}
	s.started++
	if l.paused {
		l.waiting++
	} else {
		s.holdLocked(j)
	}
	ctx := context.WithValue(q.g.ctx, jobKey{}, j)
	if j.leader {
		ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) { s.connected(l, info) },
		})
	}
	s.wg.Add(1)
	go s.run(ctx, j)
	return true
}

func (s *Scheduler) isLarge(size int64) bool {
	return s.limits.LargeSize > 0 && size >= s.limits.LargeSize
}

// holdLocked gives j a place in its lane.
func (s *Scheduler) holdLocked(j *job) {
	j.holds = true
	j.gen = j.l.gen
	j.l.holding++
	s.holding++
	if j.large {
		j.l.large++
	}
}

// yieldLocked takes j's place, if it holds one, and leaves it waiting.
func (s *Scheduler) yieldLocked(j *job) {
	if !j.holds {
		return
	}
	j.holds = false
	j.l.holding--
	s.holding--
	if j.large {
		j.l.large--
	}
	j.l.waiting++
}

// run runs job j and gives its place to the jobs waiting.
func (s *Scheduler) run(ctx context.Context, j *job) {
	defer s.wg.Done()
	j.q.g.do(ctx, j.q.i)
	s.mu.Lock()
	defer s.mu.Unlock()
	l := j.l
	if j.holds {
		j.holds = false
		l.holding--
		s.holding--
		if j.large {
			l.large--
		}
	} else {
		l.waiting--
		s.unreadyLocked(j)
	}
	j.ended = true
	if j.leader && l.probing {
		s.openLocked(l, s.limits.HTTP1)
	}
	j.q.g.endLocked(1)
	s.dispatchLocked()
}

// connected opens l once the first job of l has a connection. The hook sees
// every connection of that job — a retry's, a redirect's — and only the first
// one, while the job runs, counts.
func (s *Scheduler) connected(l *lane, info httptrace.GotConnInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !l.probing {
		return
	}
	limit := s.limits.HTTP1
	if negotiatedHTTP2(info) {
		limit = s.limits.HTTP2
	}
	s.openLocked(l, limit)
	s.dispatchLocked()
}

func (s *Scheduler) openLocked(l *lane, limit int) {
	l.probing = false
	l.limit = limit
	l.opened = limit
}

// TooLongError is Wait's error when the 429s that the job's lane waits out
// ask it to wait longer than the job may wait.
type TooLongError struct {
	// Wait is how much longer the 429s ask the lane to wait, without the
	// jitter it waits on top of that.
	Wait time.Duration
}

func (e *TooLongError) Error() string {
	return fmt.Sprintf("the lane waits out a 429 that asks for %s more", e.Wait)
}

// Wait waits until the job ctx is for may send its next request: until the
// job holds a place in its lane, and the lane does not wait out a 429. A job
// that holds a place in a lane that waits gives the place up to wait. Wait
// returns how long it waited, which is 0 when it did not.
//
// When the 429s that the lane waits out ask it to wait longer than maxWait
// from now (a maxWait of 0 allows any wait), Wait returns a TooLongError at
// once, and so does a Wait under way when a later 429 asks for that long.
// What it holds to maxWait is the wait the 429s asked for, without the jitter
// that the lane waits on top of it (RateLimited), so that a job that may wait
// what a 429 asks waits it out. It returns ctx's error once ctx is done. A
// context that Run did not give (or that Detach returned) waits for nothing.
func Wait(ctx context.Context, maxWait time.Duration) (time.Duration, error) {
	j := jobOf(ctx)
	if j == nil {
		return 0, nil
	}
	s, l := j.s, j.l
	s.mu.Lock()
	if err := tooLong(l, maxWait); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	if j.holds {
		if !l.paused {
			s.mu.Unlock()
			return 0, nil
		}
		s.yieldLocked(j)
	}
	j.wake, j.maxWait, j.err = make(chan struct{}), maxWait, nil
	at, _ := slices.BinarySearchFunc(l.ready, j.order, func(r *job, order int) int { return cmp.Compare(r.order, order) })
	l.ready = slices.Insert(l.ready, at, j)
	s.dispatchLocked()
	if j.holds {
		s.mu.Unlock()
		return 0, nil
	}
	wake := j.wake
	s.mu.Unlock()

	start := time.Now()
	select {
	case <-wake:
	case <-ctx.Done():
		s.mu.Lock()
		s.unreadyLocked(j)
		s.mu.Unlock()
		return time.Since(start), ctx.Err()
	}
	s.mu.Lock()
	err := j.err
	s.mu.Unlock()
	return time.Since(start), err
}

// tooLong returns the TooLongError of a job of l that may wait maxWait, or
// nil. It holds the wait the 429s asked for to maxWait.
func tooLong(l *lane, maxWait time.Duration) error {
	if maxWait > 0 && l.paused {
		if rest := time.Until(l.asked); rest > maxWait {
			return &TooLongError{Wait: rest}
		}
	}
	return nil
}

// unreadyLocked takes j out of its lane's ready jobs, if it is there.
func (s *Scheduler) unreadyLocked(j *job) {
	if k := slices.Index(j.l.ready, j); k >= 0 {
		j.l.ready = slices.Delete(j.l.ready, k, k+1)
	}
}

// Yield gives up the place in its lane that the job ctx is for holds, for a
// wait before it retries; Wait gets it a place again. It does nothing for a
// job that holds none, or for a context that Run did not give.
func Yield(ctx context.Context) {
	j := jobOf(ctx)
	if j == nil {
		return
	}
	s := j.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if j.ended || !j.holds {
		return
	}
	s.yieldLocked(j)
	s.dispatchLocked()
}

// RateLimited tells the lane of the job ctx is for that its origin answered
// the job's request with 429, which asks to wait asked before the next
// request (0 when the answer asks for no time), and which the job waits out
// for wait: asked and the jitter the job adds to it. The lane halves its
// limit, unless another 429 has halved it since the job got its place, and
// gives no job a place until wait has passed, as the job waits; a later 429
// that asks for longer makes it wait longer. It does nothing for a context
// that Run did not give.
func RateLimited(ctx context.Context, asked, wait time.Duration) {
	j := jobOf(ctx)
	if j == nil {
		return
	}
	s, l := j.s, j.l
	s.mu.Lock()
	defer s.mu.Unlock()
	if j.ended {
		return
	}
	if !l.probing && j.gen == l.gen {
		l.limit = max(l.limit/2, 1)
		l.gen++
		l.succeeded = 0
	}
	if wait <= 0 {
		return
	}
	now := time.Now()
	if a := now.Add(asked); !l.paused || a.After(l.asked) {
		l.asked = a
	}
	if u := now.Add(wait); !l.paused || u.After(l.until) {
		l.until = u
	}
	if !l.paused {
		l.paused = true
		if l.timer == nil {
			l.timer = time.AfterFunc(wait, func() { s.resume(l) })
		} else {
			l.timer.Reset(wait)
		}
	}
	// A job that may not wait that long is told so at once.
	for k := 0; k < len(l.ready); {
		r := l.ready[k]
		if err := tooLong(l, r.maxWait); err != nil {
			l.ready = slices.Delete(l.ready, k, k+1)
			r.err = err
			close(r.wake)
			continue
		}
		k++
	}
}

// resume ends l's wait out of a 429 once its time has passed, and gives the
// places to the jobs waiting. The timer calls it at the end of the wait; the
// wait may have grown since it was set.
func (s *Scheduler) resume(l *lane) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !l.paused || s.ctx.Err() != nil {
		return
	}
	if rest := time.Until(l.until); rest > 0 {
		l.timer.Reset(rest)
		return
	}
	l.paused = false
	s.dispatchLocked()
}

// Succeeded tells the lane of the job ctx is for that the job's request got
// its response. Once RestoreAfter responses have come to jobs that got their
// places since the latest 429 that halved the limit, the lane gets one place
// back, up to the limit it opened at. It does nothing for a context that Run
// did not give.
func Succeeded(ctx context.Context) {
	j := jobOf(ctx)
	if j == nil {
		return
	}
	s, l := j.s, j.l
	s.mu.Lock()
	defer s.mu.Unlock()
	if j.ended || s.limits.RestoreAfter == 0 || j.gen != l.gen || l.limit >= l.opened {
		return
	}
	l.succeeded++
	if l.succeeded < s.limits.RestoreAfter {
		return
	}
	l.succeeded = 0
	l.limit++
	s.dispatchLocked()
}

// negotiatedHTTP2 reports whether the connection of info negotiated HTTP/2.
// net/http hands GotConn the TLS connection of an https request, whether the
// request goes over HTTP/2 or HTTP/1.1.
func negotiatedHTTP2(info httptrace.GotConnInfo) bool {
	c, ok := info.Conn.(interface{ ConnectionState() tls.ConnectionState })
	return ok && c.ConnectionState().NegotiatedProtocol == "h2"
}

// origin is the scheme, host and port of rawURL, in lower case, with the
// scheme's default port when rawURL names none; rawURL itself when it has no
// host.
func origin(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" {
		switch scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return scheme + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}
