package demo

import (
	"context"
	"time"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// Options are the caller-specific knobs for Export. The fixture-facing
// invariants (in-process fake server, fake token + base URL, single-channel
// non-interactive resolution, skipped API pacing) are fixed by Export itself,
// so callers only supply the output location and fetch window.
type Options struct {
	OutputDir        string
	MaxPosts         int
	Days             int
	Date             string
	From             string
	To               string
	ExcludeBodyEmoji []string
	MaxAttachBytes   int64
	KeepCache        bool
	ReuseCache       string
	ToolVersion      string
	// Now overrides the export clock (footer timestamp, fetch window). Zero means
	// time.Now(). gensample sets it (from its -time flag) when a sample
	// regeneration is pinned for reproducibility; slapex --demo leaves it zero so
	// a user's demo shows current dates.
	Now time.Time
}

// Export runs the real export pipeline against sc, served by an in-process fake
// Slack server, and returns the output directory. It is the single shared
// driver behind both `slapex --demo` (Issue #113) and gensample's sample
// regeneration (Issue #51): the wiring that must stay identical between them
// — fake token, base URL, no rate-limit pacing, and resolving the fixture's one
// channel non-interactively — lives here and nowhere else.
func Export(ctx context.Context, sc *Scenario, o Options, printer *ui.Printer) (string, error) {
	srv := NewServer(sc)
	defer srv.Close()

	// The fixture is served in-process with no real rate limits, so skip the
	// Slack API pacing a real run applies; the demo/sample run stays snappy.
	client := slack.New(FakeToken,
		slack.WithBaseURL(srv.APIBaseURL()),
		slack.WithSleeper(NoPacing),
	)
	client.Logf = printer.Noticef

	return export.Run(ctx, client, export.Options{
		ChannelKeyword:   sc.ChannelName,
		OutputDir:        o.OutputDir,
		MaxPosts:         o.MaxPosts,
		Days:             o.Days,
		Date:             o.Date,
		From:             o.From,
		To:               o.To,
		ExcludeBodyEmoji: o.ExcludeBodyEmoji,
		MaxAttachBytes:   o.MaxAttachBytes,
		KeepCache:        o.KeepCache,
		ReuseCache:       o.ReuseCache,
		NoInteractive:    true,
		ToolVersion:      o.ToolVersion,
		Now:              o.Now,
	}, printer)
}
