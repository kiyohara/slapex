package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// reusableCache holds the parts of a previous run's .cache/ that --reuse-cache
// can reuse once validated: the resolved users and custom emoji (to skip
// users.info / emoji.list) and the saved-asset manifest entries (to copy files
// instead of downloading). Message bodies are never cached and are always
// re-fetched (doc/design/cache.md, decision log 0030).
type reusableCache struct {
	teamID      string
	channelID   string
	users       map[string]cachedUser
	bots        map[string]cachedBot
	emoji       map[string]string
	savedAssets map[string]output.ManifestEntry // source_url -> saved entry
	oldDir      string                          // previous run's channel directory
}

// resolveReuseCache loads and validates the --reuse-cache directory. On any
// problem — a load failure (missing / unparseable file) or a failed validation
// condition — it logs a warning and returns nil so the run falls back to a
// normal fetch instead of erroring out (doc/design/cache.md, decision log 0030).
func resolveReuseCache(path, teamID, channelID string, p *ui.Printer) *reusableCache {
	rc, err := loadReuseCache(path)
	if err != nil {
		p.Warnf("--reuse-cache %s cannot be used (%v); fetching normally", path, err)
		return nil
	}
	if reason, ok := rc.validate(teamID, channelID); !ok {
		p.Warnf("--reuse-cache %s cannot be used (%s); fetching normally", path, reason)
		return nil
	}
	p.Infof("Reusing cache from %s (cached users/emoji/assets; messages are re-fetched)", path)
	return rc
}

// loadReuseCache reads the three .cache/ files at path. path may be either a
// previous run's .cache/ directory or the previous output directory that contains
// .cache/. A missing, unparseable, or schema-mismatched file is returned as an
// error so the caller falls back to a normal fetch ("検証不能(ファイル欠落、parse
// 不能)" / schema 不一致 in cache.md). All three files carry schema_version and
// every one must match the current implementation before any of its data is
// reused (decision log 0030). The asset files live next to the .cache/ directory,
// so the previous channel directory is the resolved cache directory's parent.
func loadReuseCache(path string) (*reusableCache, error) {
	clean := resolveReuseCacheDir(path)

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
	if err := checkSchema("metadata.json", meta.SchemaVersion); err != nil {
		return nil, err
	}

	// bots is absent from caches written before decision log 0054; the zero map
	// simply means every bot ID is resolved with bots.info this run, so an older
	// cache stays reusable and schema_version is unchanged.
	var api struct {
		SchemaVersion int                   `json:"schema_version"`
		Users         map[string]cachedUser `json:"users"`
		Bots          map[string]cachedBot  `json:"bots"`
		Emoji         map[string]string     `json:"emoji"`
	}
	if err := readCacheJSON(filepath.Join(clean, "slack_api_cache.json"), &api); err != nil {
		return nil, err
	}
	if err := checkSchema("slack_api_cache.json", api.SchemaVersion); err != nil {
		return nil, err
	}

	var manifest struct {
		SchemaVersion int                    `json:"schema_version"`
		Assets        []output.ManifestEntry `json:"assets"`
	}
	if err := readCacheJSON(filepath.Join(clean, "assets_manifest.json"), &manifest); err != nil {
		return nil, err
	}
	if err := checkSchema("assets_manifest.json", manifest.SchemaVersion); err != nil {
		return nil, err
	}

	saved := map[string]output.ManifestEntry{}
	for _, e := range manifest.Assets {
		if e.Status == "saved" && e.LocalPath != "" && e.SourceURL != "" {
			saved[e.SourceURL] = e
		}
	}

	return &reusableCache{
		teamID:      meta.Workspace.TeamID,
		channelID:   meta.Channel.ID,
		users:       api.Users,
		bots:        api.Bots,
		emoji:       api.Emoji,
		savedAssets: saved,
		oldDir:      filepath.Dir(clean),
	}, nil
}

func resolveReuseCacheDir(path string) string {
	clean := filepath.Clean(path)
	if cacheMetadataExists(clean) {
		return clean
	}
	nested := filepath.Join(clean, ".cache")
	if cacheMetadataExists(nested) {
		return nested
	}
	return clean
}

func cacheMetadataExists(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "metadata.json"))
	return err == nil && !info.IsDir()
}

// checkSchema reports an error when a cache file's schema_version does not match
// the current implementation. All three .cache/ files carry schema_version
// (doc/design/cache.md); --reuse-cache requires every one to match before reusing
// any of its data, so a file written by an incompatible schema is never adopted
// (decision log 0030).
func checkSchema(file string, got int) error {
	if got != output.SchemaVersion {
		return fmt.Errorf("%s schema_version %d does not match the current %d", file, got, output.SchemaVersion)
	}
	return nil
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

// validate checks the cached team_id and channel ID against the workspace and
// channel resolved this run; both must match (decision log 0030). The third
// condition, schema_version, is enforced per-file at load time (loadReuseCache /
// checkSchema). It returns a human-readable reason when the cache must not be
// reused. --days / --max-posts differences are not considered (cache.md).
func (rc *reusableCache) validate(teamID, channelID string) (string, bool) {
	switch {
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
	u := &slack.User{ID: id, RealName: c.RealName, IsBot: c.IsBot}
	u.Profile.DisplayName = c.DisplayName
	u.Profile.RealName = c.RealName
	u.Profile.Image72 = c.AvatarURL
	return u
}

// toBot reconstructs the minimal slack.Bot the builder needs (app name and icon
// URL) from a cached entry, so a cached bot needs no bots.info call this run.
func (c cachedBot) toBot(id string) *slack.Bot {
	b := &slack.Bot{ID: id, Name: c.Name}
	b.Icons.Image72 = c.AvatarURL
	return b
}
