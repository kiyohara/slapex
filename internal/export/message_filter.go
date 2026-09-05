// Message filtering: the --exclude-body-emoji / --exclude-reaction-emoji
// decisions and the excluded message / thread bookkeeping Run consults while
// fetching (doc/design/cli-interface.md).
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
	if !f.bodyEmoji.MatchesText(message.Text) && !f.matchesReaction(message.Reactions) {
		return true
	}
	f.Exclude(message)
	return false
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

func (f *messageFilter) ExcludeThread(threadTS string) {
	if threadTS != "" {
		f.excludedThread[threadTS] = struct{}{}
	}
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
