package export

import (
	"slices"
	"testing"

	"github.com/kiyohara/slapex/internal/render"
	"github.com/kiyohara/slapex/internal/slack"
)

// userNameRecorder is a render.TextResolver that records the user IDs
// render.Mrkdwn asks it to resolve.
type userNameRecorder struct{ ids []string }

func (r *userNameRecorder) UserName(id string) string {
	if !slices.Contains(r.ids, id) {
		r.ids = append(r.ids, id)
	}
	return id
}

func (r *userNameRecorder) EmojiHTML(name string) string { return ":" + name + ":" }

// TestCollectUserIDsMatchesMrkdwn pins Issue #209: collectUserIDs collects the
// mentions in a text exactly when render.Mrkdwn resolves them through UserName.
// Mrkdwn shows a mention with a label as the label, even an empty one, so only
// a label-less mention needs its user looked up; code content resolves the
// same way. Each case checks both sides, so a change to either shows here.
func TestCollectUserIDsMatchesMrkdwn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "label-less", text: "hi <@U01> and <@W02>", want: []string{"U01", "W02"}},
		{name: "labeled", text: "hi <@U01|alice>"},
		{name: "empty label", text: "hi <@U01|>"},
		{name: "labeled and label-less", text: "<@U01|alice> and <@U01> and <@U02|bob>", want: []string{"U01"}},
		{name: "inline code", text: "`<@U01> <@U02|bob>`", want: []string{"U01"}},
		{name: "code block", text: "```\n<@U01|alice>\n<@U02>\n```", want: []string{"U02"}},
		{name: "not a mention", text: "<@u01> <@U02 x> &lt;@U03&gt;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &userNameRecorder{}
			render.Mrkdwn(tt.text, rec)
			slices.Sort(rec.ids)
			if !slices.Equal(rec.ids, tt.want) {
				t.Fatalf("Mrkdwn(%q) resolved %v, want %v", tt.text, rec.ids, tt.want)
			}
			messages := []slack.Message{{Type: "message", TS: "1700000001.000000", Text: tt.text}}
			if got := collectUserIDs(messages, nil); !slices.Equal(got, tt.want) {
				t.Fatalf("collectUserIDs(%q) = %v, want %v, the users Mrkdwn resolves", tt.text, got, tt.want)
			}
		})
	}
}
