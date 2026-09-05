package export

import (
	"testing"

	"github.com/kiyohara/slapex/internal/render"
)

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
