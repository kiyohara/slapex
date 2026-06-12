package output

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

func TestRoot(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	now := time.Date(2026, 6, 12, 13, 45, 30, 0, time.UTC)
	first, err := Root("", now)
	if err != nil {
		t.Fatalf("Root first: %v", err)
	}
	if first != "slapex-20260612-1345" {
		t.Fatalf("first root = %q, want %q", first, "slapex-20260612-1345")
	}
	if info, err := os.Stat(filepath.Join(tmp, first)); err != nil || !info.IsDir() {
		t.Fatalf("first root was not created as a directory: info=%v err=%v", info, err)
	}

	second, err := Root("", now)
	if err != nil {
		t.Fatalf("Root second: %v", err)
	}
	if second != "slapex-20260612-1345-2" {
		t.Fatalf("second root = %q, want %q", second, "slapex-20260612-1345-2")
	}

	base := filepath.Join(tmp, "custom", "out")
	got, err := Root(base, now)
	if err != nil {
		t.Fatalf("Root base: %v", err)
	}
	if got != base {
		t.Fatalf("base root = %q, want %q", got, base)
	}
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		t.Fatalf("base root was not created as a directory: info=%v err=%v", info, err)
	}
}

func TestAssetsSaveRecordsManifestAndCounts(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	dl := &fakeDownloader{
		content: map[string]fakeDownload{
			"https://example.com/avatar":          {body: "avatar", contentType: "image/jpeg"},
			"https://example.com/emoji":           {body: "emoji", contentType: "image/gif"},
			"https://example.com/og":              {body: "og", contentType: "image/png"},
			"https://example.com/thumb":           {body: "thumb", contentType: "image/webp"},
			"https://example.com/original":        {body: "original", contentType: "image/png"},
			"https://example.com/attachment":      {body: "attachment", contentType: "application/octet-stream"},
			"https://example.com/fail":            {err: errors.New("download failed")},
			"https://example.com/too-large-error": {err: slack.ErrTooLarge},
		},
	}
	assets := NewAssets(context.Background(), dl, outDir, 10)

	cases := []struct {
		kind string
		url  string
		dir  string
		ext  string
		meta AssetMeta
	}{
		{kind: KindAvatar, url: "https://example.com/avatar", dir: "assets/avatars", ext: ".jpg"},
		{kind: KindEmoji, url: "https://example.com/emoji", dir: "assets/emoji", ext: ".gif", meta: AssetMeta{EmojiName: "party"}},
		{kind: KindOGImage, url: "https://example.com/og", dir: "assets/og-images", ext: ".png"},
		{kind: KindUploadThumb, url: "https://example.com/thumb", dir: "assets/uploads/thumbs", ext: ".webp"},
		{kind: KindUploadOriginal, url: "https://example.com/original", dir: "assets/uploads/originals", ext: ".png", meta: AssetMeta{FileID: "F001", OriginalName: "photo.png"}},
		{kind: KindAttachment, url: "https://example.com/attachment", dir: "assets/attachments", ext: ".txt", meta: AssetMeta{FileID: "F002", OriginalName: "report.txt", Mimetype: "text/plain", SizeBytes: 42}},
	}

	for _, tc := range cases {
		got, ok := assets.Save(tc.kind, tc.url, tc.meta)
		if !ok {
			t.Fatalf("Save(%s, %s) ok = false", tc.kind, tc.url)
		}
		want := filepath.ToSlash(filepath.Join(tc.dir, md5Hex(tc.url)+tc.ext))
		if got != want {
			t.Fatalf("Save(%s, %s) = %q, want %q", tc.kind, tc.url, got, want)
		}
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(got))); err != nil {
			t.Fatalf("saved file %q missing: %v", got, err)
		}
	}

	assets.SkipTooLarge(KindAttachment, "https://example.com/large", AssetMeta{FileID: "F003", OriginalName: "large.zip", Mimetype: "application/zip", SizeBytes: 99})
	if rel, ok := assets.Save(KindAttachment, "https://example.com/fail", AssetMeta{FileID: "F004"}); ok || rel != "" {
		t.Fatalf("failed download returned rel=%q ok=%v, want empty false", rel, ok)
	}
	if rel, ok := assets.Save(KindUploadOriginal, "https://example.com/too-large-error", AssetMeta{FileID: "F005"}); ok || rel != "" {
		t.Fatalf("too large download returned rel=%q ok=%v, want empty false", rel, ok)
	}

	saved, skipped, failed := assets.Counts()
	if saved != len(cases) || skipped != 2 || failed != 1 {
		t.Fatalf("Counts() = saved:%d skipped:%d failed:%d, want saved:%d skipped:2 failed:1", saved, skipped, failed, len(cases))
	}

	entries := assets.Entries()
	if len(entries) != len(cases)+3 {
		t.Fatalf("entries len = %d, want %d", len(entries), len(cases)+3)
	}
	assertEntry(t, entries, "https://example.com/large", "skipped_size", "")
	assertEntry(t, entries, "https://example.com/fail", "failed", "")
	assertEntry(t, entries, "https://example.com/too-large-error", "skipped_size", "")
}

func TestExtensionFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		meta        AssetMeta
		srcURL      string
		contentType string
		want        string
	}{
		{
			name:        "uses original name first",
			meta:        AssetMeta{OriginalName: "Report.PDF"},
			srcURL:      "https://example.com/download",
			contentType: "image/png",
			want:        ".pdf",
		},
		{
			name:        "uses url extension before query",
			srcURL:      "https://example.com/image.JPG?token=redacted",
			contentType: "image/png",
			want:        ".jpg",
		},
		{
			name:        "uses supported mimetype",
			srcURL:      "https://example.com/image",
			contentType: "image/webp; charset=binary",
			want:        ".webp",
		},
		{
			name:        "falls back to bin",
			srcURL:      "https://example.com/download",
			contentType: "application/octet-stream",
			want:        ".bin",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extensionFor(tt.meta, tt.srcURL, tt.contentType); got != tt.want {
				t.Fatalf("extensionFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteCacheFile(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	generatedAt := time.Date(2026, 6, 12, 1, 2, 3, 0, time.UTC).Format(time.RFC3339)
	payloads := map[string]any{
		"assets_manifest.json": struct {
			CacheCommon
			Assets []ManifestEntry `json:"assets"`
		}{
			CacheCommon: CacheCommon{SchemaVersion: SchemaVersion, GeneratedAt: generatedAt},
			Assets: []ManifestEntry{{
				Kind:      KindEmoji,
				SourceURL: "https://example.com/emoji.gif",
				LocalPath: "assets/emoji/example.gif",
				Status:    "saved",
			}},
		},
		"metadata.json": struct {
			CacheCommon
			ToolVersion string `json:"tool_version"`
		}{
			CacheCommon: CacheCommon{SchemaVersion: SchemaVersion, GeneratedAt: generatedAt},
			ToolVersion: "test",
		},
		"slack_api_cache.json": struct {
			CacheCommon
			Users map[string]string `json:"users"`
		}{
			CacheCommon: CacheCommon{SchemaVersion: SchemaVersion, GeneratedAt: generatedAt},
			Users:       map[string]string{},
		},
	}

	for name, payload := range payloads {
		if err := WriteCacheFile(outDir, name, payload); err != nil {
			t.Fatalf("WriteCacheFile(%s): %v", name, err)
		}

		raw, err := os.ReadFile(filepath.Join(outDir, ".cache", name))
		if err != nil {
			t.Fatalf("read cache file %s: %v", name, err)
		}
		var got struct {
			SchemaVersion int    `json:"schema_version"`
			GeneratedAt   string `json:"generated_at"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal cache file %s: %v", name, err)
		}
		if got.SchemaVersion != SchemaVersion || got.GeneratedAt != generatedAt {
			t.Fatalf("%s common fields = schema:%d generated_at:%q, want schema:%d generated_at:%q", name, got.SchemaVersion, got.GeneratedAt, SchemaVersion, generatedAt)
		}
	}
}

func TestRemoveCache(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	cacheDir := filepath.Join(outDir, ".cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "metadata.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	if err := RemoveCache(outDir); err != nil {
		t.Fatalf("RemoveCache: %v", err)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatalf("cache dir still exists or unexpected error: %v", err)
	}
}

type fakeDownload struct {
	body        string
	contentType string
	err         error
}

type fakeDownloader struct {
	content map[string]fakeDownload
}

func (f *fakeDownloader) Download(_ context.Context, srcURL string, _ int64, w io.Writer) (int64, string, error) {
	item, ok := f.content[srcURL]
	if !ok {
		return 0, "", errors.New("unexpected url")
	}
	if item.err != nil {
		return 0, "", item.err
	}
	n, err := io.WriteString(w, item.body)
	if err != nil {
		return 0, "", err
	}
	return int64(n), item.contentType, nil
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func assertEntry(t *testing.T, entries []ManifestEntry, srcURL, status, localPath string) {
	t.Helper()
	for _, entry := range entries {
		if entry.SourceURL == srcURL {
			if entry.Status != status || entry.LocalPath != localPath {
				t.Fatalf("entry for %s = status:%q local_path:%q, want status:%q local_path:%q", srcURL, entry.Status, entry.LocalPath, status, localPath)
			}
			return
		}
	}
	t.Fatalf("entry for %s not found", srcURL)
}
