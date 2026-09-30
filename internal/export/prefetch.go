// Prefetch of the Web API requests (Issue #278, PF-06 of #272; decision log
// 0069, doc/design/slack-api-usage.md「取得の並行化」). Run still takes its
// stages one after another, in the same order (the driver), but from the
// Messages phase on the stages make their conversations.replies, users.info,
// bots.info and emoji.list requests through a prefetcher. It sends each
// request that a later stage is certain to make as soon as that is certain
// (the prefetch), on a goroutine of its own, and hands the stage the result
// when the stage gets there. A request goes out once, whichever asks for it
// first; the method lanes of slack.Client pace the requests of a method and
// run those of different methods side by side.
//
// A prefetched request's notices — its retries and rate limit waits — are
// held until the driver takes its result, and pass straight on from then,
// while the request is still under way, so that they come where a serial
// export prints them. Its error, too, reaches the driver only there, which
// handles it as the serial export does. When Run returns, the prefetcher
// stops the requests still under way and waits for them; the results and
// notices no stage took are dropped.

package export

import (
	"context"
	"sync"

	"github.com/kiyohara/slapex/internal/slack"
)

// prefetchOffKey is the context key of a bool that, when true, turns the
// prefetch off: each request goes out when a stage asks for it, as in the
// serial export. Only tests set it, to compare the two (Issue #278).
type prefetchOffKey struct{}

// requestTakenKey is the context key of a func(key requestKey, ahead bool)
// that the prefetcher calls each time a stage has taken the result of a
// request, with whether the request went ahead of the stage. Only tests set
// it: to see what went ahead, and to stop a run at the same point of the
// driver with the prefetch and without it (Issue #278).
type requestTakenKey struct{}

// requestKey names a request of the prefetcher: its Web API method and what
// it asks for.
type requestKey struct {
	method string
	arg    string
}

// prefetcher makes the Web API requests of the stages from the Messages phase
// on, and sends ahead the ones a later stage is certain to make (see above).
// The driver calls its methods; the prefetched requests run on goroutines of
// their own.
type prefetcher struct {
	client *slack.Client
	reuse  *reusableCache // users and bots it does not ask for; nil without --reuse-cache
	// ctx is Run's, which stop cancels: the prefetched requests run with it.
	ctx    context.Context
	cancel context.CancelFunc
	off    bool                   // prefetchOffKey
	taken  func(requestKey, bool) // requestTakenKey; nil unless a test set it
	wg     sync.WaitGroup         // the prefetched requests under way

	mu       sync.Mutex
	stopped  bool
	requests map[requestKey]*request
}

// request is one request of the prefetcher. done is closed once val and err
// hold its result.
type request struct {
	done chan struct{}
	val  any
	err  error

	mu sync.Mutex
	// held are the notices of a prefetched request that no stage has taken,
	// and logf is where its notices go once one has.
	held []heldNotice
	logf func(format string, args ...any)
}

type heldNotice struct {
	format string
	args   []any
}

// newPrefetcher returns the prefetcher of a run whose context is ctx. stop
// ends it.
func newPrefetcher(ctx context.Context, client *slack.Client, reuse *reusableCache) *prefetcher {
	f := &prefetcher{client: client, reuse: reuse, requests: map[requestKey]*request{}}
	f.ctx, f.cancel = context.WithCancel(ctx)
	f.off, _ = ctx.Value(prefetchOffKey{}).(bool)
	f.taken, _ = ctx.Value(requestTakenKey{}).(func(requestKey, bool))
	return f
}

// stop cancels the prefetched requests still under way, waits for them to
// end, and sends no more: their results and notices are dropped.
func (f *prefetcher) stop() {
	f.mu.Lock()
	f.stopped = true
	f.mu.Unlock()
	f.cancel()
	f.wg.Wait()
}

// prefetch sends the request key names ahead, through fetch, unless it has
// gone out already.
func prefetch[T any](f *prefetcher, key requestKey, fetch func(context.Context) (T, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.off || f.stopped || f.requests[key] != nil {
		return
	}
	r := &request{done: make(chan struct{})}
	f.requests[key] = r
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		r.val, r.err = fetch(slack.WithNotices(f.ctx, r.notef))
		close(r.done)
	}()
}

// take returns the result of the request key names, for the stage the driver
// is at: that of the request sent ahead, which it waits for while it is under
// way, passing its notices on, or, when none was, that of the request made
// now with ctx, as the serial export makes it.
func take[T any](ctx context.Context, f *prefetcher, key requestKey, fetch func(context.Context) (T, error)) (T, error) {
	f.mu.Lock()
	r := f.requests[key]
	ahead := r != nil
	if !ahead {
		r = &request{done: make(chan struct{})}
		f.requests[key] = r
	}
	f.mu.Unlock()
	if ahead {
		r.pass(f.client.Logf)
	} else {
		r.val, r.err = fetch(ctx)
		close(r.done)
	}
	<-r.done
	if f.taken != nil {
		f.taken(key, ahead)
	}
	val, _ := r.val.(T)
	return val, r.err
}

// notef holds a notice of a prefetched request until a stage takes the
// request, and passes it on after.
func (r *request) notef(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.logf != nil {
		r.logf(format, args...)
		return
	}
	r.held = append(r.held, heldNotice{format: format, args: args})
}

// pass passes the notices the request holds on to logf, and those it makes
// from now on.
func (r *request) pass(logf func(format string, args ...any)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.held {
		logf(n.format, n.args...)
	}
	r.held = nil
	r.logf = logf
}

// threadResult is what conversations.replies returned for a thread
// (slack.Client.Thread).
type threadResult struct {
	parent    *slack.Message
	replies   []slack.Message
	truncated bool
}

func threadKey(channelID, threadTS string) requestKey {
	return requestKey{method: "conversations.replies", arg: channelID + " " + threadTS}
}

func (f *prefetcher) fetchThread(channelID, threadTS string) func(context.Context) (threadResult, error) {
	return func(ctx context.Context) (threadResult, error) {
		parent, replies, truncated, err := f.client.Thread(ctx, channelID, threadTS, maxThreadReplies)
		return threadResult{parent: parent, replies: replies, truncated: truncated}, err
	}
}

// thread returns the thread's conversations.replies, up to maxThreadReplies
// replies.
func (f *prefetcher) thread(ctx context.Context, channelID, threadTS string) (threadResult, error) {
	return take(ctx, f, threadKey(channelID, threadTS), f.fetchThread(channelID, threadTS))
}

// prefetchThread sends the thread's conversations.replies ahead. With shown,
// every reply it returns is shown (no emoji filter is on), so the users.info
// and bots.info of the people the replies show go ahead as soon as the
// replies have come.
func (f *prefetcher) prefetchThread(channelID, threadTS string, shown bool) {
	fetch := f.fetchThread(channelID, threadTS)
	prefetch(f, threadKey(channelID, threadTS), func(ctx context.Context) (threadResult, error) {
		result, err := fetch(ctx)
		if err == nil && shown {
			f.prefetchPeople(nil, map[string][]slack.Message{threadTS: result.replies})
		}
		return result, err
	})
}

func (f *prefetcher) userInfo(ctx context.Context, id string) (*slack.User, error) {
	return take(ctx, f, requestKey{method: "users.info", arg: id}, func(ctx context.Context) (*slack.User, error) {
		return f.client.UserInfo(ctx, id)
	})
}

func (f *prefetcher) botInfo(ctx context.Context, id string) (*slack.Bot, error) {
	return take(ctx, f, requestKey{method: "bots.info", arg: id}, func(ctx context.Context) (*slack.Bot, error) {
		return f.client.BotInfo(ctx, id)
	})
}

// prefetchUsers sends ahead the users.info of the users the reuse cache does
// not hold, as lookupUsers asks for them.
func (f *prefetcher) prefetchUsers(ids []string) {
	for _, id := range ids {
		if f.reuse != nil {
			if _, ok := f.reuse.users[id]; ok {
				continue
			}
		}
		prefetch(f, requestKey{method: "users.info", arg: id}, func(ctx context.Context) (*slack.User, error) {
			return f.client.UserInfo(ctx, id)
		})
	}
}

// prefetchBots sends ahead the bots.info of the bots the reuse cache does not
// hold, as lookupBots asks for them.
func (f *prefetcher) prefetchBots(ids []string) {
	for _, id := range ids {
		if f.reuse != nil {
			if _, ok := f.reuse.bots[id]; ok {
				continue
			}
		}
		prefetch(f, requestKey{method: "bots.info", arg: id}, func(ctx context.Context) (*slack.Bot, error) {
			return f.client.BotInfo(ctx, id)
		})
	}
}

// prefetchPeople sends ahead the users.info and bots.info the Users stage
// asks for to show messages and replies (collectUserIDs, collectBotIDs).
func (f *prefetcher) prefetchPeople(messages []slack.Message, replies map[string][]slack.Message) {
	f.prefetchUsers(collectUserIDs(messages, replies))
	f.prefetchBots(collectBotIDs(messages, replies))
}

var emojiListKey = requestKey{method: "emoji.list"}

func (f *prefetcher) emojiList(ctx context.Context) (map[string]string, error) {
	return take(ctx, f, emojiListKey, f.client.EmojiList)
}

// prefetchEmojiList sends the emoji.list ahead.
func (f *prefetcher) prefetchEmojiList() {
	prefetch(f, emojiListKey, f.client.EmojiList)
}
