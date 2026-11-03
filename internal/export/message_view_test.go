package export

import (
	"testing"

	"github.com/kiyohara/slapex/internal/render"
)

func TestSplitSentUsingFooter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		text       string
		wantBody   string
		wantFooter string
	}{
		{
			// The shape seen in a real export (Issue #303): the Japanese
			// footer follows the body on the same line after a space.
			name:       "japanese on the same line",
			text:       "まとめを作りました。 *使用して送信されました* Cursor",
			wantBody:   "まとめを作りました。",
			wantFooter: "*使用して送信されました* Cursor",
		},
		{
			name:       "english on its own line",
			text:       "status update\n*Sent using* Cursor",
			wantBody:   "status update",
			wantFooter: "*Sent using* Cursor",
		},
		{
			name:       "app as a mention",
			text:       "done *Sent using* <@U02>",
			wantBody:   "done",
			wantFooter: "*Sent using* <@U02>",
		},
		{
			name:       "app name with spaces and trailing newline",
			text:       "reply *Sent using* Dimension (as you)\n",
			wantBody:   "reply",
			wantFooter: "*Sent using* Dimension (as you)",
		},
		{
			name:       "multi-line body keeps its lines",
			text:       "line 1\n\nline 2 *使用して送信されました* Cursor",
			wantBody:   "line 1\n\nline 2",
			wantFooter: "*使用して送信されました* Cursor",
		},
		{
			name:       "footer only",
			text:       "*Sent using* Cursor",
			wantBody:   "",
			wantFooter: "*Sent using* Cursor",
		},
		{
			// Slack appends one footer, so the last label starts it.
			name:       "several labels on the last line",
			text:       "a *Sent using* b *Sent using* Cursor",
			wantBody:   "a *Sent using* b",
			wantFooter: "*Sent using* Cursor",
		},
		{
			name:       "label inside inline code before the footer",
			text:       "see `x *Sent using* y` *使用して送信されました* Cursor",
			wantBody:   "see `x *Sent using* y`",
			wantFooter: "*使用して送信されました* Cursor",
		},
		{
			// Splitting here would break the code span.
			name:     "label inside inline code ending the text",
			text:     "see `x *Sent using* y`",
			wantBody: "see `x *Sent using* y`",
		},
		{
			name:     "label inside a one-line code block",
			text:     "```x *Sent using* Cursor```",
			wantBody: "```x *Sent using* Cursor```",
		},
		{
			name:     "no footer",
			text:     "plain *bold* message",
			wantBody: "plain *bold* message",
		},
		{
			name:     "footer not at the end",
			text:     "*Sent using* Cursor\nmore text",
			wantBody: "*Sent using* Cursor\nmore text",
		},
		{
			name:     "label glued to the previous word",
			text:     "foo*Sent using* Cursor",
			wantBody: "foo*Sent using* Cursor",
		},
		{
			name:     "label without an app",
			text:     "body *Sent using*",
			wantBody: "body *Sent using*",
		},
		{
			name:     "app on the next line",
			text:     "body *Sent using*\nCursor",
			wantBody: "body *Sent using*\nCursor",
		},
		{
			name:     "unknown label",
			text:     "corps *Envoyé avec* Cursor",
			wantBody: "corps *Envoyé avec* Cursor",
		},
		{
			name:     "label not bold",
			text:     "body Sent using Cursor",
			wantBody: "body Sent using Cursor",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, footer := splitSentUsingFooter(tc.text)
			if body != tc.wantBody || footer != tc.wantFooter {
				t.Fatalf("splitSentUsingFooter(%q) = (%q, %q), want (%q, %q)",
					tc.text, body, footer, tc.wantBody, tc.wantFooter)
			}
		})
	}
}

func TestThreadParticipants(t *testing.T) {
	t.Parallel()

	replies := []*render.MessageView{
		{Author: "Alice", AvatarPath: "avatars/alice.png", AvatarInitial: "A"},
		{Author: "Bob", AvatarPath: "avatars/bob.png", AvatarInitial: "B"},
		{Author: "Alice", AvatarPath: "avatars/alice.png", AvatarInitial: "A"},
		{Author: "Carol", AvatarPath: "avatars/carol.png", AvatarInitial: "C"},
		{Author: "Dave", AvatarPath: "avatars/dave.png", AvatarInitial: "D"},
		{IsSystem: true, Author: "system", AvatarInitial: "?"},
	}

	got, extra := threadParticipants(replies)
	if extra != 1 {
		t.Fatalf("extra = %d, want 1", extra)
	}
	if len(got) != 3 {
		t.Fatalf("participants len = %d, want 3", len(got))
	}
	for i, want := range []string{"Alice", "Bob", "Carol"} {
		if got[i].Author != want {
			t.Fatalf("participants[%d].Author = %q, want %q", i, got[i].Author, want)
		}
	}
}
