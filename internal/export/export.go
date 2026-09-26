// Package export orchestrates one slapex run: workspace resolution, channel
// selection, fetching, asset downloads, HTML rendering and .cache/ output
// (doc/design/usage-flow.md and the spec documents it references).
package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// UsageError maps to exit code 2: the target could not be determined from
// the given arguments (doc/design/cli-interface.md).
type UsageError struct{ msg string }

func (e *UsageError) Error() string { return e.msg }

func usagef(format string, args ...any) *UsageError {
	return &UsageError{msg: fmt.Sprintf(format, args...)}
}

// Options are the validated CLI inputs (doc/design/cli-interface.md).
type Options struct {
	ChannelKeyword       string
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
	NoInteractive        bool
	PromptTTY            *os.File // controlling terminal for interactive prompts; nil when unavailable
	ToolVersion          string
	// Now overrides the export clock used for the footer "Exported" line, the
	// --days range boundaries, the default output-root name and the .cache/
	// generated_at / executed_at timestamps. Zero means time.Now(). gensample
	// sets it from its -time flag when a sample regeneration is pinned for
	// reproducibility; normal runs and slapex --demo leave it zero. It does not
	// affect the Done line's elapsed time, which Run measures on the real clock
	// from its own start (Issue #207).
	Now time.Time
}

// Run performs the export and returns the absolute path of the directory
// holding index.html. Progress and diagnostics go through p; each stage is a
// ui phase line (doc/design/usage-flow.md「処理対象の表示」). The stages run in
// this order, and the first error ends the run:
//
//   - resolveTarget (Workspace, Channel phases): the token's workspace and the
//     channel to export;
//   - resolveReuseCache, createOutputDir, resolveFetchRange: the --reuse-cache
//     cache, the channel's output directory and the [start, end) range;
//   - fetchMessages (Messages): the timeline and thread replies the emoji
//     filters and --max-posts leave;
//   - resolveUsers (Users) and resolveCustomEmoji (Emoji): the users, bots and
//     custom emoji the messages show;
//   - the Assets phase: the workspace icon and avatars, the timeline view with
//     the assets it shows, and index.html;
//   - writeCaches and the .cache/ cleanup, then reportDone (Done).
func Run(ctx context.Context, client *slack.Client, opts Options, p *ui.Printer) (string, error) {
	// start is the real clock for the Done elapsed time; now is the export
	// clock, which opts.Now may pin to another instant (see Options.Now).
	start := time.Now()
	now := opts.Now
	if now.IsZero() {
		now = start
	}

	target, err := resolveTarget(ctx, client, opts, p)
	if err != nil {
		return "", err
	}
	var reuse *reusableCache
	if opts.ReuseCache != "" {
		reuse = resolveReuseCache(opts.ReuseCache, target.auth.TeamID, target.channel.ID, p)
	}
	out, err := createOutputDir(opts.OutputDir, now, target)
	if err != nil {
		return "", err
	}
	fetchRange, err := resolveFetchRange(opts, now)
	if err != nil {
		return "", err
	}

	fetched, err := fetchMessages(ctx, client, target.channel.ID, fetchRange, opts, p)
	if err != nil {
		return "", err
	}
	resolved := resolveUsers(ctx, client, fetched, reuse, p)
	customEmoji, err := resolveCustomEmoji(ctx, client, reuse, p)
	if err != nil {
		return "", err
	}
	emojiResolver, err := emoji.NewResolver(customEmoji)
	if err != nil {
		return "", fmt.Errorf("load embedded emoji table: %w", err)
	}

	assets := output.NewAssets(ctx, client, out.path, opts.MaxAttachBytes)
	assets.Logf = p.Warnf
	if reuse != nil {
		assets.SetReuseSource(reuse.reuseSource())
	}
	p.StartPhase("Assets", "downloading assets and rendering HTML ...")
	workspaceIcon := saveWorkspaceIcon(assets, target.teamInfo)
	views := newMessageViewBuilder(assets, resolved, emojiResolver, opts.MaxAttachBytes)
	items, counts := buildTimeline(views, fetched)
	page := buildPage(target, workspaceIcon, items, fetched.truncated, fetchRange, opts, now)
	if err := writePage(out.path, page); err != nil {
		return "", err
	}
	assetTotals := endAssetsPhase(p, assets)

	if err := writeCaches(out.path, now, target.auth, target.channel, opts, fetchRange, out.wsLabel, out.chLabel,
		counts.timeline, counts.threads, counts.replies, counts.excluded,
		assetTotals.saved, assetTotals.skipped, assetTotals.failed,
		resolved.users, resolved.bots, customEmoji, assets); err != nil {
		return "", err
	}
	if !opts.KeepCache {
		if err := output.RemoveCache(out.path); err != nil {
			return "", err
		}
	}
	return reportDone(p, target, out.path, start, counts, assetTotals, excludedMessagesLabel(opts)), nil
}

// exportTarget is the result of the Workspace and Channel stages: the token's
// workspace, the channel to export, and the lines that describe them in the
// phase lines, the footer and the Done summary.
type exportTarget struct {
	auth     *slack.AuthTest
	teamInfo *slack.TeamInfo // as team.info returned it; only the header icon uses it
	channel  slack.Channel
	wsLine   string
	chLine   string
}

// resolveTarget runs the Workspace and Channel phases: auth.test checks the
// token, team.info supplies the workspace icon (a failure only costs the icon),
// and the channel comes from the channel list by keyword or selection.
func resolveTarget(ctx context.Context, client *slack.Client, opts Options, p *ui.Printer) (exportTarget, error) {
	p.StartPhase("Workspace", "checking token (auth.test) ...")
	auth, err := client.AuthTest(ctx)
	if err != nil {
		return exportTarget{}, err
	}
	teamInfo, err := client.TeamInfo(ctx)
	if err != nil {
		p.Warnf("workspace icon unavailable: %s", err)
	}
	wsLine := fmt.Sprintf("%s (%s, %s)", auth.Team, hostOf(auth.URL), auth.TeamID)
	p.EndPhase(ui.StatusSuccess, "Workspace", auth.Team, hostOf(auth.URL)+", "+auth.TeamID)

	p.StartPhase("Channel", "listing channels ...")
	channels, err := client.ListChannels(ctx)
	if err != nil {
		return exportTarget{}, err
	}
	ch, err := chooseChannel(channels, opts, wsLine, p)
	if err != nil {
		return exportTarget{}, err
	}
	p.EndPhase(ui.StatusSuccess, "Channel", "#"+ch.Name, channelMeta(ch))
	return exportTarget{auth: auth, teamInfo: teamInfo, channel: ch, wsLine: wsLine, chLine: channelLine(ch)}, nil
}

// outputDir is the channel directory one run writes into, with the workspace
// and channel labels that name it (doc/design/output-format.md).
type outputDir struct {
	path    string
	wsLabel string
	chLabel string
}

// createOutputDir creates the channel directory under the output root, which
// output.Root names from outputRoot and now.
func createOutputDir(outputRoot string, now time.Time, target exportTarget) (outputDir, error) {
	root, err := output.Root(outputRoot, now)
	if err != nil {
		return outputDir{}, fmt.Errorf("create output root: %w", err)
	}
	wsLabel := output.WorkspaceLabel(target.auth.URL, target.auth.Team, target.auth.TeamID)
	chLabel := output.ChannelLabel(target.channel.Name, target.channel.ID)
	path := filepath.Join(root, wsLabel, chLabel)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return outputDir{}, fmt.Errorf("create output directory: %w", err)
	}
	return outputDir{path: path, wsLabel: wsLabel, chLabel: chLabel}, nil
}

// resolveCustomEmoji runs the Emoji phase: the workspace's custom emoji from
// emoji.list, or from the reuse cache without the call.
func resolveCustomEmoji(ctx context.Context, client *slack.Client, reuse *reusableCache, p *ui.Printer) (map[string]string, error) {
	if reuse != nil {
		p.EndPhase(ui.StatusSuccess, "Emoji", fmt.Sprintf("%d custom emoji", len(reuse.emoji)), "from cache, emoji.list skipped")
		return reuse.emoji, nil
	}
	p.StartPhase("Emoji", "fetching custom emoji list ...")
	customEmoji, err := client.EmojiList(ctx)
	if err != nil {
		return nil, err
	}
	p.EndPhase(ui.StatusSuccess, "Emoji", fmt.Sprintf("%d custom emoji", len(customEmoji)), "")
	return customEmoji, nil
}

// exportCounts are the message counts metadata.json and the Done summary
// report. threads and replies count what the page shows: the timeline messages
// shown with replies, and those replies.
type exportCounts struct {
	timeline int
	threads  int
	replies  int
	excluded int
}

// reportDone runs the Done phase: the exported workspace and channel with the
// run's elapsed time on the real clock since start, then the summary of the
// message counts, the asset totals the Assets line reported and the output
// directory. It returns dir as an absolute path, or dir itself when that
// fails. excludedLabel names the emoji filters in use and is "" without one.
func reportDone(p *ui.Printer, target exportTarget, dir string, start time.Time, counts exportCounts, assetTotals assetCounts, excludedLabel string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	p.EndPhase(ui.StatusSuccess, "Done", fmt.Sprintf("%s / %s", target.wsLine, target.chLine),
		fmt.Sprintf("in %s", time.Since(start).Round(time.Second)))
	p.Plainf("  messages: %d (threads: %d, replies: %d)", counts.timeline, counts.threads, counts.replies)
	if excludedLabel != "" {
		p.Plainf("    %s: %d", excludedLabel, counts.excluded)
	}
	p.Plainf("  assets: %d saved, %d skipped by size limit, %d failed", assetTotals.saved, assetTotals.skipped, assetTotals.failed)
	if assetTotals.reused > 0 {
		p.Plainf("    (of which %d reused from cache, no download)", assetTotals.reused)
	}
	p.Plainf("  output: %s", abs)
	return abs
}

func excludedMessagesLabel(opts Options) string {
	switch {
	case len(opts.ExcludeBodyEmoji) > 0 && len(opts.ExcludeReactionEmoji) > 0:
		return "excluded by emoji filters"
	case len(opts.ExcludeBodyEmoji) > 0:
		return "excluded by body emoji"
	case len(opts.ExcludeReactionEmoji) > 0:
		return "excluded by reaction emoji"
	default:
		return ""
	}
}

// avatarURL is the avatar image URL slapex saves for a user: the 72px image,
// falling back to the 48px image. Persisting this effective URL (rather than
// image_72 alone) lets --reuse-cache reproduce the same avatar source_url, so a
// user whose avatar came from image_48 is not dropped on reuse.
func avatarURL(u *slack.User) string {
	if u.Profile.Image72 != "" {
		return u.Profile.Image72
	}
	return u.Profile.Image48
}

// --- small helpers -----------------------------------------------------------

func tsTime(ts string) time.Time {
	f, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return time.Time{}
	}
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}

func tsLess(a, b string) bool {
	fa, _ := strconv.ParseFloat(a, 64)
	fb, _ := strconv.ParseFloat(b, 64)
	return fa < fb
}

func hostOf(rawURL string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
	host = strings.TrimSuffix(host, "/")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	return host
}

func offsetString(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d:%02d", sign, seconds/3600, (seconds%3600)/60)
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30 && n%(1<<30) == 0:
		return fmt.Sprintf("%dGB", n>>30)
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%dMB", n>>20)
	case n >= 1<<10 && n%(1<<10) == 0:
		return fmt.Sprintf("%dKB", n>>10)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
