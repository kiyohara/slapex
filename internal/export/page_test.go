package export

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// TestNewMessageViewBuilderSavesAvatarsInIDOrder checks that the avatars are
// asked for in a fixed order, the users' in user ID order and then the
// bots.info app icons in bot ID order, whatever order the maps holding them
// iterate in, so the plan, the manifest and the warnings keep one order on
// every run (Issue #274). A planner records the requests.
func TestNewMessageViewBuilderSavesAvatarsInIDOrder(t *testing.T) {
	t.Parallel()

	resolved := resolvedUsers{users: map[string]*slack.User{}, bots: map[string]*slack.Bot{}}
	var want []string
	for i := 1; i <= 20; i++ {
		u := testUser(fmt.Sprintf("U%02d", i), "user", "User", "User", fmt.Sprintf("https://example.com/avatars/U%02d.png", i))
		resolved.users[u.ID] = &u
		want = append(want, u.Profile.Image72)
	}
	for i := 1; i <= 5; i++ {
		b := slack.Bot{ID: fmt.Sprintf("B%02d", i), Icons: botIcons(fmt.Sprintf("https://example.com/apps/B%02d.png", i))}
		resolved.bots[b.ID] = &b
		want = append(want, b.Icons.URL())
	}

	planner := output.NewAssets(context.Background(), nil, "", 0).Planner()
	newMessageViewBuilder(planner, resolved, nil, 0)

	var got []string
	for _, p := range planner.Plan() {
		got = append(got, p.SourceURL)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("avatar requests = %q\nwant %q", got, want)
	}
}
