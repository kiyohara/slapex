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
// non-interactive resolution, skipped API pacing) are fixed by Run itself,
// so callers only supply the output location and fetch window.
type Options struct {
	OutputDir            string
	MaxPosts             int
	Days                 int
	Date                 string
	From                 string
	To                   string
	ExcludeBodyEmoji     []string
	ExcludeReactionEmoji []string
	MaxAttachBytes       int64
	KeepCache            bool
	ReuseCache           string
	ToolVersion          string
	// Now overrides the export clock (footer timestamp, fetch window). Zero means
	// time.Now(). gensample sets it (from its -time flag) when a sample
	// regeneration is pinned for reproducibility; slapex --demo leaves it zero so
	// a user's demo shows current dates.
	Now time.Time
}

// Export runs Run with o's options. gensample and the tests set a sample's
// options through it; slapex --demo passes the export options of its command
// line to Run instead.
func Export(ctx context.Context, sc *Scenario, o Options, printer *ui.Printer) (string, error) {
	return Run(ctx, sc, export.Options{
		OutputDir:            o.OutputDir,
		MaxPosts:             o.MaxPosts,
		Days:                 o.Days,
		Date:                 o.Date,
		From:                 o.From,
		To:                   o.To,
		ExcludeBodyEmoji:     o.ExcludeBodyEmoji,
		ExcludeReactionEmoji: o.ExcludeReactionEmoji,
		MaxAttachBytes:       o.MaxAttachBytes,
		KeepCache:            o.KeepCache,
		ReuseCache:           o.ReuseCache,
		ToolVersion:          o.ToolVersion,
		Now:                  o.Now,
	}, printer)
}

// Run runs the real export pipeline with opts against sc, served by an
// in-process fake Slack server, and returns the output directory. It is the
// single shared driver behind both `slapex --demo` (Issue #113), which passes
// the export options of its command line, and gensample's sample regeneration
// (Issue #51, through Export): the wiring that must stay identical between
// them — fake token, base URL, no rate-limit pacing, and resolving the
// fixture's one channel non-interactively — lives here and nowhere else. Of
// opts, Run replaces only what that wiring decides: the channel keyword
// becomes the fixture's channel, NoInteractive is set and PromptTTY cleared.
func Run(ctx context.Context, sc *Scenario, opts export.Options, printer *ui.Printer) (string, error) {
	srv := NewServer(sc)
	defer srv.Close()

	// The fixture is served in-process with no real rate limits, so skip the
	// Slack API pacing a real run applies; the demo/sample run stays snappy.
	client := slack.New(FakeToken,
		slack.WithBaseURL(srv.APIBaseURL()),
		slack.WithSleeper(NoPacing),
	)
	client.Logf = printer.Noticef

	opts.ChannelKeyword = sc.ChannelName
	opts.NoInteractive = true
	opts.PromptTTY = nil
	return export.Run(ctx, client, opts, printer)
}
