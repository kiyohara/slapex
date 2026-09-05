// Message collection helpers: which threads still need conversations.replies,
// which user / bot IDs need resolving, and the oldest timestamp of a history
// batch (doc/design/slack-api-usage.md).
package export

import (
	"regexp"
	"sort"

	"github.com/kiyohara/slapex/internal/slack"
)

func newThreadIDs(messages []slack.Message, fetched map[string]bool, inspectBroadcasts bool) []string {
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

func collectUserIDs(messages []slack.Message, replies map[string][]slack.Message) []string {
	seen := map[string]bool{}
	add := func(m *slack.Message) {
		if m.User != "" {
			seen[m.User] = true
		}
		if m.Inviter != "" {
			seen[m.Inviter] = true
		}
		for _, match := range reMention.FindAllStringSubmatch(m.Text, -1) {
			seen[match[1]] = true
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
