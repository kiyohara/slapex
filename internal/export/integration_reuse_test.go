package export

// Integration --reuse-cache scenarios for v1-10 (Issue #24). Each test builds on
// the v1-07 fake Slack server harness (happyPathScenario / newFakeSlackServer)
// and runs the export twice against one shared server: run 1 populates and keeps
// the .cache/, run 2 points --reuse-cache at it. Comparing the fake server's
// per-endpoint request counts between the two runs is the practical check that
// reuse skipped users.info / emoji.list / asset downloads, while history /
// replies are always re-fetched. The expected behaviour is the confirmed spec in
// doc/design/cache.md「--reuse-cache の整合性検証」and decision log 0030.
//
// The two runs share one server on purpose: cached avatar / emoji / asset URLs
// embed the workspace (here, the test server) URL, so reuse only matches when the
// workspace is the same across runs — exactly the team_id validation condition.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// --- case 1: a valid cache reuses users / emoji / assets ---------------------

func TestRunIntegrationReuseCacheReducesRequests(t *testing.T) {
	t.Parallel()

	r := runReuseScenario(t, happyPathScenario(), nil)

	// users.info and emoji.list are not called again, and no asset is downloaded
	// a second time: all came from the reused cache.
	if d := delta(r.before, r.after, "/api/users.info"); d != 0 {
		t.Fatalf("users.info delta = %d, want 0 (resolved users reused from cache)", d)
	}
	if d := delta(r.before, r.after, "/api/emoji.list"); d != 0 {
		t.Fatalf("emoji.list delta = %d, want 0 (emoji reused from cache)", d)
	}
	for _, p := range r.assets {
		if d := delta(r.before, r.after, p); d != 0 {
			t.Fatalf("asset %s download delta = %d, want 0 (copied from cache)", p, d)
		}
	}

	// Message bodies are always re-fetched (spec): history and replies run again.
	if d := delta(r.before, r.after, "/api/conversations.history"); d < 1 {
		t.Fatalf("history delta = %d, want >= 1 (messages re-fetched every run)", d)
	}
	if d := delta(r.before, r.after, "/api/conversations.replies"); d < 1 {
		t.Fatalf("replies delta = %d, want >= 1 (messages re-fetched every run)", d)
	}

	if !logsContain(r.logs2, "Reusing cache from") {
		t.Fatalf("run 2 did not report cache reuse:\n%s", strings.Join(r.logs2, "\n"))
	}

	// Output is equivalent to run 1: byte-identical asset files, and the reused
	// users / emoji still render (a cached display name and the copied custom
	// emoji image both appear without any users.info / emoji.list call).
	assertAssetsIdentical(t, r.dir1, r.dir2)
	body := readIndexHTML(t, r.dir2)
	mustContain(t, body, "Bob")           // display name from the cached user
	mustContain(t, body, `assets/emoji/`) // custom emoji image copied from cache
	mustContain(t, body, "First timeline note")
}

// --- cases 2-5: invalid / unusable cache falls back to a normal fetch --------

// Each case makes the kept cache fail one of the validation conditions (or makes
// it unreadable) and asserts run 2 falls back: it re-resolves users, re-fetches
// emoji and re-downloads assets (request counts do not drop), and prints the
// fallback warning instead of erroring out (cache.md: 警告して通常取得へフォールバック).
func TestRunIntegrationReuseCacheFallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		tamper func(t *testing.T, cacheDir string)
	}{
		{
			name: "team_id mismatch",
			tamper: func(t *testing.T, dir string) {
				rewriteJSON(t, filepath.Join(dir, "metadata.json"), func(m map[string]any) {
					m["workspace"].(map[string]any)["team_id"] = "TOTHER999"
				})
			},
		},
		{
			name: "schema_version mismatch",
			tamper: func(t *testing.T, dir string) {
				rewriteJSON(t, filepath.Join(dir, "metadata.json"), func(m map[string]any) {
					m["schema_version"] = 999
				})
			},
		},
		{
			name: "channel ID mismatch",
			tamper: func(t *testing.T, dir string) {
				rewriteJSON(t, filepath.Join(dir, "metadata.json"), func(m map[string]any) {
					m["channel"].(map[string]any)["id"] = "CWRONG999"
				})
			},
		},
		{
			name: "missing cache file",
			tamper: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, "slack_api_cache.json")); err != nil {
					t.Fatalf("remove cache file: %v", err)
				}
			},
		},
		{
			name: "unparseable cache file",
			tamper: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte("{ not json"), 0o644); err != nil {
					t.Fatalf("corrupt cache file: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := runReuseScenario(t, happyPathScenario(), tc.tamper)

			if d := delta(r.before, r.after, "/api/users.info"); d < 1 {
				t.Fatalf("users.info delta = %d, want >= 1 (fallback re-resolves users)", d)
			}
			if d := delta(r.before, r.after, "/api/emoji.list"); d != 1 {
				t.Fatalf("emoji.list delta = %d, want 1 (fallback re-fetches emoji)", d)
			}
			downloaded := 0
			for _, p := range r.assets {
				downloaded += delta(r.before, r.after, p)
			}
			if downloaded < 1 {
				t.Fatalf("asset download delta = %d, want >= 1 (fallback re-downloads assets)", downloaded)
			}
			if !logsContain(r.logs2, "fetching normally") {
				t.Fatalf("run 2 did not warn + fall back:\n%s", strings.Join(r.logs2, "\n"))
			}
		})
	}
}

// --- case 6: an image_48-only avatar is preserved across reuse ---------------

// Regression guard for the avatar fidelity fix: a user whose avatar comes from
// image_48 (image_72 empty) must still have its avatar copied on reuse, not
// dropped. If the cache persisted only image_72, run 2 would save no avatar and
// its assets/ subtree would differ from run 1's, failing assertAssetsIdentical.
func TestRunIntegrationReuseCacheImage48Avatar(t *testing.T) {
	t.Parallel()

	r := runReuseScenario(t, image48AvatarScenario(), nil)

	// The image_48 avatar is copied from the cache (no re-download) and the
	// assets/ subtree is byte-identical to the first run.
	if d := delta(r.before, r.after, "/files/avatar48.png"); d != 0 {
		t.Fatalf("avatar download delta = %d, want 0 (image_48 avatar copied from cache)", d)
	}
	assertAssetsIdentical(t, r.dir1, r.dir2)
}

// --- shared harness ----------------------------------------------------------

// reuseRun holds the two output directories, the fake server request counts
// captured after each run, and run 2's logs.
type reuseRun struct {
	dir1, dir2 string
	assets     []string // fake-server asset paths (sc.Assets keys)
	before     map[string]int
	after      map[string]int
	logs2      []string
}

// runReuseScenario runs the happy-path export twice against one shared fake
// server: run 1 keeps its cache, then tamper (if any) mutates that cache, then
// run 2 reuses it. It returns the request-count snapshots and run 2's logs.
func runReuseScenario(t *testing.T, sc exportScenario, tamper func(t *testing.T, cacheDir string)) reuseRun {
	t.Helper()

	fake := newFakeSlackServer(t, &sc)
	t.Cleanup(fake.Close)

	client := slack.New(integrationTestToken,
		slack.WithBaseURL(fake.URL()+"/api/"),
		slack.WithSleeper(func(context.Context, time.Duration) error { return nil }),
	)

	assets := assetPaths(sc)
	countPaths := append([]string{
		"/api/auth.test", "/api/conversations.list",
		"/api/conversations.history", "/api/conversations.replies",
		"/api/users.info", "/api/emoji.list",
	}, assets...)

	dir1, err := Run(context.Background(), client, reuseOptions(t, true), func(string, ...any) {})
	if err != nil {
		t.Fatalf("run 1 (populate cache) error: %v", err)
	}
	before := snapshotCounts(fake, countPaths)

	cacheDir := filepath.Join(dir1, ".cache")
	if tamper != nil {
		tamper(t, cacheDir)
	}

	opts2 := reuseOptions(t, false)
	opts2.ReuseCache = cacheDir
	var logs2 []string
	dir2, err := Run(context.Background(), client, opts2, appendLogf(&logs2))
	if err != nil {
		t.Fatalf("run 2 (reuse) error: %v\nlogs:\n%s", err, strings.Join(logs2, "\n"))
	}
	after := snapshotCounts(fake, countPaths)

	return reuseRun{dir1: dir1, dir2: dir2, assets: assets, before: before, after: after, logs2: logs2}
}

func reuseOptions(t *testing.T, keepCache bool) Options {
	t.Helper()
	return Options{
		ChannelKeyword: "project-alpha",
		OutputDir:      t.TempDir(),
		MaxPosts:       10,
		Days:           90,
		MaxAttachBytes: 1 << 20,
		KeepCache:      keepCache,
		ToolVersion:    "test",
	}
}

// image48AvatarScenario is a minimal valid scenario whose single resolved user
// has only an image_48 avatar (image_72 empty), exercising the image_48 fallback
// path through the cache.
func image48AvatarScenario() exportScenario {
	sc := baseScenario()
	u := sc.Users["U01"]
	u.Profile.Image72 = ""
	u.Profile.Image48 = "{{base}}/files/avatar48.png"
	sc.Users["U01"] = u
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000001.000000", User: "U01", Text: "hello"},
	}
	sc.Assets = map[string]fakeAsset{
		"/files/avatar48.png": {ContentType: "image/png", Body: "avatar-48-only"},
	}
	return sc
}

func assetPaths(sc exportScenario) []string {
	paths := make([]string, 0, len(sc.Assets))
	for p := range sc.Assets {
		paths = append(paths, p)
	}
	return paths
}

func snapshotCounts(fake *fakeSlackServer, paths []string) map[string]int {
	m := make(map[string]int, len(paths))
	for _, p := range paths {
		m[p] = fake.Count(p)
	}
	return m
}

func delta(before, after map[string]int, path string) int {
	return after[path] - before[path]
}

// appendLogf returns a logf that records formatted lines. Run logs sequentially
// from a single goroutine, so no synchronisation is needed.
func appendLogf(logs *[]string) func(string, ...any) {
	return func(format string, args ...any) {
		*logs = append(*logs, fmt.Sprintf(format, args...))
	}
}

func rewriteJSON(t *testing.T, path string, mutate func(m map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	mutate(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// assertAssetsIdentical fails unless the assets/ subtrees of the two output
// directories contain the same relative files with identical bytes.
func assertAssetsIdentical(t *testing.T, dir1, dir2 string) {
	t.Helper()
	files1 := collectAssetFiles(t, dir1)
	files2 := collectAssetFiles(t, dir2)
	if len(files1) != len(files2) {
		t.Fatalf("asset file count differs: run1=%d run2=%d", len(files1), len(files2))
	}
	if len(files1) == 0 {
		t.Fatalf("no asset files collected from run 1 (%s)", dir1)
	}
	for rel, b1 := range files1 {
		b2, ok := files2[rel]
		if !ok {
			t.Fatalf("run 2 missing asset %s", rel)
		}
		if !bytes.Equal(b1, b2) {
			t.Fatalf("asset %s differs between runs (run1=%d bytes, run2=%d bytes)", rel, len(b1), len(b2))
		}
	}
}

func collectAssetFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	root := filepath.Join(dir, "assets")
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("walk assets in %s: %v", dir, err)
	}
	return out
}
