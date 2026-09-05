// Package export orchestrates one slapex run: workspace resolution, channel
// selection, fetching, asset downloads, HTML rendering and .cache/ output
// (doc/design/usage-flow.md and the spec documents it references).
package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/render"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

const (
	maxThreadReplies = 1000 // per-thread reply cap (doc/design/output-format.md)
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
	// --days range boundaries and the default output-root name. Zero means
	// time.Now(). gensample sets it from its -time flag when a sample
	// regeneration is pinned for reproducibility; normal runs and slapex --demo
	// leave it zero.
	Now time.Time
}

// Run performs the export and returns the absolute path of the directory
// holding index.html. Progress and diagnostics go through p; each stage is a
// ui phase line (doc/design/usage-flow.md「処理対象の表示」).
func Run(ctx context.Context, client *slack.Client, opts Options, p *ui.Printer) (string, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	p.StartPhase("Workspace", "checking token (auth.test) ...")
	auth, err := client.AuthTest(ctx)
	if err != nil {
		return "", err
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
		return "", err
	}
	ch, err := chooseChannel(channels, opts, wsLine, p)
	if err != nil {
		return "", err
	}
	chLine := channelLine(ch)
	p.EndPhase(ui.StatusSuccess, "Channel", "#"+ch.Name, channelMeta(ch))

	var reuse *reusableCache
	if opts.ReuseCache != "" {
		reuse = resolveReuseCache(opts.ReuseCache, auth.TeamID, ch.ID, p)
	}

	root, err := output.Root(opts.OutputDir, now)
	if err != nil {
		return "", fmt.Errorf("create output root: %w", err)
	}
	wsLabel := output.WorkspaceLabel(auth.URL, auth.Team, auth.TeamID)
	chLabel := output.ChannelLabel(ch.Name, ch.ID)
	dir := filepath.Join(root, wsLabel, chLabel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	fetchRange, err := resolveFetchRange(opts, now)
	if err != nil {
		return "", err
	}
	filter := newMessageFilter(opts.ExcludeBodyEmoji, opts.ExcludeReactionEmoji)
	p.StartPhase("Messages", fmt.Sprintf("fetching %s (--max-posts %d) ...", fetchRange.progressLabel(), opts.MaxPosts))
	var messages []slack.Message
	replies := map[string][]slack.Message{}
	repliesTruncated := map[string]bool{}
	replyTotal := 0
	historyLatest := fetchRange.latestTS()
	truncated := false
	threadFetches := map[string]bool{}
	threadFetchIndex := 0
	for len(messages) < opts.MaxPosts {
		remaining := opts.MaxPosts - len(messages)
		batch, more, err := client.History(ctx, ch.ID, fetchRange.oldestTS(), historyLatest, remaining, filter.Include,
			func(n int) {
				p.UpdatePhase(fmt.Sprintf("fetching %s ... %d fetched", fetchRange.progressLabel(), len(messages)+n))
			})
		if err != nil {
			return "", err
		}
		if len(batch) == 0 {
			truncated = false
			break
		}
		historyLatest = oldestMessageTS(batch)
		messages = append(messages, batch...)

		threadIDs := unfetchedThreadIDs(batch, threadFetches, filter.Enabled())
		threadTotal := len(threadFetches) + len(threadIDs)
		for _, threadTS := range threadIDs {
			threadFetchIndex++
			p.UpdatePhase(fmt.Sprintf("fetching thread replies ... %d/%d", threadFetchIndex, threadTotal))
			parent, r, trunc, err := client.Thread(ctx, ch.ID, threadTS, maxThreadReplies)
			if err != nil {
				return "", err
			}
			threadExcluded := filter.ThreadExcluded(threadTS)
			if parent != nil && !filter.Include(parent) {
				filter.ExcludeThread(threadTS)
				threadExcluded = true
			}
			threadFetches[threadTS] = threadExcluded
			if threadExcluded {
				continue
			}
			var kept []slack.Message
			for i := range r {
				if filter.Include(&r[i]) {
					kept = append(kept, r[i])
				}
			}
			sort.Slice(kept, func(i, j int) bool { return tsLess(kept[i].TS, kept[j].TS) })
			if len(kept) > 0 {
				replies[threadTS] = kept
				repliesTruncated[threadTS] = trunc
			}
			replyTotal += len(kept)
		}

		keptTimeline := messages[:0]
		for i := range messages {
			threadTS := messageThreadTS(&messages[i])
			if filter.ThreadExcluded(threadTS) {
				filter.Exclude(&messages[i])
				if keptReplies, ok := replies[threadTS]; ok {
					replyTotal -= len(keptReplies)
					delete(replies, threadTS)
					delete(repliesTruncated, threadTS)
				}
				threadFetches[threadTS] = true
				continue
			}
			keptTimeline = append(keptTimeline, messages[i])
		}
		messages = keptTimeline

		if len(messages) >= opts.MaxPosts {
			truncated = more
			break
		}
		if !more {
			truncated = false
			break
		}
	}
	sort.Slice(messages, func(i, j int) bool { return tsLess(messages[i].TS, messages[j].TS) })
	excludedTotal := filter.ExcludedCount()
	messagesStatus := ui.StatusSuccess
	messagesMeta := fmt.Sprintf("threads %d, replies %d", len(replies), replyTotal)
	if label := excludedMessagesLabel(opts); excludedTotal > 0 && label != "" {
		messagesMeta += fmt.Sprintf(", %s: %d", label, excludedTotal)
	}
	if truncated {
		messagesStatus = ui.StatusWarn
		messagesMeta += fmt.Sprintf(", truncated by --max-posts %d", opts.MaxPosts)
	}
	p.EndPhase(messagesStatus, "Messages", fmt.Sprintf("%d fetched %s", len(messages), fetchRange.progressLabel()), messagesMeta)

	userIDs := collectUserIDs(messages, replies)
	botIDs := collectBotIDs(messages, replies)
	p.StartPhase("Users", fmt.Sprintf("resolving %s ...", resolveTargetsLabel(len(userIDs), len(botIDs))))
	users := map[string]*slack.User{}
	reusedUsers := 0
	for _, id := range userIDs {
		if reuse != nil {
			if cu, ok := reuse.users[id]; ok {
				users[id] = cu.toUser(id)
				reusedUsers++
				continue
			}
		}
		u, err := client.UserInfo(ctx, id)
		if err != nil {
			p.Warnf("could not resolve user %s: %s", id, err)
			continue
		}
		users[id] = u
	}
	// A bot message that carries only bot_id has no user to resolve; bots.info
	// supplies the app name and icon instead (decision log 0054). A failure is
	// warned about and skipped, like an unresolvable user, so the export still
	// completes with the bot_id and the initial fallback.
	bots := map[string]*slack.Bot{}
	reusedBots := 0
	for _, id := range botIDs {
		if reuse != nil {
			if cb, ok := reuse.bots[id]; ok {
				bots[id] = cb.toBot(id)
				reusedBots++
				continue
			}
		}
		bot, err := client.BotInfo(ctx, id)
		if err != nil {
			p.Warnf("could not resolve bot %s: %s", id, err)
			continue
		}
		bots[id] = bot
	}
	p.EndPhase(ui.StatusSuccess, "Users", resolvedTargetsLabel(len(users), len(bots)),
		reusedTargetsMeta(reusedUsers, reusedBots))

	var customEmoji map[string]string
	if reuse != nil {
		customEmoji = reuse.emoji
		p.EndPhase(ui.StatusSuccess, "Emoji", fmt.Sprintf("%d custom emoji", len(customEmoji)), "from cache, emoji.list skipped")
	} else {
		p.StartPhase("Emoji", "fetching custom emoji list ...")
		customEmoji, err = client.EmojiList(ctx)
		if err != nil {
			return "", err
		}
		p.EndPhase(ui.StatusSuccess, "Emoji", fmt.Sprintf("%d custom emoji", len(customEmoji)), "")
	}
	emojiResolver, err := emoji.NewResolver(customEmoji)
	if err != nil {
		return "", fmt.Errorf("load embedded emoji table: %w", err)
	}

	assets := output.NewAssets(ctx, client, dir, opts.MaxAttachBytes)
	assets.Logf = p.Warnf
	if reuse != nil {
		assets.SetReuseSource(reuse.reuseSource())
	}

	p.StartPhase("Assets", "downloading assets and rendering HTML ...")
	workspaceIconPath := ""
	if rel, ok := assets.Save(output.KindWorkspaceIcon, workspaceIconURL(teamInfo), output.AssetMeta{}); ok {
		workspaceIconPath = rel
	}
	avatars := map[string]string{}
	for id, u := range users {
		if rel, ok := assets.Save(output.KindAvatar, avatarURL(u), output.AssetMeta{}); ok {
			avatars[id] = rel
		}
	}
	// App icons are saved as ordinary avatars (output.KindAvatar), so they land
	// in assets/avatars/ next to the human ones and stay public downloads with
	// no Authorization header (doc/guidelines/credential-scope-guidelines.md).
	botAvatars := map[string]string{}
	for _, id := range botIDs {
		bot, ok := bots[id]
		if !ok {
			continue
		}
		if rel, ok := assets.Save(output.KindAvatar, bot.Icons.URL(), output.AssetMeta{}); ok {
			botAvatars[id] = rel
		}
	}

	viewBuilder := &messageViewBuilder{
		users:              users,
		avatars:            avatars,
		bots:               bots,
		botAvatars:         botAvatars,
		emoji:              emojiResolver,
		assets:             assets,
		maxAttachmentBytes: opts.MaxAttachBytes,
	}
	var items []render.TimelineItem
	lastDate := ""
	threadCount := 0
	replyCount := 0
	for _, m := range messages {
		date := tsTime(m.TS).Format("2006-01-02")
		if date != lastDate {
			items = append(items, render.TimelineItem{IsDateDivider: true, Date: date})
			lastDate = date
		}
		view := viewBuilder.messageView(&m)
		if rs, ok := replies[m.TS]; ok {
			threadCount++
			for i := range rs {
				view.Replies = append(view.Replies, viewBuilder.messageView(&rs[i]))
			}
			view.ThreadParticipants, view.ThreadExtraParticipants = threadParticipants(view.Replies)
			replyCount += len(rs)
			view.RepliesTruncated = repliesTruncated[m.TS]
		}
		items = append(items, render.TimelineItem{Message: view})
	}

	_, tzOffset := now.Zone()
	page := &render.PageData{
		WorkspaceName:     auth.Team,
		WorkspaceIconPath: workspaceIconPath,
		WorkspaceHref:     auth.URL,
		ChannelName:       ch.Name,
		ChannelHref:       channelURL(auth.URL, ch.ID),
		WorkspaceLine:     wsLine,
		ChannelLine:       chLine,
		ExportedLine: fmt.Sprintf("%s (UTC%s) / %s",
			now.Format("2006-01-02 15:04"), offsetString(tzOffset), now.UTC().Format(time.RFC3339)),
		RangeLine:   fetchRange.footerRangeLabel(),
		OptionsLine: fetchRange.footerOptionsLabel(opts),
		ToolLine:    fmt.Sprintf("slapex %s", opts.ToolVersion),
		Items:       items,
		Truncated:   truncated,
	}

	htmlFile, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return "", err
	}
	if err := render.WriteHTML(htmlFile, page); err != nil {
		htmlFile.Close()
		return "", fmt.Errorf("render index.html: %w", err)
	}
	if err := htmlFile.Close(); err != nil {
		return "", err
	}
	if err := render.WriteStyleCSS(dir); err != nil {
		return "", err
	}
	if err := render.WriteStaticAssets(dir); err != nil {
		return "", err
	}

	saved, skipped, failed := assets.Counts()
	assetsStatus := ui.StatusSuccess
	if skipped > 0 || failed > 0 {
		assetsStatus = ui.StatusWarn
	}
	assetsMeta := ""
	if n := assets.Reused(); n > 0 {
		assetsMeta = fmt.Sprintf("%d copied from reused cache, no download", n)
	}
	p.EndPhase(assetsStatus, "Assets",
		fmt.Sprintf("%d saved, %d skipped by size limit, %d failed", saved, skipped, failed), assetsMeta)

	if err := writeCaches(dir, now, auth, ch, opts, fetchRange, wsLabel, chLabel,
		len(messages), threadCount, replyCount, excludedTotal, saved, skipped, failed, users, bots, customEmoji, assets); err != nil {
		return "", err
	}
	if !opts.KeepCache {
		if err := output.RemoveCache(dir); err != nil {
			return "", err
		}
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	p.EndPhase(ui.StatusSuccess, "Done", fmt.Sprintf("%s / %s", wsLine, chLine),
		fmt.Sprintf("in %s", time.Since(now).Round(time.Second)))
	p.Plainf("  messages: %d (threads: %d, replies: %d)", len(messages), threadCount, replyCount)
	if label := excludedMessagesLabel(opts); label != "" {
		p.Plainf("    %s: %d", label, excludedTotal)
	}
	p.Plainf("  assets: %d saved, %d skipped by size limit, %d failed", saved, skipped, failed)
	if n := assets.Reused(); n > 0 {
		p.Plainf("    (of which %d copied from reused cache, no download)", n)
	}
	p.Plainf("  output: %s", abs)
	return abs, nil
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

// resolveTargetsLabel / resolvedTargetsLabel / reusedTargetsMeta render the Users
// phase counts. Bots only appear once the channel actually has bot posts to
// resolve, so a channel without them keeps the original wording.
func resolveTargetsLabel(users, bots int) string {
	if bots == 0 {
		return fmt.Sprintf("%d users", users)
	}
	return fmt.Sprintf("%d users, %s", users, botCountLabel(bots))
}

func resolvedTargetsLabel(users, bots int) string {
	if bots == 0 {
		return fmt.Sprintf("%d resolved", users)
	}
	return fmt.Sprintf("%d users, %s resolved", users, botCountLabel(bots))
}

func reusedTargetsMeta(users, bots int) string {
	switch {
	case users > 0 && bots > 0:
		return fmt.Sprintf("%d from cache, users.info / bots.info skipped", users+bots)
	case users > 0:
		return fmt.Sprintf("%d from cache, users.info skipped", users)
	case bots > 0:
		return fmt.Sprintf("%d from cache, bots.info skipped", bots)
	}
	return ""
}

func botCountLabel(n int) string {
	if n == 1 {
		return "1 bot"
	}
	return fmt.Sprintf("%d bots", n)
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

func workspaceIconURL(teamInfo *slack.TeamInfo) string {
	if teamInfo == nil || teamInfo.Icon.ImageDefault {
		return ""
	}
	for _, u := range []string{
		teamInfo.Icon.Image68,
		teamInfo.Icon.Image88,
		teamInfo.Icon.Image102,
		teamInfo.Icon.Image132,
		teamInfo.Icon.Image230,
		teamInfo.Icon.Image44,
		teamInfo.Icon.Image34,
	} {
		if u != "" {
			return u
		}
	}
	return ""
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
