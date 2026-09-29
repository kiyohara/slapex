// Package lane runs the asset downloads of an export in parallel, in one lane
// per origin (Issue #275, decision log 0067). An origin — scheme, host and
// port — is what an HTTP client keeps its connections by. A lane runs its
// first download alone until that download has a connection, so the ones
// after it share the connection instead of each opening their own. The lane
// then runs up to its limit at a time: higher for an origin whose connection
// speaks HTTP/2, where the downloads share it, than for one that speaks
// HTTP/1.1, where each needs a connection of its own. The lanes run side by
// side, within a limit over all of them.
package lane

import (
	"cmp"
	"context"
	"crypto/tls"
	"net"
	"net/http/httptrace"
	"net/url"
	"slices"
	"strings"
	"sync"
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
	// Total bounds the jobs of all the lanes together.
	Total int
	// Large bounds the jobs of one lane whose Size is LargeSize or more; the
	// smaller jobs take the rest of the lane's limit. A LargeSize of 0 counts
	// no job as large.
	Large     int
	LargeSize int64
}

// Defaults are the limits slapex downloads with. Decision log 0067 has the
// benchmark they come from (tools/assetbench).
var Defaults = Limits{HTTP2: 16, HTTP1: 6, Total: 64, Large: 4, LargeSize: 4 << 20}

// PerOrigin is the most jobs one lane runs at a time.
func (l Limits) PerOrigin() int { return max(l.HTTP2, l.HTTP1, 1) }

// Run calls do(ctx, i) for each job i, each on a goroutine of its own, and
// returns once every call it made has returned. The jobs of a lane start in
// order of size, the largest first, and those of unknown size after them in
// the order given. Run keeps to limits:
//
//   - A lane runs its first job alone until that job has a connection, which
//     the context Run gives do reports (httptrace GotConn). The lane's limit
//     is then HTTP2 or HTTP1, by the protocol the connection negotiated. A
//     first job that ends without a connection — it could not connect, or it
//     sent no request — leaves the lane at HTTP1.
//   - A lane runs at most Large jobs of LargeSize or more at a time.
//   - All the lanes together run at most Total jobs at a time, and the lanes
//     take turns at the places that come free.
//
// Once ctx is done, Run starts no more jobs: a job it has not started gets no
// call. do should return soon after ctx is done.
func Run(ctx context.Context, jobs []Job, limits Limits, do func(ctx context.Context, i int)) {
	s := &scheduler{ctx: ctx, jobs: jobs, limits: limits.normalized(), do: do}
	byOrigin := map[string]*lane{}
	for i, j := range jobs {
		key := origin(j.URL)
		l := byOrigin[key]
		if l == nil {
			l = &lane{limit: 1, probing: true}
			byOrigin[key] = l
			s.lanes = append(s.lanes, l)
		}
		l.queue = append(l.queue, i)
	}
	for _, l := range s.lanes {
		// A stable sort keeps the given order among equal sizes, and puts the
		// jobs of unknown size (0) last.
		slices.SortStableFunc(l.queue, func(a, b int) int { return cmp.Compare(jobs[b].Size, jobs[a].Size) })
	}
	s.mu.Lock()
	s.dispatchLocked()
	s.mu.Unlock()
	s.wg.Wait()
}

func (l Limits) normalized() Limits {
	l.HTTP2 = max(l.HTTP2, 1)
	l.HTTP1 = max(l.HTTP1, 1)
	l.Total = max(l.Total, 1)
	l.Large = max(l.Large, 1)
	return l
}

type scheduler struct {
	ctx    context.Context
	jobs   []Job
	limits Limits
	do     func(context.Context, int)
	wg     sync.WaitGroup

	mu      sync.Mutex
	lanes   []*lane // in the order of their first job
	next    int     // the lane that is offered a free place first
	running int     // over all the lanes
}

// lane is the jobs of one origin.
type lane struct {
	queue   []int // the jobs not started, in the order they start
	running int
	large   int // running jobs of LargeSize or more
	limit   int
	// probing is set while the lane runs its first job alone, until that job
	// has a connection or ends.
	probing bool
}

// dispatchLocked starts the jobs the limits allow, one per lane in turn.
func (s *scheduler) dispatchLocked() {
	if s.ctx.Err() != nil {
		return
	}
	for {
		started := false
		first := s.next
		for k := range len(s.lanes) {
			if s.running >= s.limits.Total {
				return
			}
			idx := (first + k) % len(s.lanes)
			if s.startLocked(s.lanes[idx]) {
				started = true
				s.next = (idx + 1) % len(s.lanes)
			}
		}
		if !started {
			return
		}
	}
}

// startLocked starts the next job of l, when its limits let it, and reports
// whether it did.
func (s *scheduler) startLocked(l *lane) bool {
	if len(l.queue) == 0 || l.running >= l.limit {
		return false
	}
	// The large jobs come first in the queue: when l runs as many as it may,
	// the first small one takes the place.
	k := 0
	if s.isLarge(l.queue[0]) && l.large >= s.limits.Large {
		k = slices.IndexFunc(l.queue, func(i int) bool { return !s.isLarge(i) })
		if k < 0 {
			return false
		}
	}
	i := l.queue[k]
	l.queue = slices.Delete(l.queue, k, k+1)
	large := s.isLarge(i)
	l.running++
	s.running++
	if large {
		l.large++
	}
	ctx := s.ctx
	leader := l.probing
	if leader {
		ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) { s.connected(l, info) },
		})
	}
	s.wg.Add(1)
	go s.run(ctx, l, i, large, leader)
	return true
}

func (s *scheduler) isLarge(i int) bool {
	return s.limits.LargeSize > 0 && s.jobs[i].Size >= s.limits.LargeSize
}

// run runs job i of l and gives its place to the jobs waiting.
func (s *scheduler) run(ctx context.Context, l *lane, i int, large, leader bool) {
	defer s.wg.Done()
	s.do(ctx, i)
	s.mu.Lock()
	defer s.mu.Unlock()
	l.running--
	s.running--
	if large {
		l.large--
	}
	if leader && l.probing {
		s.openLocked(l, s.limits.HTTP1)
	}
	s.dispatchLocked()
}

// connected opens l once the first job of l has a connection. The hook sees
// every connection of that job — a retry's, a redirect's — and only the first
// one, while the job runs, counts.
func (s *scheduler) connected(l *lane, info httptrace.GotConnInfo) {
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

func (s *scheduler) openLocked(l *lane, limit int) {
	l.probing = false
	l.limit = limit
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
