// Package export orchestrates one slapex run: workspace resolution, channel
// selection, fetching, asset downloads, HTML rendering and .cache/ output
// (doc/design/usage-flow.md and the spec documents it references).
package export

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"

	"github.com/kiyohara/slapex/internal/datetime"
	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/render"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

const (
	maxSelectable    = 10   // interactive selection limit (decision log 0004)
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

		threadIDs := newThreadIDs(batch, threadFetches, filter.Enabled())
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
	if excludedTotal > 0 {
		messagesMeta += fmt.Sprintf(", excluded by body emoji: %d", excludedTotal)
	}
	if truncated {
		messagesStatus = ui.StatusWarn
		messagesMeta += fmt.Sprintf(", truncated by --max-posts %d", opts.MaxPosts)
	}
	p.EndPhase(messagesStatus, "Messages", fmt.Sprintf("%d fetched %s", len(messages), fetchRange.progressLabel()), messagesMeta)

	userIDs := collectUserIDs(messages, replies)
	p.StartPhase("Users", fmt.Sprintf("resolving %d users ...", len(userIDs)))
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
	usersMeta := ""
	if reusedUsers > 0 {
		usersMeta = fmt.Sprintf("%d from cache, users.info skipped", reusedUsers)
	}
	p.EndPhase(ui.StatusSuccess, "Users", fmt.Sprintf("%d resolved", len(users)), usersMeta)

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

	b := &builder{
		users:   users,
		avatars: avatars,
		emoji:   emojiResolver,
		assets:  assets,
		limit:   opts.MaxAttachBytes,
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
		view := b.messageView(&m)
		if rs, ok := replies[m.TS]; ok {
			threadCount++
			for i := range rs {
				view.Replies = append(view.Replies, b.messageView(&rs[i]))
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
		len(messages), threadCount, replyCount, excludedTotal, saved, skipped, failed, users, customEmoji, assets); err != nil {
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
	switch {
	case len(opts.ExcludeBodyEmoji) > 0 && len(opts.ExcludeReactionEmoji) > 0:
		p.Plainf("    excluded by emoji filters: %d", excludedTotal)
	case len(opts.ExcludeBodyEmoji) > 0:
		p.Plainf("    excluded by body emoji: %d", excludedTotal)
	case len(opts.ExcludeReactionEmoji) > 0:
		p.Plainf("    excluded by reaction emoji: %d", excludedTotal)
	}
	p.Plainf("  assets: %d saved, %d skipped by size limit, %d failed", saved, skipped, failed)
	if n := assets.Reused(); n > 0 {
		p.Plainf("    (of which %d copied from reused cache, no download)", n)
	}
	p.Plainf("  output: %s", abs)
	return abs, nil
}

type messageFetchRange struct {
	mode  string
	start time.Time
	end   time.Time
}

func resolveFetchRange(opts Options, now time.Time) (messageFetchRange, error) {
	if opts.From != "" || opts.To != "" {
		return resolveDateTimeFetchRange(opts.From, opts.To, time.Local)
	}
	if opts.Date == "" {
		return messageFetchRange{
			mode:  "days",
			start: now.Add(-time.Duration(opts.Days) * 24 * time.Hour),
			end:   now,
		}, nil
	}
	return resolveDateFetchRange(opts.Date, time.Local)
}

func resolveDateTimeFetchRange(fromInput, toInput string, loc *time.Location) (messageFetchRange, error) {
	start, err := datetime.Parse(fromInput, loc)
	if err != nil {
		return messageFetchRange{}, usagef("invalid from date/time %q", fromInput)
	}
	end, err := datetime.Parse(toInput, loc)
	if err != nil {
		return messageFetchRange{}, usagef("invalid to date/time %q", toInput)
	}
	if !start.Before(end) {
		return messageFetchRange{}, usagef("from date/time must be before to date/time")
	}
	return messageFetchRange{mode: "datetime-range", start: start, end: end}, nil
}

func resolveDateFetchRange(input string, loc *time.Location) (messageFetchRange, error) {
	parsed, err := datetime.Parse(input, loc)
	if err != nil {
		return messageFetchRange{}, usagef("invalid date %q", input)
	}
	localDate := parsed.In(loc)
	start := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, loc)
	return messageFetchRange{mode: "date", start: start, end: start.AddDate(0, 0, 1)}, nil
}

func (r messageFetchRange) oldestTS() string { return slack.FormatTS(r.start.Unix()) }

func (r messageFetchRange) latestTS() string {
	if r.end.IsZero() {
		return ""
	}
	return slack.FormatTS(r.end.Unix())
}

func (r messageFetchRange) progressLabel() string {
	switch r.mode {
	case "date":
		return "on " + r.start.Format("2006-01-02") + " (local time)"
	case "datetime-range":
		return "from " + r.start.UTC().Format(time.RFC3339) + " (included) to " + r.end.UTC().Format(time.RFC3339) + " (not included)"
	}
	return "since " + r.start.Format("2006-01-02")
}

func (r messageFetchRange) footerRangeLabel() string {
	start := r.start.UTC().Format(time.RFC3339)
	if r.end.IsZero() {
		return fmt.Sprintf("From %s (included); no end boundary", start)
	}
	return fmt.Sprintf("From %s (included); to %s (not included)", start, r.end.UTC().Format(time.RFC3339))
}

func (r messageFetchRange) footerOptionsLabel(opts Options) string {
	limit := fmt.Sprintf("--max-posts %d, --max-attachment-size %s", opts.MaxPosts, humanBytes(opts.MaxAttachBytes))
	if len(opts.ExcludeBodyEmoji) > 0 {
		limit += ", --exclude-body-emoji " + strings.Join(opts.ExcludeBodyEmoji, ",")
	}
	if len(opts.ExcludeReactionEmoji) > 0 {
		limit += ", --exclude-reaction-emoji " + strings.Join(opts.ExcludeReactionEmoji, ",")
	}
	if r.mode == "date" {
		return fmt.Sprintf("--date %q, %s", opts.Date, limit)
	}
	if r.mode == "datetime-range" {
		return fmt.Sprintf("--from %q, --to %q, %s", opts.From, opts.To, limit)
	}
	return fmt.Sprintf("--days %d, %s", opts.Days, limit)
}

func threadParticipants(replies []*render.MessageView) ([]render.ThreadParticipantView, int) {
	const maxParticipants = 3

	seen := map[string]bool{}
	var participants []render.ThreadParticipantView
	uniqueCount := 0
	for _, reply := range replies {
		if reply == nil || reply.IsSystem || reply.Author == "" {
			continue
		}
		key := reply.Author + "\x00" + reply.AvatarPath
		if seen[key] {
			continue
		}
		seen[key] = true
		uniqueCount++
		if len(participants) < maxParticipants {
			participants = append(participants, render.ThreadParticipantView{
				Author:        reply.Author,
				AvatarPath:    reply.AvatarPath,
				AvatarInitial: reply.AvatarInitial,
			})
		}
	}
	if uniqueCount <= maxParticipants {
		return participants, 0
	}
	return participants, uniqueCount - maxParticipants
}

// --- channel selection -----------------------------------------------------

func chooseChannel(channels []slack.Channel, opts Options, wsLine string, p *ui.Printer) (slack.Channel, error) {
	keyword := opts.ChannelKeyword
	var candidates []slack.Channel
	if keyword == "" {
		candidates = channels
	} else {
		for _, ch := range channels {
			if ch.ID == keyword || ch.Name == strings.TrimPrefix(keyword, "#") {
				return ch, nil
			}
		}
		lower := strings.ToLower(strings.TrimPrefix(keyword, "#"))
		for _, ch := range channels {
			if strings.Contains(strings.ToLower(ch.Name), lower) {
				candidates = append(candidates, ch)
			}
		}
	}

	switch {
	case len(candidates) == 0:
		return slack.Channel{}, usagef("no channel matched %q. Check the channel name or ID; for private channels the bot must be a member.", keyword)
	case len(candidates) == 1 && keyword != "":
		return candidates[0], nil
	}

	if len(candidates) > maxSelectable {
		p.StopPhase()
		p.Warnf("%d channels matched. Run again with a more specific channel name or a channel ID.", len(candidates))
		return slack.Channel{}, usagef("too many candidates (%d). Re-run as: slapex <channel-id-or-name>", len(candidates))
	}

	if opts.PromptTTY == nil || opts.NoInteractive {
		p.StopPhase()
		if keyword == "" {
			p.Warnf("No channel specified. Select one of the following channels:")
		} else {
			p.Warnf("Multiple channels matched %q.", keyword)
		}
		p.Plainf("")
		p.Plainf("Workspace: %s", wsLine)
		p.Plainf("")
		p.Plainf("Candidates:")
		for _, ch := range candidates {
			p.Plainf("  %s", channelLine(ch))
		}
		p.Plainf("")
		p.Plainf("Run again with a more specific channel:")
		p.Plainf("")
		p.Plainf("  slapex %s", candidates[0].ID)
		return slack.Channel{}, usagef("channel selection required but interactive selection is unavailable")
	}

	// Stop the live phase line before huh draws its selection UI on the
	// controlling terminal; a running spinner on stderr would fight it.
	p.StopPhase()
	return selectChannel(candidates, opts.PromptTTY)
}

func selectChannel(candidates []slack.Channel, tty *os.File) (slack.Channel, error) {
	opts := make([]huh.Option[int], len(candidates))
	for i, ch := range candidates {
		opts[i] = huh.NewOption(channelLine(ch), i)
	}
	idx := 0
	// Drive the form entirely over the controlling terminal so selection works
	// even when stdout/stderr are redirected or wrapped (e.g. `op run` masking).
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[int]().
			Title("Select a channel").
			Options(opts...).
			Value(&idx),
	)).WithInput(tty).WithOutput(tty)
	if err := form.Run(); err != nil {
		return slack.Channel{}, usagef("channel selection cancelled")
	}
	return candidates[idx], nil
}

func channelLine(ch slack.Channel) string {
	return fmt.Sprintf("#%s (%s)", ch.Name, channelMeta(ch))
}

// channelMeta is the parenthesized channel detail shared by channelLine and
// the Channel phase line (usage-flow.md「処理対象の表示」).
func channelMeta(ch slack.Channel) string {
	visibility := "public"
	if ch.IsPrivate {
		visibility = "private"
	}
	state := "active"
	if ch.IsArchived {
		state = "archived"
	}
	membership := "not member"
	if ch.IsMember {
		membership = "member"
	}
	return fmt.Sprintf("%s, %s, %s, %s", ch.ID, visibility, state, membership)
}

// --- view building ----------------------------------------------------------

var systemSubtypes = map[string]bool{
	"channel_join":      true,
	"channel_leave":     true,
	"channel_topic":     true,
	"channel_purpose":   true,
	"channel_name":      true,
	"channel_archive":   true,
	"channel_unarchive": true,
	"pinned_item":       true,
}

var actorPrefixSystemSubtypes = map[string]bool{
	"channel_topic":   true,
	"channel_purpose": true,
	"channel_name":    true,
}

var normalSubtypes = map[string]bool{
	"":                 true,
	"file_share":       true,
	"thread_broadcast": true,
	"bot_message":      true,
	"me_message":       true,
}

type builder struct {
	users   map[string]*slack.User
	avatars map[string]string
	emoji   *emoji.Resolver
	assets  *output.Assets
	limit   int64
}

// UserName implements render.TextResolver.
func (b *builder) UserName(id string) string {
	if u, ok := b.users[id]; ok {
		return u.DisplayName()
	}
	return id
}

// EmojiHTML implements render.TextResolver.
func (b *builder) EmojiHTML(name string) string {
	literal := html.EscapeString(":" + name + ":")
	r := b.emoji.Resolve(name)
	switch {
	case r.Unicode != "":
		return r.Unicode
	case r.ImageURL != "":
		if rel, ok := b.assets.Save(output.KindEmoji, r.ImageURL, output.AssetMeta{EmojiName: name}); ok {
			return `<img class="emoji" src="` + rel + `" alt="` + literal + `" title="` + literal + `">`
		}
		return literal
	default:
		return literal
	}
}

func (b *builder) messageView(m *slack.Message) *render.MessageView {
	t := tsTime(m.TS)
	v := &render.MessageView{
		TimeLabel: t.Format("2006-01-02 15:04"),
		TimeISO:   t.UTC().Format(time.RFC3339),
		Edited:    m.Edited != nil,
	}

	switch {
	case systemSubtypes[m.Subtype]:
		v.IsSystem = true
		v.Body = b.systemBody(m)
		return v
	case m.Subtype == "tombstone":
		v.Author = "(削除)"
		v.AvatarInitial = "?"
		v.Body = render.Safe("(削除されたメッセージ)")
		return v
	case !normalSubtypes[m.Subtype] && m.Text == "":
		v.IsSystem = true
		v.Body = render.Safe("(未対応のメッセージ種別: " + html.EscapeString(m.Subtype) + ")")
		return v
	}

	v.Author = b.authorName(m)
	v.AvatarPath = b.avatars[m.User]
	v.AvatarInitial = initialOf(v.Author)
	v.Italic = m.Subtype == "me_message"
	if m.Text != "" {
		v.Body = render.Mrkdwn(m.Text, b)
	}
	b.addFiles(v, m)
	b.addUnfurls(v, m)
	for _, r := range m.Reactions {
		v.Reactions = append(v.Reactions, render.ReactionView{
			Emoji: render.Safe(b.EmojiHTML(r.Name)),
			Count: r.Count,
		})
	}
	return v
}

func (b *builder) systemBody(m *slack.Message) template.HTML {
	body := render.Mrkdwn(m.Text, b)
	if suffix, ok := b.channelJoinInviterSuffix(m); ok {
		body += suffix
	}
	name, ok := b.userDisplayName(m.User)
	if !ok || !actorPrefixSystemSubtypes[m.Subtype] || systemTextStartsWithActor(m.Text, m.User, name) {
		return body
	}
	prefix := `<span class="mention">` + html.EscapeString("@"+name) + `</span> `
	return render.Safe(prefix) + body
}

func (b *builder) channelJoinInviterSuffix(m *slack.Message) (template.HTML, bool) {
	if m.Subtype != "channel_join" || m.Inviter == "" || m.Inviter == m.User || systemTextMentionsUser(m.Text, m.Inviter) {
		return "", false
	}
	name, ok := b.userDisplayName(m.Inviter)
	if !ok {
		return "", false
	}
	suffix := ` <span class="system-context">(invited by <span class="mention">` + html.EscapeString("@"+name) + `</span>)</span>`
	return render.Safe(suffix), true
}

func (b *builder) authorName(m *slack.Message) string {
	if m.User != "" {
		return b.UserName(m.User)
	}
	if m.BotProfile != nil && m.BotProfile.Name != "" {
		return m.BotProfile.Name
	}
	if m.Username != "" {
		return m.Username
	}
	if m.BotID != "" {
		return m.BotID
	}
	return "(unknown)"
}

func (b *builder) userDisplayName(id string) (string, bool) {
	if id == "" {
		return "", false
	}
	u, ok := b.users[id]
	if !ok {
		return "", false
	}
	return u.DisplayName(), true
}

func systemTextStartsWithActor(text, userID, displayName string) bool {
	for _, prefix := range []string{
		"<@" + userID + ">",
		"<@" + userID + "|",
		"@" + displayName,
	} {
		if prefix != "" && strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func systemTextMentionsUser(text, userID string) bool {
	return userID != "" && (strings.Contains(text, "<@"+userID+">") || strings.Contains(text, "<@"+userID+"|"))
}

func (b *builder) addFiles(v *render.MessageView, m *slack.Message) {
	for i := range m.Files {
		f := &m.Files[i]
		switch {
		case f.Mode == "tombstone":
			v.FilesList = append(v.FilesList, render.FileView{Name: "(削除されたファイル)"})
		case strings.HasPrefix(f.Mimetype, "image/"):
			b.addImage(v, f)
		default:
			b.addAttachmentFile(v, f)
		}
	}
}

func (b *builder) addImage(v *render.MessageView, f *slack.File) {
	meta := output.AssetMeta{FileID: f.ID, OriginalName: f.Name, Mimetype: f.Mimetype, SizeBytes: f.Size}
	thumbURL := f.ThumbURL()
	if thumbURL == "" && f.IsExternal {
		v.FilesList = append(v.FilesList, render.FileView{Name: f.Name, Note: "(外部サービス連携の画像のため保存対象外)"})
		return
	}
	img := render.ImageView{Name: f.Name}
	if thumbURL != "" {
		img.ThumbPath, _ = b.assets.Save(output.KindUploadThumb, thumbURL, meta)
	}
	switch {
	case b.limit > 0 && f.Size > b.limit:
		b.assets.SkipTooLarge(output.KindUploadOriginal, f.DownloadURL(), meta)
		img.Note = fmt.Sprintf("original はサイズ上限超過のため保存されませんでした。(%s: %s, 上限 %s)",
			f.Name, humanBytes(f.Size), humanBytes(b.limit))
	case f.DownloadURL() != "":
		if rel, ok := b.assets.Save(output.KindUploadOriginal, f.DownloadURL(), meta); ok {
			img.OriginalPath = rel
		} else {
			img.Note = "original の取得に失敗しました。"
		}
	}
	if img.ThumbPath == "" && img.OriginalPath != "" {
		img.ThumbPath = img.OriginalPath
	}
	if img.ThumbPath == "" {
		v.FilesList = append(v.FilesList, render.FileView{Name: f.Name, Note: "画像の取得に失敗しました。"})
		return
	}
	v.Images = append(v.Images, img)
}

func (b *builder) addAttachmentFile(v *render.MessageView, f *slack.File) {
	meta := output.AssetMeta{FileID: f.ID, OriginalName: f.Name, Mimetype: f.Mimetype, SizeBytes: f.Size}
	name := f.Name
	if name == "" {
		name = f.ID
	}
	switch {
	case f.IsExternal || f.DownloadURL() == "":
		v.FilesList = append(v.FilesList, render.FileView{Name: name, Note: "(外部サービス連携のファイルのため保存対象外)"})
	case b.limit > 0 && f.Size > b.limit:
		b.assets.SkipTooLarge(output.KindAttachment, f.DownloadURL(), meta)
		// 置換表示にはファイル名 / file ID / 元サイズ / 上限を含める
		// (output-format.md「添付ファイルのサイズ制限」)。file ID は取得できる
		// 場合のみ添える。
		detail := fmt.Sprintf("%s, 上限 %s", humanBytes(f.Size), humanBytes(b.limit))
		if f.ID != "" {
			detail = fmt.Sprintf("file ID: %s, %s", f.ID, detail)
		}
		v.FilesList = append(v.FilesList, render.FileView{
			Name: name,
			Note: "サイズオーバーのため保存されませんでした。(" + detail + ")",
		})
	default:
		if rel, ok := b.assets.Save(output.KindAttachment, f.DownloadURL(), meta); ok {
			v.FilesList = append(v.FilesList, render.FileView{Name: name, Path: rel})
		} else {
			v.FilesList = append(v.FilesList, render.FileView{Name: name, Note: "取得に失敗しました。"})
		}
	}
}

func (b *builder) addUnfurls(v *render.MessageView, m *slack.Message) {
	for i := range m.Attachments {
		a := &m.Attachments[i]
		uv := render.UnfurlView{Service: a.ServiceName, Title: a.Title}
		if strings.HasPrefix(a.TitleLink, "http://") || strings.HasPrefix(a.TitleLink, "https://") {
			uv.TitleHref = a.TitleLink
		}
		if a.Text != "" {
			uv.Text = render.Mrkdwn(a.Text, b)
		}
		if a.ServiceIcon != "" {
			uv.ServiceIconPath, _ = b.assets.Save(output.KindServiceIcon, a.ServiceIcon, output.AssetMeta{})
		}
		if src := a.PreviewImageURL(); src != "" {
			uv.ImagePath, _ = b.assets.Save(output.KindOGImage, src, output.AssetMeta{})
		}
		if uv.Service == "" && uv.ServiceIconPath == "" && uv.Title == "" && uv.Text == "" && uv.ImagePath == "" {
			continue
		}
		v.Unfurls = append(v.Unfurls, uv)
	}
}

// --- cache files -------------------------------------------------------------

type cachedUser struct {
	DisplayName string `json:"display_name"`
	RealName    string `json:"real_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

func writeCaches(dir string, now time.Time, auth *slack.AuthTest, ch slack.Channel, opts Options,
	fetchRange messageFetchRange, wsLabel, chLabel string, timeline, threads, replyTotal, excludedTotal, saved, skipped, failed int,
	users map[string]*slack.User, customEmoji map[string]string, assets *output.Assets) error {

	common := output.CacheCommon{SchemaVersion: output.SchemaVersion, GeneratedAt: now.UTC().Format(time.RFC3339)}

	metadata := map[string]any{
		"schema_version": common.SchemaVersion,
		"generated_at":   common.GeneratedAt,
		"tool_version":   opts.ToolVersion,
		"workspace": map[string]string{
			"team_id": auth.TeamID, "name": auth.Team, "url": auth.URL, "domain": hostOf(auth.URL),
		},
		"channel": map[string]any{
			"id": ch.ID, "name": ch.Name,
			"is_private": ch.IsPrivate, "is_archived": ch.IsArchived, "is_member": ch.IsMember,
		},
		"fetch": map[string]any{
			// Keep the original v1 flat fields for cache compatibility. New
			// consumers should prefer target_range and options, which separate
			// the absolute fetch boundary from the CLI input that produced it.
			"days": opts.Days, "max_posts": opts.MaxPosts,
			"max_attachment_size_bytes": opts.MaxAttachBytes,
			"oldest_ts":                 fetchRange.oldestTS(),
			"latest_ts":                 fetchRange.latestTS(),
			"executed_at":               now.UTC().Format(time.RFC3339),
			"target_range":              fetchRange.metadataTargetRange(),
			"options":                   fetchRange.metadataOptions(opts),
		},
		"labels": map[string]string{
			"workspace_label": wsLabel, "channel_label": chLabel,
			"workspace_name": auth.Team, "channel_name": ch.Name,
		},
		"counts": map[string]int{
			"timeline_messages": timeline, "threads": threads, "replies": replyTotal,
			"excluded_messages": excludedTotal,
			"assets_saved":      saved, "assets_skipped": skipped, "assets_failed": failed,
		},
	}
	if err := output.WriteCacheFile(dir, "metadata.json", metadata); err != nil {
		return err
	}

	manifest := map[string]any{
		"schema_version": common.SchemaVersion,
		"generated_at":   common.GeneratedAt,
		"assets":         assets.Entries(),
	}
	if err := output.WriteCacheFile(dir, "assets_manifest.json", manifest); err != nil {
		return err
	}

	cachedUsers := map[string]cachedUser{}
	for id, u := range users {
		cachedUsers[id] = cachedUser{DisplayName: u.DisplayName(), RealName: u.RealName, AvatarURL: avatarURL(u)}
	}
	apiCache := map[string]any{
		"schema_version": common.SchemaVersion,
		"generated_at":   common.GeneratedAt,
		"users":          cachedUsers,
		"emoji":          customEmoji,
		"workspace":      auth,
		"channel":        ch,
	}
	return output.WriteCacheFile(dir, "slack_api_cache.json", apiCache)
}

func (r messageFetchRange) metadataTargetRange() map[string]any {
	end := any(nil)
	endSlackTS := any(nil)
	if !r.end.IsZero() {
		end = r.end.UTC().Format(time.RFC3339)
		endSlackTS = r.latestTS()
	}
	return map[string]any{
		"start":          r.start.UTC().Format(time.RFC3339),
		"end":            end,
		"start_slack_ts": r.oldestTS(),
		"end_slack_ts":   endSlackTS,
	}
}

func (r messageFetchRange) metadataOptions(opts Options) map[string]any {
	values := map[string]any{
		"range_mode":                r.mode,
		"max_posts":                 opts.MaxPosts,
		"max_attachment_size_bytes": opts.MaxAttachBytes,
	}
	if len(opts.ExcludeBodyEmoji) > 0 {
		values["exclude_body_emoji"] = opts.ExcludeBodyEmoji
	}
	if len(opts.ExcludeReactionEmoji) > 0 {
		values["exclude_reaction_emoji"] = opts.ExcludeReactionEmoji
	}
	if r.mode == "date" {
		values["date"] = opts.Date
	} else if r.mode == "datetime-range" {
		values["from"] = opts.From
		values["to"] = opts.To
	} else {
		values["days"] = opts.Days
	}
	return values
}

type messageFilter struct {
	bodyEmoji      emoji.NameSet
	reactionEmoji  emoji.NameSet
	excluded       map[string]struct{}
	excludedThread map[string]struct{}
}

func newMessageFilter(bodyEmoji, reactionEmoji []string) *messageFilter {
	return &messageFilter{
		bodyEmoji:      emoji.NewNameSet(bodyEmoji),
		reactionEmoji:  emoji.NewNameSet(reactionEmoji),
		excluded:       map[string]struct{}{},
		excludedThread: map[string]struct{}{},
	}
}

func (f *messageFilter) Include(message *slack.Message) bool {
	if message == nil {
		return false
	}
	if !f.bodyEmoji.MatchesText(message.Text) && !f.matchesReaction(message.Reactions) {
		return true
	}
	f.Exclude(message)
	return false
}

func (f *messageFilter) matchesReaction(reactions []slack.Reaction) bool {
	for _, reaction := range reactions {
		if f.reactionEmoji.MatchesName(reaction.Name) {
			return true
		}
	}
	return false
}

func (f *messageFilter) Exclude(message *slack.Message) {
	if message == nil {
		return
	}
	f.excluded[message.TS] = struct{}{}
	if message.IsThreadParent() {
		f.ExcludeThread(message.TS)
	}
}

func (f *messageFilter) ExcludeThread(threadTS string) {
	if threadTS != "" {
		f.excludedThread[threadTS] = struct{}{}
	}
}

func (f *messageFilter) ThreadExcluded(threadTS string) bool {
	_, ok := f.excludedThread[threadTS]
	return ok
}

func (f *messageFilter) Enabled() bool {
	return len(f.bodyEmoji) > 0 || len(f.reactionEmoji) > 0
}

func (f *messageFilter) ExcludedCount() int {
	return len(f.excluded)
}

func newThreadIDs(messages []slack.Message, fetched map[string]bool, inspectBroadcasts bool) []string {
	seen := map[string]bool{}
	var ids []string
	for i := range messages {
		threadTS := messageThreadTS(&messages[i])
		if threadTS == "" || seen[threadTS] {
			continue
		}
		if _, ok := fetched[threadTS]; ok {
			continue
		}
		if !messages[i].IsThreadParent() && (!inspectBroadcasts || messages[i].Subtype != "thread_broadcast") {
			continue
		}
		seen[threadTS] = true
		ids = append(ids, threadTS)
	}
	return ids
}

func messageThreadTS(message *slack.Message) string {
	if message == nil {
		return ""
	}
	if message.ThreadTS != "" {
		return message.ThreadTS
	}
	if message.IsThreadParent() {
		return message.TS
	}
	return ""
}

func oldestMessageTS(messages []slack.Message) string {
	oldest := ""
	for i := range messages {
		if oldest == "" || tsLess(messages[i].TS, oldest) {
			oldest = messages[i].TS
		}
	}
	return oldest
}

// --- small helpers -----------------------------------------------------------

var reMention = regexp.MustCompile(`<@([UW][A-Z0-9]+)[|>]`)

func collectUserIDs(messages []slack.Message, replies map[string][]slack.Message) []string {
	seen := map[string]bool{}
	add := func(m *slack.Message) {
		if m.User != "" {
			seen[m.User] = true
		}
		if m.Inviter != "" {
			seen[m.Inviter] = true
		}
		for _, match := range reMention.FindAllStringSubmatch(m.Text, -1) {
			seen[match[1]] = true
		}
	}
	for i := range messages {
		add(&messages[i])
	}
	for _, rs := range replies {
		for i := range rs {
			add(&rs[i])
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
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

func channelURL(workspaceURL, channelID string) string {
	if workspaceURL == "" || channelID == "" {
		return ""
	}
	return strings.TrimRight(workspaceURL, "/") + "/archives/" + channelID
}

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

func initialOf(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
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
