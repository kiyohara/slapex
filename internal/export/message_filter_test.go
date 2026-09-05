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
