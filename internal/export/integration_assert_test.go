package export

// Generic read / assert helpers shared by the integration cases: reading
// index.html and the .cache/ JSON files of an export, substring and ordering
// checks on the HTML, manifest lookups, request-count and log-line checks.
// Helpers that encode one scenario's expected values stay with that scenario
// (assertOutputFiles / assertHTMLMarkers / assertCacheFiles for the happy path,
// assertExcludedMetadata / assertCacheOmits for the emoji filters,
// assertAssetsIdentical for --reuse-cache).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// --- HTML ---------------------------------------------------------------------

func readIndexHTML(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	return string(data)
}

func mustContain(t *testing.T, body, marker string) {
	t.Helper()
	if !strings.Contains(body, marker) {
		t.Fatalf("index.html missing marker %q", marker)
	}
}

func mustNotContain(t *testing.T, body, marker string) {
	t.Helper()
	if strings.Contains(body, marker) {
		t.Fatalf("index.html unexpectedly contains marker %q", marker)
	}
}

// assertOrder fails unless every marker appears in body, each strictly after
// the previous one's first occurrence.
func assertOrder(t *testing.T, body string, markers ...string) {
	t.Helper()

	last := -1
	for _, marker := range markers {
		idx := strings.Index(body, marker)
		if idx < 0 {
			t.Fatalf("missing marker %q", marker)
		}
		if idx <= last {
			t.Fatalf("marker %q appeared out of order", marker)
		}
		last = idx
	}
}

// --- .cache/ JSON -------------------------------------------------------------

func readJSON(t *testing.T, path string, out any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

// manifestEntryFull mirrors the fields of assets_manifest.json the cases assert
// on: kind / status, the saved path and mimetype, and the upload identity.
type manifestEntryFull struct {
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	SourceURL    string `json:"source_url"`
	LocalPath    string `json:"local_path"`
	Mimetype     string `json:"mimetype"`
	FileID       string `json:"file_id"`
	OriginalName string `json:"original_name"`
	SizeBytes    int64  `json:"size_bytes"`
}

func readManifestEntries(t *testing.T, dir string) []manifestEntryFull {
	t.Helper()
	var manifest struct {
		Assets []manifestEntryFull `json:"assets"`
	}
	readJSON(t, filepath.Join(dir, ".cache/assets_manifest.json"), &manifest)
	return manifest.Assets
}

func findManifest(entries []manifestEntryFull, match func(manifestEntryFull) bool) (manifestEntryFull, bool) {
	for _, e := range entries {
		if match(e) {
			return e, true
		}
	}
	return manifestEntryFull{}, false
}

func hasSavedAsset(assets []manifestEntryFull, kind string) bool {
	return slices.ContainsFunc(assets, func(asset manifestEntryFull) bool {
		return asset.Kind == kind && asset.Status == "saved"
	})
}

// --- fake server / printer output ---------------------------------------------

func assertEndpointCounts(t *testing.T, fake *fakeSlackServer, want map[string]int) {
	t.Helper()

	for path, count := range want {
		if got := fake.Count(path); got != count {
			t.Fatalf("%s count = %d, want %d", path, got, count)
		}
	}
}

func logsContain(logs []string, substr string) bool {
	for _, line := range logs {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

func hasSleepAtLeast(sleeps []time.Duration, atLeast time.Duration) bool {
	for _, d := range sleeps {
		if d >= atLeast {
			return true
		}
	}
	return false
}
