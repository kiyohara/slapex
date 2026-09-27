// Cache output: assembles the metadata.json / assets_manifest.json /
// slack_api_cache.json payloads written under .cache/ (doc/design/cache.md).
// The slack_api_cache.json user and bot entries live here with their
// conversions both ways: from the resolved users and bots when writing, and
// back to them for --reuse-cache. Loading and validating a cache for
// --reuse-cache lives in reuse.go.

package export

import (
	"time"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// cachedUser is one users.info result: the names and the avatar URL slapex
// saved, so --reuse-cache can skip the call (doc/design/cache.md).
type cachedUser struct {
	DisplayName string `json:"display_name"`
	RealName    string `json:"real_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	// IsBot preserves users.info is_bot so --reuse-cache can still tell a bot
	// user's post from a person's and render the APP chip (decision log 0054).
	IsBot bool `json:"is_bot,omitempty"`
}

// newCachedUser records u for slack_api_cache.json. The avatar URL is the one
// slapex saves (avatarURL), so a reused user's avatar has the same source_url.
func newCachedUser(u *slack.User) cachedUser {
	return cachedUser{DisplayName: u.DisplayName(), RealName: u.RealName, AvatarURL: avatarURL(u), IsBot: u.IsBot}
}

// toUser reconstructs the minimal slack.User the messageViewBuilder needs
// (resolved display name and avatar URL) from a cached entry, so a cached user
// needs no users.info call this run.
func (c cachedUser) toUser(id string) *slack.User {
	u := &slack.User{ID: id, RealName: c.RealName, IsBot: c.IsBot}
	u.Profile.DisplayName = c.DisplayName
	u.Profile.RealName = c.RealName
	u.Profile.Image72 = c.AvatarURL
	return u
}

// cachedBot is one bots.info result: the app name and the icon URL slapex saved,
// so --reuse-cache can skip the call (doc/design/cache.md).
type cachedBot struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// newCachedBot records b for slack_api_cache.json, with the icon URL slapex
// saves (slack.BotIcons.URL).
func newCachedBot(b *slack.Bot) cachedBot {
	return cachedBot{Name: b.Name, AvatarURL: b.Icons.URL()}
}

// toBot reconstructs the minimal slack.Bot the messageViewBuilder needs (app
// name and icon URL) from a cached entry, so a cached bot needs no bots.info
// call this run.
func (c cachedBot) toBot(id string) *slack.Bot {
	b := &slack.Bot{ID: id, Name: c.Name}
	b.Icons.Image72 = c.AvatarURL
	return b
}

// writeCaches writes the three .cache/ files into out's directory from the
// results of Run's stages. Every parameter has its own type, so an argument in
// the wrong position does not compile, and the counts are recorded as the
// Messages line, the Assets line and the Done summary report them. It takes no
// message bodies, which the cache never keeps (doc/design/cache.md).
func writeCaches(out outputDir, now time.Time, target exportTarget, opts Options, fetchRange messageFetchRange,
	counts exportCounts, assetTotals assetCounts, resolved resolvedUsers, customEmoji map[string]string, assets *output.Assets) error {

	common := output.CacheCommon{SchemaVersion: output.SchemaVersion, GeneratedAt: now.UTC().Format(time.RFC3339)}
	auth, ch := target.auth, target.channel

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
			"workspace_label": out.wsLabel, "channel_label": out.chLabel,
			"workspace_name": auth.Team, "channel_name": ch.Name,
		},
		"counts": map[string]int{
			"timeline_messages": counts.timeline, "threads": counts.threads, "replies": counts.replies,
			"excluded_messages": counts.excluded,
			"assets_saved":      assetTotals.saved, "assets_skipped": assetTotals.skipped, "assets_failed": assetTotals.failed,
		},
	}
	if err := output.WriteCacheFile(out.path, "metadata.json", metadata); err != nil {
		return err
	}

	manifest := map[string]any{
		"schema_version": common.SchemaVersion,
		"generated_at":   common.GeneratedAt,
		"assets":         assets.Entries(),
	}
	if err := output.WriteCacheFile(out.path, "assets_manifest.json", manifest); err != nil {
		return err
	}

	cachedUsers := map[string]cachedUser{}
	for id, u := range resolved.users {
		cachedUsers[id] = newCachedUser(u)
	}
	cachedBots := map[string]cachedBot{}
	for id, bot := range resolved.bots {
		cachedBots[id] = newCachedBot(bot)
	}
	apiCache := map[string]any{
		"schema_version": common.SchemaVersion,
		"generated_at":   common.GeneratedAt,
		"users":          cachedUsers,
		"bots":           cachedBots,
		"emoji":          customEmoji,
		"workspace":      auth,
		"channel":        ch,
	}
	return output.WriteCacheFile(out.path, "slack_api_cache.json", apiCache)
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
