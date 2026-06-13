// Package output manages the export directory layout, asset files and the
// .cache/ intermediate files (doc/design/output-format.md, cache.md).
package output

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// Downloader is the asset fetch dependency (implemented by *slack.Client).
type Downloader interface {
	Download(ctx context.Context, srcURL string, limit int64, w io.Writer) (int64, string, error)
}

// Root creates the output root directory. When base is empty, a
// slapex-<yyyymmdd>-<hhmm> directory under the current directory is used,
// with a numeric suffix when it already exists.
func Root(base string, now time.Time) (string, error) {
	if base != "" {
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", err
		}
		return base, nil
	}
	stamp := now.Format("20060102-1504")
	name := fmt.Sprintf("slapex-%s", stamp)
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", name, i)
		}
		err := os.Mkdir(candidate, 0o755)
		if err == nil {
			return candidate, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
}

// AssetKind values stored in the manifest (doc/design/cache.md).
const (
	KindEmoji          = "emoji"
	KindOGImage        = "og_image"
	KindUploadThumb    = "upload_thumb"
	KindUploadOriginal = "upload_original"
	KindAttachment     = "attachment"
	KindAvatar         = "avatar"
)

var kindDirs = map[string]string{
	KindEmoji:          "assets/emoji",
	KindOGImage:        "assets/og-images",
	KindUploadThumb:    "assets/uploads/thumbs",
	KindUploadOriginal: "assets/uploads/originals",
	KindAttachment:     "assets/attachments",
	KindAvatar:         "assets/avatars",
}

// ManifestEntry mirrors the assets_manifest.json schema (doc/design/cache.md).
type ManifestEntry struct {
	Kind         string `json:"kind"`
	SourceURL    string `json:"source_url"`
	LocalPath    string `json:"local_path,omitempty"`
	FileID       string `json:"file_id,omitempty"`
	EmojiName    string `json:"emoji_name,omitempty"`
	OriginalName string `json:"original_name,omitempty"`
	Mimetype     string `json:"mimetype,omitempty"`
	SizeBytes    int64  `json:"size_bytes,omitempty"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
}

// AssetMeta carries optional manifest metadata for an asset.
type AssetMeta struct {
	FileID       string
	EmojiName    string
	OriginalName string
	Mimetype     string
	SizeBytes    int64
}

// Assets downloads asset URLs into the per-kind directories with URL-hash
// file names, deduplicates by source URL, and records every outcome.
type Assets struct {
	ctx     context.Context
	dl      Downloader
	dir     string
	limit   int64 // per-file byte limit, 0 = unlimited
	known   map[string]string
	entries []ManifestEntry
	reuse   *ReuseSource // previous run's assets to copy instead of downloading
	reused  int          // assets copied from the reuse source
	Logf    func(format string, args ...any)
}

// ReuseSource lets Save copy an already-saved asset from a previous run's
// output instead of downloading it again, for --reuse-cache (doc/design/cache.md,
// decision log 0030). OldDir is the previous run's channel directory (the parent
// of the reused .cache/); Entries maps each previously saved source_url to its
// manifest entry, whose LocalPath is relative to OldDir.
type ReuseSource struct {
	OldDir  string
	Entries map[string]ManifestEntry
}

func NewAssets(ctx context.Context, dl Downloader, dir string, limit int64) *Assets {
	return &Assets{
		ctx:   ctx,
		dl:    dl,
		dir:   dir,
		limit: limit,
		known: map[string]string{},
		Logf:  func(string, ...any) {},
	}
}

// SetReuseSource enables copy-from-previous-run behaviour in Save (--reuse-cache).
func (a *Assets) SetReuseSource(r *ReuseSource) { a.reuse = r }

// Reused returns how many assets were copied from the reuse source instead of
// being downloaded.
func (a *Assets) Reused() int { return a.reused }

// SkipTooLarge records a file that was not downloaded due to the size limit.
func (a *Assets) SkipTooLarge(kind, srcURL string, meta AssetMeta) {
	a.entries = append(a.entries, ManifestEntry{
		Kind: kind, SourceURL: srcURL, Status: "skipped_size",
		FileID: meta.FileID, OriginalName: meta.OriginalName,
		Mimetype: meta.Mimetype, SizeBytes: meta.SizeBytes,
	})
}

// Save downloads srcURL (unless already saved) and returns the path relative
// to the output directory. ok is false when the asset is unavailable.
func (a *Assets) Save(kind, srcURL string, meta AssetMeta) (relPath string, ok bool) {
	if srcURL == "" {
		return "", false
	}
	if rel, seen := a.known[srcURL]; seen {
		return rel, rel != ""
	}

	if a.reuse != nil {
		if rel, ok := a.copyFromReuse(kind, srcURL, meta); ok {
			return rel, true
		}
	}

	sum := md5.Sum([]byte(srcURL))
	base := hex.EncodeToString(sum[:])
	rel := filepath.Join(kindDirs[kind], base) // extension added after download

	tmp, err := os.CreateTemp(a.dir, "asset-*")
	if err != nil {
		a.record(kind, srcURL, meta, "", "failed", err.Error())
		return "", false
	}
	defer os.Remove(tmp.Name())

	limit := a.limit
	if kind == KindEmoji || kind == KindUploadThumb || kind == KindOGImage || kind == KindAvatar {
		limit = 0 // size limit applies to originals and attachments only
	}
	size, contentType, err := a.dl.Download(a.ctx, srcURL, limit, tmp)
	tmp.Close()
	if err != nil {
		status := "failed"
		if errors.Is(err, slack.ErrTooLarge) {
			status = "skipped_size"
		}
		a.record(kind, srcURL, meta, "", status, err.Error())
		a.Logf("asset failed (%s): %s", kind, err)
		return "", false
	}

	rel += extensionFor(meta, srcURL, contentType)
	dst := filepath.Join(a.dir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		a.record(kind, srcURL, meta, "", "failed", err.Error())
		return "", false
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		a.record(kind, srcURL, meta, "", "failed", err.Error())
		return "", false
	}
	if meta.SizeBytes == 0 {
		meta.SizeBytes = size
	}
	if meta.Mimetype == "" {
		meta.Mimetype = contentType
	}
	a.record(kind, srcURL, meta, filepath.ToSlash(rel), "saved", "")
	return filepath.ToSlash(rel), true
}

func (a *Assets) record(kind, srcURL string, meta AssetMeta, rel, status, errMsg string) {
	a.known[srcURL] = rel
	a.entries = append(a.entries, ManifestEntry{
		Kind: kind, SourceURL: srcURL, LocalPath: rel,
		FileID: meta.FileID, EmojiName: meta.EmojiName, OriginalName: meta.OriginalName,
		Mimetype: meta.Mimetype, SizeBytes: meta.SizeBytes,
		Status: status, Error: errMsg,
	})
}

// copyFromReuse copies an asset that a previous run already saved into this
// run's output instead of downloading it, when --reuse-cache matched (decision
// log 0030). It reuses the old LocalPath verbatim so the deterministic
// md5+extension layout (and therefore the HTML references) stays identical
// across runs. It returns false — so Save falls back to a normal download — when
// srcURL was not a saved asset before or the previous file is gone.
func (a *Assets) copyFromReuse(kind, srcURL string, meta AssetMeta) (string, bool) {
	entry, ok := a.reuse.Entries[srcURL]
	if !ok || entry.LocalPath == "" {
		return "", false
	}
	src := filepath.Join(a.reuse.OldDir, filepath.FromSlash(entry.LocalPath))
	info, err := os.Stat(src)
	if err != nil || info.IsDir() {
		return "", false
	}
	dst := filepath.Join(a.dir, filepath.FromSlash(entry.LocalPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", false
	}
	if err := copyFile(src, dst); err != nil {
		return "", false
	}
	if meta.SizeBytes == 0 {
		meta.SizeBytes = entry.SizeBytes
	}
	if meta.Mimetype == "" {
		meta.Mimetype = entry.Mimetype
	}
	a.reused++
	a.record(kind, srcURL, meta, entry.LocalPath, "saved", "")
	return entry.LocalPath, true
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (a *Assets) Entries() []ManifestEntry { return a.entries }

// Counts returns saved / skipped / failed totals for the summary.
func (a *Assets) Counts() (saved, skipped, failed int) {
	for _, e := range a.entries {
		switch e.Status {
		case "saved":
			saved++
		case "skipped_size":
			skipped++
		default:
			failed++
		}
	}
	return
}

func extensionFor(meta AssetMeta, srcURL, contentType string) string {
	if meta.OriginalName != "" {
		if ext := filepath.Ext(meta.OriginalName); ext != "" && len(ext) <= 8 {
			return strings.ToLower(ext)
		}
	}
	if i := strings.IndexAny(srcURL, "?#"); i >= 0 {
		srcURL = srcURL[:i]
	}
	if ext := filepath.Ext(srcURL); ext != "" && len(ext) <= 8 {
		return strings.ToLower(ext)
	}
	if mt, _, err := mime.ParseMediaType(contentType); err == nil {
		switch mt {
		case "image/jpeg":
			return ".jpg"
		case "image/png":
			return ".png"
		case "image/gif":
			return ".gif"
		case "image/webp":
			return ".webp"
		}
	}
	return ".bin"
}

// CacheCommon is the shared header of every .cache/ file.
type CacheCommon struct {
	SchemaVersion int    `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
}

const SchemaVersion = 1

// WriteCacheFile writes one .cache/ JSON file.
func WriteCacheFile(outDir, name string, payload any) error {
	cacheDir := filepath.Join(outDir, ".cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, name), append(data, '\n'), 0o644)
}

// RemoveCache removes the .cache directory (default behaviour without
// --keep-cache, doc/design/cache.md).
func RemoveCache(outDir string) error {
	return os.RemoveAll(filepath.Join(outDir, ".cache"))
}
