// Assets stage: the images the page shows saved under assets/, and index.html
// rendered from the fetched messages with its stylesheet and static assets
// (doc/design/output-format.md, doc/design/html-rendering.md).

package export

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/render"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// saveWorkspaceIcon saves the workspace icon for the page header and returns
// its path, or "" when there is none or it was not saved.
func saveWorkspaceIcon(assets *output.Assets, teamInfo *slack.TeamInfo) string {
	rel, ok := assets.Save(output.KindWorkspaceIcon, workspaceIconURL(teamInfo), output.AssetMeta{})
	if !ok {
		return ""
	}
	return rel
}

// newMessageViewBuilder saves the avatars the page shows, each resolved user's
// and then each resolved bot's app icon in bot ID order, and returns the view
// builder that renders messages with them. App icons are saved as ordinary
// avatars (output.KindAvatar), so they land in assets/avatars/ next to the
// human ones and stay public downloads with no Authorization header
// (doc/guidelines/credential-scope-guidelines.md).
func newMessageViewBuilder(assets *output.Assets, resolved resolvedUsers, emojiResolver *emoji.Resolver, maxAttachmentBytes int64) *messageViewBuilder {
	avatars := map[string]string{}
	for id, u := range resolved.users {
		if rel, ok := assets.Save(output.KindAvatar, avatarURL(u), output.AssetMeta{}); ok {
			avatars[id] = rel
		}
	}
	botAvatars := map[string]string{}
	for _, id := range slices.Sorted(maps.Keys(resolved.bots)) {
		if rel, ok := assets.Save(output.KindAvatar, resolved.bots[id].Icons.URL(), output.AssetMeta{}); ok {
			botAvatars[id] = rel
		}
	}
	return &messageViewBuilder{
		users:              resolved.users,
		avatars:            avatars,
		bots:               resolved.bots,
		botAvatars:         botAvatars,
		emoji:              emojiResolver,
		assets:             assets,
		maxAttachmentBytes: maxAttachmentBytes,
	}
}

// buildTimeline turns the fetched messages into the page's timeline: a date
// divider whenever the local date changes, and each thread's replies under its
// parent. Rendering a message saves the files, images and custom emoji it
// shows.
func buildTimeline(views *messageViewBuilder, fetched fetchedMessages) []render.TimelineItem {
	var items []render.TimelineItem
	lastDate := ""
	for _, m := range fetched.timeline {
		date := tsTime(m.TS).Format("2006-01-02")
		if date != lastDate {
			items = append(items, render.TimelineItem{IsDateDivider: true, Date: date})
			lastDate = date
		}
		view := views.messageView(&m)
		if rs, ok := fetched.replies[m.TS]; ok {
			for i := range rs {
				view.Replies = append(view.Replies, views.messageView(&rs[i]))
			}
			view.ThreadParticipants, view.ThreadExtraParticipants = threadParticipants(view.Replies)
			view.RepliesTruncated = fetched.repliesTruncated[m.TS]
		}
		items = append(items, render.TimelineItem{Message: view})
	}
	return items
}

// buildPage assembles the page: the header naming the workspace and channel,
// the timeline, the --max-posts notice when truncated, and the footer's export
// information, whose Exported line shows now, the export clock.
func buildPage(target exportTarget, workspaceIcon string, items []render.TimelineItem, truncated bool,
	fetchRange messageFetchRange, opts Options, now time.Time) *render.PageData {
	_, tzOffset := now.Zone()
	return &render.PageData{
		WorkspaceName:     target.auth.Team,
		WorkspaceIconPath: workspaceIcon,
		WorkspaceHref:     target.auth.URL,
		ChannelName:       target.channel.Name,
		ChannelHref:       channelURL(target.auth.URL, target.channel.ID),
		WorkspaceLine:     target.wsLine,
		ChannelLine:       target.chLine,
		ExportedLine: fmt.Sprintf("%s (UTC%s) / %s",
			now.Format("2006-01-02 15:04"), offsetString(tzOffset), now.UTC().Format(time.RFC3339)),
		RangeLine:   fetchRange.footerRangeLabel(),
		OptionsLine: fetchRange.footerOptionsLabel(opts),
		ToolLine:    fmt.Sprintf("slapex %s", opts.ToolVersion),
		Items:       items,
		Truncated:   truncated,
	}
}

// writePage writes index.html, its stylesheet and the static assets into dir.
func writePage(dir string, page *render.PageData) error {
	htmlFile, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return err
	}
	if err := render.WriteHTML(htmlFile, page); err != nil {
		htmlFile.Close()
		return fmt.Errorf("render index.html: %w", err)
	}
	if err := htmlFile.Close(); err != nil {
		return err
	}
	if err := render.WriteStyleCSS(dir); err != nil {
		return err
	}
	return render.WriteStaticAssets(dir)
}

// assetCounts are the asset totals of the Assets stage, which the Assets line,
// metadata.json and the Done summary report.
type assetCounts struct {
	saved   int
	skipped int
	failed  int
	reused  int // of saved, those taken from the reuse cache instead of downloaded
}

// endAssetsPhase ends the Assets phase with the saved / skipped / failed
// totals, as a warning when an asset was skipped or failed, and how many
// assets came from the reuse cache. It returns those totals.
func endAssetsPhase(p *ui.Printer, assets *output.Assets) assetCounts {
	var totals assetCounts
	totals.saved, totals.skipped, totals.failed = assets.Counts()
	totals.reused = assets.Reused()
	status := ui.StatusSuccess
	if totals.skipped > 0 || totals.failed > 0 {
		status = ui.StatusWarn
	}
	meta := ""
	if totals.reused > 0 {
		meta = fmt.Sprintf("%d reused from cache, no download", totals.reused)
	}
	p.EndPhase(status, "Assets", fmt.Sprintf("%d saved, %d skipped by size limit, %d failed", totals.saved, totals.skipped, totals.failed), meta)
	return totals
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
