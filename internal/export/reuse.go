package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// reusableCache holds the parts of a previous run's .cache/ that --reuse-cache
// can reuse once validated: the resolved users and custom emoji (to skip
// users.info / emoji.list) and the saved-asset manifest entries (to copy files
// instead of downloading). Message bodies are never cached and are always
// re-fetched (doc/design/cache.md, decision log 0030).
type reusableCache struct {
	schemaVersion int
	teamID        string
	channelID     string
	users         map[string]cachedUser
	emoji         map[string]string
	savedAssets   map[string]output.ManifestEntry // source_url -> saved entry
	oldDir        string                          // previous run's channel directory
}

// resolveReuseCache loads and validates the --reuse-cache directory. On any
// problem — a load failure (missing / unparseable file) or a failed validation
// condition — it logs a warning and returns nil so the run falls back to a
// normal fetch instead of erroring out (doc/design/cache.md, decision log 0030).
func resolveReuseCache(path, teamID, channelID string, logf func(string, ...any)) *reusableCache {
	rc, err := loadReuseCache(path)
	if err != nil {
		logf("warning: --reuse-cache %s cannot be used (%v); fetching normally", path, err)
		return nil
	}
	if reason, ok := rc.validate(teamID, channelID); !ok {
		logf("warning: --reuse-cache %s cannot be used (%s); fetching normally", path, reason)
		return nil
	}
	logf("Reusing cache from %s (cached users/emoji/assets; messages are re-fetched)", path)
	return rc
}

// loadReuseCache reads the three .cache/ files at path (a previous run's .cache/
// directory). A missing or unparseable file is returned as an error so the
// caller falls back to a normal fetch ("検証不能(ファイル欠落、parse 不能)" in
// cache.md). The asset files live next to the .cache/ directory, so the previous
// channel directory is path's parent.
func loadReuseCache(path string) (*reusableCache, error) {
	clean := filepath.Clean(path)

	var meta struct {
		SchemaVersion int `json:"schema_version"`
		Workspace     struct {
			TeamID string `json:"team_id"`
		} `json:"workspace"`
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
	}
	if err := readCacheJSON(filepath.Join(clean, "metadata.json"), &meta); err != nil {
		return nil, err
	}

	var api struct {
		Users map[string]cachedUser `json:"users"`
		Emoji map[string]string     `json:"emoji"`
	}
	if err := readCacheJSON(filepath.Join(clean, "slack_api_cache.json"), &api); err != nil {
		return nil, err
	}

	var manifest struct {
		Assets []output.ManifestEntry `json:"assets"`
	}
	if err := readCacheJSON(filepath.Join(clean, "assets_manifest.json"), &manifest); err != nil {
		return nil, err
	}

	saved := map[string]output.ManifestEntry{}
	for _, e := range manifest.Assets {
		if e.Status == "saved" && e.LocalPath != "" && e.SourceURL != "" {
			saved[e.SourceURL] = e
		}
	}

	return &reusableCache{
		schemaVersion: meta.SchemaVersion,
		teamID:        meta.Workspace.TeamID,
		channelID:     meta.Channel.ID,
		users:         api.Users,
		emoji:         api.Emoji,
		savedAssets:   saved,
		oldDir:        filepath.Dir(clean),
	}, nil
}

func readCacheJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return nil
}

// validate checks the three reuse conditions (decision log 0030): schema_version
// matches the current implementation, and the cached team_id / channel ID match
// the workspace and channel resolved this run. It returns a human-readable reason
// when the cache must not be reused. --days / --max-posts differences are not
// considered (cache.md).
func (rc *reusableCache) validate(teamID, channelID string) (string, bool) {
	switch {
	case rc.schemaVersion != output.SchemaVersion:
		return fmt.Sprintf("schema_version %d does not match the current %d", rc.schemaVersion, output.SchemaVersion), false
	case rc.teamID != teamID:
		return fmt.Sprintf("cached workspace %q does not match the current %q", rc.teamID, teamID), false
	case rc.channelID != channelID:
		return fmt.Sprintf("cached channel %q does not match the current %q", rc.channelID, channelID), false
	}
	return "", true
}

// reuseSource adapts the loaded cache for output.Assets asset copying.
func (rc *reusableCache) reuseSource() *output.ReuseSource {
	return &output.ReuseSource{OldDir: rc.oldDir, Entries: rc.savedAssets}
}

// toUser reconstructs the minimal slack.User the builder needs (resolved display
// name and avatar URL) from a cached entry, so a cached user needs no users.info
// call this run.
func (c cachedUser) toUser(id string) *slack.User {
	u := &slack.User{ID: id, RealName: c.RealName}
	u.Profile.DisplayName = c.DisplayName
	u.Profile.RealName = c.RealName
	u.Profile.Image72 = c.AvatarURL
	return u
}
