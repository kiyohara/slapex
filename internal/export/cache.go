// Cache output: assembles the metadata.json / assets_manifest.json /
// slack_api_cache.json payloads written under .cache/ (doc/design/cache.md).
// Reading them back for --reuse-cache lives in reuse.go.
package export

import (
	"time"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

type cachedUser struct {
	DisplayName string `json:"display_name"`
	RealName    string `json:"real_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	// IsBot preserves users.info is_bot so --reuse-cache can still tell a bot
	// user's post from a person's and render the APP chip (decision log 0054).
	IsBot bool `json:"is_bot,omitempty"`
}

// cachedBot is one bots.info result: the app name and the icon URL slapex saved,
// so --reuse-cache can skip the call (doc/design/cache.md).
type cachedBot struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

func writeCaches(dir string, now time.Time, auth *slack.AuthTest, ch slack.Channel, opts Options,
	fetchRange messageFetchRange, wsLabel, chLabel string, timeline, threads, replyTotal, excludedTotal, saved, skipped, failed int,
	users map[string]*slack.User, bots map[string]*slack.Bot, customEmoji map[string]string, assets *output.Assets) error {

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
		cachedUsers[id] = cachedUser{
			DisplayName: u.DisplayName(), RealName: u.RealName, AvatarURL: avatarURL(u), IsBot: u.IsBot,
		}
	}
	cachedBots := map[string]cachedBot{}
	for id, bot := range bots {
		cachedBots[id] = cachedBot{Name: bot.Name, AvatarURL: bot.Icons.URL()}
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
