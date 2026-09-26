// Message filtering: the --exclude-body-emoji / --exclude-reaction-emoji
// decisions and the excluded message / thread bookkeeping the Messages stage
// consults while fetching (doc/design/cli-interface.md).

package export

import (
	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/slack"
)

type messageFilter struct {
	bodyEmoji      emoji.NameSet
	reactionEmoji  emoji.NameSet
	excluded       map[string]struct{}
	excludedThread map[string]struct{}
}

func newMessageFilter(bodyEmoji, reactionEmoji []string) *messageFilter {
	return &messageFilter{
		bodyEmoji:      emoji.NewNameSet(bodyEmoji),
		reactionEmoji:  emoji.NewNameSet(reactionEmoji),
		excluded:       map[string]struct{}{},
		excludedThread: map[string]struct{}{},
	}
}

func (f *messageFilter) Include(message *slack.Message) bool {
	if message == nil {
		return false
	}
	if !f.matches(message) {
		return true
	}
	f.Exclude(message)
	return false
}

// matches reports whether message matches either emoji filter. Unlike
// Include, it does not count the message as excluded.
func (f *messageFilter) matches(message *slack.Message) bool {
	return f.bodyEmoji.MatchesText(message.Text) || f.matchesReaction(message.Reactions)
}

func (f *messageFilter) matchesReaction(reactions []slack.Reaction) bool {
	for _, reaction := range reactions {
		if f.reactionEmoji.MatchesName(reaction.Name) {
			return true
		}
	}
	return false
}

func (f *messageFilter) Exclude(message *slack.Message) {
	if message == nil {
		return
	}
	f.excluded[message.TS] = struct{}{}
	if message.IsThreadParent() {
		f.ExcludeThread(message.TS)
	}
}

// ExcludeReply counts the thread reply at ts as excluded. The Messages stage
// judges a thread's replies with matches as it fetches them, keeps only the ts
// of the excluded ones, and counts those here once the thread's parent is on
// the timeline (timelineReplies). A reply is no thread parent, so unlike
// Exclude this never excludes a thread.
func (f *messageFilter) ExcludeReply(ts string) {
	f.excluded[ts] = struct{}{}
}

func (f *messageFilter) ExcludeThread(threadTS string) {
	if threadTS != "" {
		f.excludedThread[threadTS] = struct{}{}
	}
}

// IncludeThread reports whether the thread rooted at threadTS stays in the
// export once conversations.replies has returned its parent (nil when the
// response had none). The thread is dropped when it is already excluded,
// because an excluded copy of its parent was seen on the timeline, or when this
// copy of the parent matches the filters; the latter marks the thread
// excluded, so its messages on the timeline go too. The parent is not counted
// as excluded here: this copy may be all slapex sees of the parent of a
// broadcast, when that parent is older than the fetch range or beyond
// --max-posts and so never a timeline candidate (Issue #206). A parent on the
// timeline is counted there, by conversations.history or when
// dropExcludedThreads drops it with its thread.
func (f *messageFilter) IncludeThread(threadTS string, parent *slack.Message) bool {
	if parent != nil && f.matches(parent) {
		f.ExcludeThread(threadTS)
	}
	return !f.ThreadExcluded(threadTS)
}

func (f *messageFilter) ThreadExcluded(threadTS string) bool {
	_, ok := f.excludedThread[threadTS]
	return ok
}

func (f *messageFilter) Enabled() bool {
	return len(f.bodyEmoji) > 0 || len(f.reactionEmoji) > 0
}

func (f *messageFilter) ExcludedCount() int {
	return len(f.excluded)
}
