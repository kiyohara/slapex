package slack

// Method lanes (Issue #278, PF-06 of #272; decision log 0069). The Web API
// calls of one method run in a lane of their own: one at a time, in the order
// they come, each once the previous one has ended — its retries and its waits
// for a 429's Retry-After included — and at least methodPace after the
// previous one started (the pacing of decision log 0025). The calls of
// different methods run side by side. The export sends some of its calls
// ahead of the stage that takes their results (internal/export), so the calls
// of one method may come from several goroutines at once.

import (
	"context"
	"slices"
	"sync"
	"time"
)

// methodLanes are the lanes of a client's Web API methods. The zero value is
// ready to use.
type methodLanes struct {
	mu    sync.Mutex
	lanes map[string]*methodLane
}

// methodLane is the lane of one method.
type methodLane struct {
	// busy is set while a call holds the lane. leave hands the lane to the
	// first call that waits for it, if any, and busy stays set.
	busy bool
	// waiting are the calls that wait for the lane, in the order they came:
	// a call's channel is closed when the lane passes to it.
	waiting []chan struct{}
	// start is when the latest call to hold the lane started, after its
	// pacing wait; zero before the lane's first call. Only the call that
	// holds the lane uses it (pace).
	start time.Time
}

// enter waits for the lane of method, and returns it for the call to hold
// until leave. The calls of a method get its lane in the order they come. A
// call whose ctx is done gets no lane: it stops waiting, or does not start
// to, and enter returns ctx's error.
func (m *methodLanes) enter(ctx context.Context, method string) (*methodLane, error) {
	m.mu.Lock()
	if m.lanes == nil {
		m.lanes = map[string]*methodLane{}
	}
	l := m.lanes[method]
	if l == nil {
		l = &methodLane{}
		m.lanes[method] = l
	}
	var turn chan struct{}
	if l.busy {
		turn = make(chan struct{})
		l.waiting = append(l.waiting, turn)
	} else {
		l.busy = true
	}
	m.mu.Unlock()

	if turn != nil {
		select {
		case <-turn:
		case <-ctx.Done():
		}
	}
	if err := ctx.Err(); err != nil {
		m.abandon(l, turn)
		return nil, err
	}
	return l, nil
}

// abandon takes back a call whose context ended: out of the lane's queue when
// it still waits there, or, when the lane had passed to it (turn closed, or
// no turn to wait for), out of the lane, which passes to the next call.
func (m *methodLanes) abandon(l *methodLane, turn chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if turn != nil {
		if i := slices.Index(l.waiting, turn); i >= 0 {
			l.waiting = slices.Delete(l.waiting, i, i+1)
			return
		}
	}
	m.leaveLocked(l)
}

// leave gives up the lane a call held, to the first call that waits for it.
func (m *methodLanes) leave(l *methodLane) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leaveLocked(l)
}

func (m *methodLanes) leaveLocked(l *methodLane) {
	if len(l.waiting) == 0 {
		l.busy = false
		return
	}
	next := l.waiting[0]
	l.waiting = slices.Delete(l.waiting, 0, 1)
	close(next)
}

// pace waits, once a call holds the lane l, until methodPace has passed since
// the lane's previous call started, and marks the call's start. The wait goes
// through the client's sleeper, so the HTTP trace counts it as the call's
// pacing wait; the wait for the lane (enter) is not counted, as a download's
// wait for lane.Run to start it is not (decision log 0069).
func (c *Client) pace(ctx context.Context, l *methodLane) error {
	if !l.start.IsZero() {
		if wait := methodPace - time.Since(l.start); wait > 0 {
			if err := c.sleep(ctx, wait); err != nil {
				return err
			}
		}
	}
	l.start = time.Now()
	return nil
}
