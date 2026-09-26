package export

import (
	"testing"

	"github.com/kiyohara/slapex/internal/slack"
)

func TestMessageFilterCountsExcludedMessageOnce(t *testing.T) {
	filter := newMessageFilter([]string{"shushing_face"}, []string{"speak_no_evil"})
	message := slack.Message{TS: "1700000001.000000", Text: "private :shushing_face:"}
	if filter.Include(&message) || filter.Include(&message) {
		t.Fatal("Include returned true for excluded message")
	}
	if got := filter.ExcludedCount(); got != 1 {
		t.Fatalf("ExcludedCount = %d, want 1", got)
	}
}

func TestMessageFilterMatchesNormalizedReactionName(t *testing.T) {
	filter := newMessageFilter(nil, []string{"+1", "do_not_archive"})
	for _, reaction := range []string{"+1::skin-tone-3", "DO_NOT_ARCHIVE"} {
		message := slack.Message{
			TS:        reaction,
			Reactions: []slack.Reaction{{Name: reaction, Count: 1}},
		}
		if filter.Include(&message) {
			t.Fatalf("Include returned true for reaction %q", reaction)
		}
	}
	if got := filter.ExcludedCount(); got != 2 {
		t.Fatalf("ExcludedCount = %d, want 2", got)
	}
}

// TestMessageFilterIncludeThread covers the thread decision taken once
// conversations.replies has returned a thread's parent: the thread stays only
// when neither that parent nor an earlier timeline copy of it is excluded, and
// the parent is counted once however often it is examined. A parent copy
// without reply_count is not recognised as a thread parent by Exclude, so
// IncludeThread has to mark its thread itself.
func TestMessageFilterIncludeThread(t *testing.T) {
	const threadTS = "1700000001.000000"
	parent := func(text string, replyCount int) *slack.Message {
		return &slack.Message{TS: threadTS, ThreadTS: threadTS, Text: text, ReplyCount: replyCount}
	}
	for _, tc := range []struct {
		name          string
		excludedFirst bool // an excluded timeline copy of the parent came first
		parent        *slack.Message
		want          bool
		wantExcluded  int
	}{
		{name: "kept parent", parent: parent("kept", 1), want: true},
		{name: "no parent", parent: nil, want: true},
		{name: "excluded parent", parent: parent("private :shushing_face:", 1), wantExcluded: 1},
		{name: "excluded parent without reply_count", parent: parent("private :shushing_face:", 0), wantExcluded: 1},
		{name: "excluded before the fetch", excludedFirst: true, parent: parent("private :shushing_face:", 1), wantExcluded: 1},
		{name: "excluded before the fetch, kept copy", excludedFirst: true, parent: parent("kept copy", 1), wantExcluded: 1},
		{name: "excluded before the fetch, no parent", excludedFirst: true, parent: nil, wantExcluded: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter := newMessageFilter([]string{"shushing_face"}, nil)
			if tc.excludedFirst && filter.Include(parent("private :shushing_face:", 1)) {
				t.Fatal("Include returned true for the excluded timeline copy")
			}
			if got := filter.IncludeThread(threadTS, tc.parent); got != tc.want {
				t.Fatalf("IncludeThread = %v, want %v", got, tc.want)
			}
			if got := filter.ThreadExcluded(threadTS); got == tc.want {
				t.Fatalf("ThreadExcluded = %v, want %v", got, !tc.want)
			}
			if got := filter.ExcludedCount(); got != tc.wantExcluded {
				t.Fatalf("ExcludedCount = %d, want %d", got, tc.wantExcluded)
			}
		})
	}
}
