package export

import (
	"reflect"
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
// Mrkdwn never resolves a mention with a label, even an empty one: it shows the
// label, or the user ID when the label is empty. So only a label-less mention
// needs its user looked up; code content resolves the same way. Each case
// checks both sides, so a change to either shows here.
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

// TestCollectUserIDsMatchesMessageView pins Issue #251: collectUserIDs
// collects a poster or a channel_join inviter exactly when messageView uses
// that user's users.info result. A message shown in full shows its poster's
// name and avatar. A system row shows no avatar and names a user only in the
// actor prefix of channel_topic / channel_purpose / channel_name or in the
// invited-by suffix of channel_join, and a tombstone or a bodiless unknown
// subtype shows no user at all. Each case checks both sides: messageView
// renders the message alike with only want resolved and with every user
// resolved, and collectUserIDs returns want, so a change to either shows here.
// A system row text starting with "@" and the poster's display name renders
// alike without the poster too, but only the name tells that it does, so the
// poster stays in want. Mentions follow TestCollectUserIDsMatchesMrkdwn.
func TestCollectUserIDsMatchesMessageView(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message slack.Message
		want    []string
	}{
		{name: "shown in full", message: slack.Message{User: "U01", Text: "hello"}, want: []string{"U01"}},
		{name: "unknown subtype with a body", message: slack.Message{Subtype: "huddle_thread", User: "U01", Text: "summary"}, want: []string{"U01"}},
		{name: "inviter of a message shown in full", message: slack.Message{Subtype: "group_join", User: "U01", Inviter: "U02", Text: "joined"}, want: []string{"U01"}},
		{name: "tombstone", message: slack.Message{Subtype: "tombstone", User: "U01", Inviter: "U02", Text: "This message was deleted."}},
		{name: "bodiless unknown subtype", message: slack.Message{Subtype: "some_unknown_event", User: "U01", Inviter: "U02"}},
		{name: "channel_join", message: slack.Message{Subtype: "channel_join", User: "U01", Text: "has joined the channel"}},
		{name: "channel_join mentioning the joiner", message: slack.Message{Subtype: "channel_join", User: "U01", Text: "<@U01> has joined the channel"}, want: []string{"U01"}},
		{name: "channel_join with an inviter", message: slack.Message{Subtype: "channel_join", User: "U01", Inviter: "U02", Text: "<@U01|alice> has joined the channel"}, want: []string{"U02"}},
		{name: "channel_join invited by the joiner", message: slack.Message{Subtype: "channel_join", User: "U01", Inviter: "U01", Text: "has joined the channel"}},
		{name: "channel_join mentioning the inviter", message: slack.Message{Subtype: "channel_join", User: "U01", Inviter: "U02", Text: "has joined the channel, added by <@U02>"}, want: []string{"U02"}},
		{name: "channel_join mentioning the inviter with a label", message: slack.Message{Subtype: "channel_join", User: "U01", Inviter: "U02", Text: "has joined the channel, added by <@U02|bob>"}},
		{name: "channel_leave", message: slack.Message{Subtype: "channel_leave", User: "U01", Text: "has left the channel"}},
		{name: "pinned_item", message: slack.Message{Subtype: "pinned_item", User: "U01", Text: "pinned a message to this channel."}},
		{name: "channel_topic", message: slack.Message{Subtype: "channel_topic", User: "U01", Text: "set the channel topic: Launch"}, want: []string{"U01"}},
		{name: "channel_purpose starting with the poster's mention", message: slack.Message{Subtype: "channel_purpose", User: "U01", Text: "<@U01> set the channel purpose: Docs"}, want: []string{"U01"}},
		{name: "channel_name starting with the poster's labeled mention", message: slack.Message{Subtype: "channel_name", User: "U01", Text: "<@U01|alice> set the channel name: beta"}},
		{name: "channel_topic starting with the poster's display name", message: slack.Message{Subtype: "channel_topic", User: "U02", Text: "@Bob set the channel topic: Launch"}, want: []string{"U02"}},
		{name: "channel_topic without a poster", message: slack.Message{Subtype: "channel_topic", Text: "set the channel topic: Launch"}},
	}
	alice := testUser("U01", "alice", "Alice Example", "Alice", "")
	bob := testUser("U02", "bob", "Bob Builder", "Bob", "")
	users := map[string]*slack.User{"U01": &alice, "U02": &bob}
	// viewWith renders m with only ids resolved, each with a saved avatar, so
	// a view using a user left out renders differently.
	viewWith := func(m *slack.Message, ids []string) *render.MessageView {
		b := &messageViewBuilder{users: map[string]*slack.User{}, avatars: map[string]string{}}
		for _, id := range ids {
			b.users[id] = users[id]
			b.avatars[id] = "assets/avatars/" + id + ".png"
		}
		return b.messageView(m)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := tt.message
			m.Type, m.TS = "message", "1700000001.000000"
			if got, all := viewWith(&m, tt.want), viewWith(&m, []string{"U01", "U02"}); !reflect.DeepEqual(got, all) {
				t.Fatalf("messageView with only %v resolved = %+v, want %+v as with every user resolved", tt.want, got, all)
			}
			if got := collectUserIDs([]slack.Message{m}, nil); !slices.Equal(got, tt.want) {
				t.Fatalf("collectUserIDs() = %v, want %v, the users messageView uses", got, tt.want)
			}
		})
	}
}
