// Message collection helpers: which threads still need conversations.replies,
// which user / bot IDs need resolving, and the oldest timestamp of a history
// batch (doc/design/slack-api-usage.md).

package export

import (
	"regexp"
	"sort"

	"github.com/kiyohara/slapex/internal/slack"
)

// fetchedThreads is the set of threads whose replies Run has fetched through
// conversations.replies, keyed by thread_ts. It only records the fetch: whether
// a thread is excluded is the messageFilter's to say.
type fetchedThreads map[string]struct{}

// unfetchedThreadIDs returns the threads of messages whose replies still need
// fetching, in message order: those of thread parents and, with
// inspectBroadcasts, those of thread_broadcast copies too, so the emoji filters
// can judge the thread of a broadcast whose parent is not among the fetched
// messages. A thread in fetched is skipped, so a thread whose parent or
// broadcast shows up again on a later history page is fetched only once.
func unfetchedThreadIDs(messages []slack.Message, fetched fetchedThreads, inspectBroadcasts bool) []string {
	seen := map[string]bool{}
	var ids []string
	for i := range messages {
		threadTS := messageThreadTS(&messages[i])
		if threadTS == "" || seen[threadTS] {
			continue
		}
		if _, ok := fetched[threadTS]; ok {
			continue
		}
		if !messages[i].IsThreadParent() && (!inspectBroadcasts || messages[i].Subtype != "thread_broadcast") {
			continue
		}
		seen[threadTS] = true
		ids = append(ids, threadTS)
	}
	return ids
}

// countReplies returns the number of replies kept across all threads.
func countReplies(replies map[string][]slack.Message) int {
	n := 0
	for _, rs := range replies {
		n += len(rs)
	}
	return n
}

func messageThreadTS(message *slack.Message) string {
	if message == nil {
		return ""
	}
	if message.ThreadTS != "" {
		return message.ThreadTS
	}
	if message.IsThreadParent() {
		return message.TS
	}
	return ""
}

func oldestMessageTS(messages []slack.Message) string {
	oldest := ""
	for i := range messages {
		if oldest == "" || tsLess(messages[i].TS, oldest) {
			oldest = messages[i].TS
		}
	}
	return oldest
}

var reMention = regexp.MustCompile(`<@([UW][A-Z0-9]+)[|>]`)

// collectUserIDs returns the unique user IDs to resolve through users.info:
// posters, channel_join inviters and the mentions in every text the view
// builder passes to render.Mrkdwn. Mrkdwn resolves a label-less mention through
// messageViewBuilder.UserName, which shows the raw user ID for a user not
// collected here, so the scanned texts must match those Mrkdwn call sites. They
// depend on how messageView shows the message, so the scan switches on the same
// messageKindOf: a message shown in full renders its body (messageView) and
// each legacy attachment's text (addUnfurls), and a system row only its body
// (systemBody). Texts that are not rendered, such as the attachments of a
// pinned_item row or a tombstone's body, are not scanned, nor are fields
// rendered as plain text, such as the attachment title.
func collectUserIDs(messages []slack.Message, replies map[string][]slack.Message) []string {
	seen := map[string]bool{}
	addMentions := func(text string) {
		for _, match := range reMention.FindAllStringSubmatch(text, -1) {
			seen[match[1]] = true
		}
	}
	add := func(m *slack.Message) {
		if m.User != "" {
			seen[m.User] = true
		}
		if m.Inviter != "" {
			seen[m.Inviter] = true
		}
		switch messageKindOf(m) {
		case messageFull:
			addMentions(m.Text)
			for i := range m.Attachments {
				addMentions(m.Attachments[i].Text)
			}
		case messageSystem:
			addMentions(m.Text)
		}
	}
	for i := range messages {
		add(&messages[i])
	}
	for _, rs := range replies {
		for i := range rs {
			add(&rs[i])
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// collectBotIDs returns the unique bot IDs that still need a bots.info call:
// bot messages that carry only bot_id, and those whose inline bot_profile is
// missing either the name or the icon. A bot_profile carrying both already
// answers everything slapex renders, so it costs no call (decision log 0054).
func collectBotIDs(messages []slack.Message, replies map[string][]slack.Message) []string {
	seen := map[string]bool{}
	add := func(m *slack.Message) {
		if m.User != "" || m.BotID == "" {
			return
		}
		if m.BotProfile != nil && m.BotProfile.Name != "" && m.BotProfile.Icons.URL() != "" {
			return
		}
		seen[m.BotID] = true
	}
	for i := range messages {
		add(&messages[i])
	}
	for _, rs := range replies {
		for i := range rs {
			add(&rs[i])
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
