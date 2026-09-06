package output

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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
			"https://example.com/avatar":         {body: "avatar", contentType: "image/jpeg"},
			"https://example.com/emoji":          {body: "emoji", contentType: "image/gif"},
			"https://example.com/og":             {body: "og", contentType: "image/png"},
			"https://example.com/service-icon":   {body: "service-icon", contentType: "image/png"},
			"https://example.com/workspace-icon": {body: "workspace-icon", contentType: "image/png"},
			"https://example.com/thumb":          {body: "thumb", contentType: "image/webp"},
			"https://example.com/original":       {body: "original", contentType: "image/png"},
			"https://example.com/attachment":     {body: "attachment", contentType: "application/octet-stream"},
			// Gravatar shape: the URL path says .jpg but the bytes are a PNG,
			// because gravatar redirects to the PNG default image (Issue #183).
			gravatarAvatarURL:                     {body: pngBody, contentType: "image/png"},
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
		{kind: KindServiceIcon, url: "https://example.com/service-icon", dir: "assets/service-icons", ext: ".png"},
		{kind: KindWorkspaceIcon, url: "https://example.com/workspace-icon", dir: "assets/workspace-icons", ext: ".png"},
		{kind: KindUploadThumb, url: "https://example.com/thumb", dir: "assets/uploads/thumbs", ext: ".webp"},
		{kind: KindUploadOriginal, url: "https://example.com/original", dir: "assets/uploads/originals", ext: ".png", meta: AssetMeta{FileID: "F001", OriginalName: "photo.png"}},
		{kind: KindAttachment, url: "https://example.com/attachment", dir: "assets/attachments", ext: ".txt", meta: AssetMeta{FileID: "F002", OriginalName: "report.txt", Mimetype: "text/plain", SizeBytes: 42}},
		{kind: KindAvatar, url: gravatarAvatarURL, dir: "assets/avatars", ext: ".png"},
	}

	for _, tc := range cases {
		got, ok := assets.Save(tc.kind, tc.url, tc.meta)
		if !ok {
			t.Fatalf("Save(%s, %s) ok = false", tc.kind, tc.url)
		}
		// The file name is the sha256 of the downloaded content, not of the URL.
		want := filepath.ToSlash(filepath.Join(tc.dir, sha256Hex(dl.content[tc.url].body)+tc.ext))
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

	// The gravatar avatar is saved as .png (from the content, not the .jpg URL)
	// and its manifest mimetype agrees with that extension.
	gravatar := findEntry(t, entries, gravatarAvatarURL)
	if want := ".png"; filepath.Ext(gravatar.LocalPath) != want {
		t.Fatalf("gravatar avatar local_path = %q, want extension %q", gravatar.LocalPath, want)
	}
	if gravatar.Mimetype != "image/png" {
		t.Fatalf("gravatar avatar mimetype = %q, want %q", gravatar.Mimetype, "image/png")
	}
}

func TestAssetsSaveContentHashDeduplicatesByContent(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	dl := &fakeDownloader{
		content: map[string]fakeDownload{
			"https://a.example.com/photo.png": {body: "same-bytes", contentType: "image/png"},
			"https://b.example.com/photo.png": {body: "same-bytes", contentType: "image/png"},
			"https://c.example.com/photo.png": {body: "different-bytes", contentType: "image/png"},
		},
	}
	assets := NewAssets(context.Background(), dl, outDir, 0)

	// Two different source URLs with identical content and extension resolve to the
	// same content-hash file name, so the sample diff stays stable when only the
	// URL changed (Issue #135).
	first, ok := assets.Save(KindUploadOriginal, "https://a.example.com/photo.png", AssetMeta{})
	if !ok {
		t.Fatalf("Save(a) ok = false")
	}
	want := filepath.ToSlash(filepath.Join("assets/uploads/originals", sha256Hex("same-bytes")+".png"))
	if first != want {
		t.Fatalf("content-hash path = %q, want %q", first, want)
	}
	second, ok := assets.Save(KindUploadOriginal, "https://b.example.com/photo.png", AssetMeta{})
	if !ok {
		t.Fatalf("Save(b) ok = false")
	}
	if first != second {
		t.Fatalf("identical content saved to different paths: %q vs %q", first, second)
	}

	// Different content resolves to a different file name.
	third, ok := assets.Save(KindUploadOriginal, "https://c.example.com/photo.png", AssetMeta{})
	if !ok {
		t.Fatalf("Save(c) ok = false")
	}
	if third == first {
		t.Fatalf("different content saved to the same path %q", third)
	}

	// Both deduplicated source URLs are recorded in the manifest, each pointing at
	// the same local_path.
	entries := assets.Entries()
	assertEntry(t, entries, "https://a.example.com/photo.png", "saved", first)
	assertEntry(t, entries, "https://b.example.com/photo.png", "saved", first)
}

// TestAssetsSaveReuseFromOwnOutputKeepsAsset covers --reuse-cache pointed at the
// directory this run writes to: the reuse copy's source and destination are one
// file, and copying it onto itself used to truncate it to 0 bytes, destroying
// both runs' asset (Issue #202). The file must be kept as it is and still count
// as reused, while a reuse source in another directory still copies.
func TestAssetsSaveReuseFromOwnOutputKeepsAsset(t *testing.T) {
	t.Parallel()

	const (
		srcURL   = "https://example.com/avatar.png"
		localRel = "assets/avatars/avatar.png"
		body     = "avatar-bytes"
	)
	oldDir := t.TempDir()
	writeAssetFile(t, oldDir, localRel, body)
	reuse := &ReuseSource{
		OldDir: oldDir,
		Entries: map[string]ManifestEntry{
			srcURL: {
				Kind: KindAvatar, SourceURL: srcURL, LocalPath: localRel,
				SizeBytes: int64(len(body)), Status: "saved",
			},
		},
	}

	// The downloader knows no URL, so any fallback to a normal download fails the
	// Save and is reported below instead of silently hiding the reuse behaviour.
	assets := NewAssets(context.Background(), &fakeDownloader{}, oldDir, 0)
	assets.SetReuseSource(reuse)

	rel, ok := assets.Save(KindAvatar, srcURL, AssetMeta{})
	if !ok || rel != localRel {
		t.Fatalf("Save into the reuse source directory = %q ok=%v, want %q true", rel, ok, localRel)
	}
	if got := readAssetFile(t, oldDir, localRel); got != body {
		t.Fatalf("reused asset content = %q, want %q (self-copy must not truncate it)", got, body)
	}
	if assets.Reused() != 1 {
		t.Fatalf("Reused() = %d, want 1", assets.Reused())
	}
	entry := findEntry(t, assets.Entries(), srcURL)
	if entry.Status != "saved" || entry.LocalPath != localRel {
		t.Fatalf("manifest entry = %+v, want status saved at %q", entry, localRel)
	}
	if entry.SizeBytes != int64(len(body)) {
		t.Fatalf("manifest size_bytes = %d, want %d (must match the file on disk)", entry.SizeBytes, len(body))
	}

	// A reuse source in another directory is unaffected: the asset is copied.
	newDir := t.TempDir()
	other := NewAssets(context.Background(), &fakeDownloader{}, newDir, 0)
	other.SetReuseSource(reuse)
	if rel, ok := other.Save(KindAvatar, srcURL, AssetMeta{}); !ok || rel != localRel {
		t.Fatalf("Save into a separate directory = %q ok=%v, want %q true", rel, ok, localRel)
	}
	if got := readAssetFile(t, newDir, localRel); got != body {
		t.Fatalf("copied asset content = %q, want %q", got, body)
	}
	if got := readAssetFile(t, oldDir, localRel); got != body {
		t.Fatalf("reuse source content after copy = %q, want %q", got, body)
	}
}

// TestCopyFileOntoItselfKeepsContent guards copyFile itself, so a future caller
// that resolves both sides to one file cannot empty it (Issue #202).
func TestCopyFileOntoItselfKeepsContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "asset.bin")
	if err := os.WriteFile(path, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// Same path, and the same file reached through a different spelling.
	for _, dst := range []string{path, filepath.Join(dir, ".", "asset.bin")} {
		if err := copyFile(path, dst); err != nil {
			t.Fatalf("copyFile(%q, %q) error: %v", path, dst, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if string(got) != "keep me" {
			t.Fatalf("content after copyFile onto itself (dst %q) = %q, want %q", dst, got, "keep me")
		}
	}

	// A real copy to a different file still works.
	dst := filepath.Join(dir, "copy.bin")
	if err := copyFile(path, dst); err != nil {
		t.Fatalf("copyFile to a new path: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read copy: %v", err)
	}
	if string(got) != "keep me" {
		t.Fatalf("copied content = %q, want %q", got, "keep me")
	}
}

func writeAssetFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func readAssetFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestAssetsLimitFor(t *testing.T) {
	t.Parallel()

	assets := NewAssets(context.Background(), &fakeDownloader{}, t.TempDir(), 10)

	tests := []struct {
		kind string
		want int64
	}{
		{kind: KindEmoji, want: 0},
		{kind: KindUploadThumb, want: 0},
		{kind: KindAvatar, want: 0},
		{kind: KindOGImage, want: publicPreviewAssetLimit},
		{kind: KindServiceIcon, want: publicPreviewAssetLimit},
		{kind: KindWorkspaceIcon, want: publicPreviewAssetLimit},
		{kind: KindUploadOriginal, want: 10},
		{kind: KindAttachment, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			t.Parallel()
			if got := assets.limitFor(tt.kind); got != tt.want {
				t.Fatalf("limitFor(%s) = %d, want %d", tt.kind, got, tt.want)
			}
		})
	}
}

func TestExtensionFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		meta        AssetMeta
		srcURL      string
		contentType string
		sniffed     string
		want        string
	}{
		{
			name:        "sniffed content wins over the url extension",
			srcURL:      gravatarAvatarURL,
			contentType: "image/png",
			sniffed:     "image/png",
			want:        ".png",
		},
		{
			name:    "sniffed content wins over the original name",
			meta:    AssetMeta{OriginalName: "Report.PDF"},
			srcURL:  "https://example.com/download",
			sniffed: "application/pdf",
			want:    ".pdf",
		},
		{
			name:    "sniffed icon",
			srcURL:  "https://example.com/favicon",
			sniffed: "image/x-icon",
			want:    ".ico",
		},
		{
			name:        "uses original name first when the sniff is inconclusive",
			meta:        AssetMeta{OriginalName: "Report.PDF"},
			srcURL:      "https://example.com/download",
			contentType: "image/png",
			sniffed:     "text/plain",
			want:        ".pdf",
		},
		{
			name:        "uses url extension before query",
			srcURL:      "https://example.com/image.JPG?token=redacted",
			contentType: "image/png",
			sniffed:     "text/plain",
			want:        ".jpg",
		},
		{
			name:    "keeps the url extension for unsniffable svg",
			srcURL:  "https://example.com/logo.svg",
			sniffed: "text/xml",
			want:    ".svg",
		},
		{
			name:        "uses supported mimetype",
			srcURL:      "https://example.com/image",
			contentType: "image/webp; charset=binary",
			want:        ".webp",
		},
		{
			name:        "uses svg mimetype when the url has no extension",
			srcURL:      "https://example.com/logo",
			contentType: "image/svg+xml",
			sniffed:     "text/xml",
			want:        ".svg",
		},
		{
			name:        "uses icon mimetype when the url has no extension",
			srcURL:      "https://example.com/favicon",
			contentType: "image/vnd.microsoft.icon",
			want:        ".ico",
		},
		{
			name:        "falls back to bin",
			srcURL:      "https://example.com/download",
			contentType: "application/octet-stream",
			want:        ".bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extensionFor(tt.meta, tt.srcURL, tt.contentType, tt.sniffed); got != tt.want {
				t.Fatalf("extensionFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMimetypeFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		sniffed     string
		want        string
	}{
		{
			name:        "prefers the sniffed type",
			contentType: "image/jpeg",
			sniffed:     "image/png",
			want:        "image/png",
		},
		{
			name:        "falls back to the response content type",
			contentType: "image/svg+xml",
			sniffed:     "text/xml",
			want:        "image/svg+xml",
		},
		{
			name:        "falls back for an unsniffable empty body",
			contentType: "application/octet-stream",
			want:        "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mimetypeFor(tt.contentType, tt.sniffed); got != tt.want {
				t.Fatalf("mimetypeFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHeadBufferDetect(t *testing.T) {
	t.Parallel()

	t.Run("detects from the first bytes only", func(t *testing.T) {
		t.Parallel()
		var head headBuffer
		// Written in several chunks, as the download's io.Copy does, and longer
		// than the sniff window so the cap is exercised.
		for _, chunk := range []string{pngBody[:4], pngBody[4:], strings.Repeat("x", 2*sniffLen)} {
			if n, err := head.Write([]byte(chunk)); n != len(chunk) || err != nil {
				t.Fatalf("Write(%d bytes) = %d, %v", len(chunk), n, err)
			}
		}
		if len(head.buf) != sniffLen {
			t.Fatalf("head buffer len = %d, want %d", len(head.buf), sniffLen)
		}
		if got := head.detect(); got != "image/png" {
			t.Fatalf("detect() = %q, want %q", got, "image/png")
		}
	})

	t.Run("empty download detects nothing", func(t *testing.T) {
		t.Parallel()
		var head headBuffer
		if got := head.detect(); got != "" {
			t.Fatalf("detect() = %q, want empty", got)
		}
	})
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
		t.Run(name, func(t *testing.T) {
			t.Parallel()
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
		})
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

// gravatarAvatarURL has the shape Slack's users.info returns for a gravatar
// user: a path ending in .jpg with the Slack default image as the d= fallback.
// The hash is a placeholder. Gravatar redirects to that default image, so the
// bytes that arrive are a PNG (Issue #183).
const gravatarAvatarURL = "https://secure.gravatar.com/avatar/0123456789abcdef0123456789abcdef.jpg?s=72&d=https%3A%2F%2Fexample.com%2Fdefault-72.png"

// pngBody is a fake download body that starts with the PNG magic bytes, so
// http.DetectContentType reports image/png for it.
const pngBody = "\x89PNG\r\n\x1a\n" + "fake png body"

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

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func findEntry(t *testing.T, entries []ManifestEntry, srcURL string) ManifestEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.SourceURL == srcURL {
			return entry
		}
	}
	t.Fatalf("entry for %s not found", srcURL)
	return ManifestEntry{}
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
