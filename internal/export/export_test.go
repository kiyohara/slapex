package export

import (
	"testing"
)

func TestExcludedMessagesLabel(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{name: "none"},
		{name: "body", opts: Options{ExcludeBodyEmoji: []string{"shushing_face"}}, want: "excluded by body emoji"},
		{name: "reaction", opts: Options{ExcludeReactionEmoji: []string{"speak_no_evil"}}, want: "excluded by reaction emoji"},
		{name: "both", opts: Options{ExcludeBodyEmoji: []string{"shushing_face"}, ExcludeReactionEmoji: []string{"speak_no_evil"}}, want: "excluded by emoji filters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := excludedMessagesLabel(tt.opts); got != tt.want {
				t.Fatalf("excludedMessagesLabel = %q, want %q", got, tt.want)
			}
		})
	}
}
