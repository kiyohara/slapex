// Package output manages the export directory layout, asset files and the
// .cache/ intermediate files (doc/design/output-format.md, cache.md).
package output

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
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
	KindServiceIcon    = "service_icon"
	KindWorkspaceIcon  = "workspace_icon"
)

const publicPreviewAssetLimit int64 = 5 << 20 // 5 MiB guard for third-party unfurl assets.

var kindDirs = map[string]string{
	KindEmoji:          "assets/emoji",
	KindOGImage:        "assets/og-images",
	KindUploadThumb:    "assets/uploads/thumbs",
	KindUploadOriginal: "assets/uploads/originals",
	KindAttachment:     "assets/attachments",
	KindAvatar:         "assets/avatars",
	KindServiceIcon:    "assets/service-icons",
	KindWorkspaceIcon:  "assets/workspace-icons",
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

// Assets downloads asset URLs into the per-kind directories with content-hash
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

// limitFor returns the per-file byte limit that applies to kind (0 = unlimited).
// The user-configurable size limit applies to original images and attachments.
// Third-party unfurl display assets get a fixed guard limit.
func (a *Assets) limitFor(kind string) int64 {
	switch kind {
	case KindEmoji, KindUploadThumb, KindAvatar:
		return 0
	case KindOGImage, KindServiceIcon, KindWorkspaceIcon:
		return publicPreviewAssetLimit
	default:
		return a.limit
	}
}

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

	tmp, err := os.CreateTemp(a.dir, "asset-*")
	if err != nil {
		a.record(kind, srcURL, meta, "", "failed", err.Error())
		return "", false
	}
	defer os.Remove(tmp.Name())

	// Hash the downloaded bytes (not the source URL) so the saved file name is a
	// content hash: identical content resolves to the same name, and regenerating
	// the samples does not churn file names just because a signed URL or the
	// fixture's base URL changed (decision log 0016 / 0052). The hash is computed
	// during the download via a MultiWriter, so the temp file is not re-read. The
	// same MultiWriter keeps the first bytes so the real format can be detected
	// from the content (decision log 0052, Issue #183) without a second read.
	h := sha256.New()
	var head headBuffer
	size, contentType, err := a.dl.Download(a.ctx, srcURL, a.limitFor(kind), io.MultiWriter(tmp, h, &head))
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

	sniffed := head.detect()
	base := hex.EncodeToString(h.Sum(nil))
	rel := filepath.Join(kindDirs[kind], base+extensionFor(meta, srcURL, contentType, sniffed))
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
		meta.Mimetype = mimetypeFor(contentType, sniffed)
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
// log 0030). It reuses the old LocalPath verbatim so the previous run's file
// name (and therefore the HTML references) stays identical across runs. Because
// the asset content is unchanged, a fresh download would resolve to the same
// content hash anyway (decision log 0052); reusing a cache written by an older
// URL-hash build simply keeps that build's names for the copied assets. It
// returns false — so Save falls back to a normal download — when srcURL was not
// a saved asset before or the previous file is gone.
func (a *Assets) copyFromReuse(kind, srcURL string, meta AssetMeta) (string, bool) {
	entry, ok := a.reuse.Entries[srcURL]
	if !ok || entry.LocalPath == "" {
		return "", false
	}
	// LocalPath comes from a previous run's manifest. Reject anything that is not
	// a contained relative path so a corrupted or untrusted cache cannot read or
	// write outside the old / new output directories (path traversal); such an
	// asset falls back to a normal download.
	if !filepath.IsLocal(filepath.FromSlash(entry.LocalPath)) {
		return "", false
	}
	src := filepath.Join(a.reuse.OldDir, filepath.FromSlash(entry.LocalPath))
	info, err := os.Stat(src)
	if err != nil || info.IsDir() {
		return "", false
	}
	// A previous run may have saved this asset under a larger --max-attachment-size.
	// If its real size now exceeds this run's limit, do not copy it: fall back to a
	// normal download so it is enforced and recorded as skipped_size, exactly like a
	// fresh run (the builder's pre-check uses Slack's file.size, which can be absent
	// or understated).
	if limit := a.limitFor(kind); limit > 0 && info.Size() > limit {
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
	// Record under the requested kind: each source_url maps to exactly one kind,
	// so this matches both the copied file's directory and what a fresh download
	// would record, keeping the reused manifest identical to a normal run.
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

const sniffLen = 512 // http.DetectContentType looks at no more than this many bytes.

// headBuffer keeps the first sniffLen bytes written through it so the download's
// real format can be detected from the bytes themselves. It sits in Save's
// MultiWriter next to the temp file and the hash, so nothing is downloaded or
// read twice.
type headBuffer struct{ buf []byte }

func (b *headBuffer) Write(p []byte) (int, error) {
	if room := sniffLen - len(b.buf); room > 0 {
		b.buf = append(b.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

// detect returns the media type sniffed from the head of the download, without
// parameters (e.g. "image/png"), or "" when nothing was downloaded.
func (b *headBuffer) detect() string {
	if len(b.buf) == 0 {
		return ""
	}
	mt, _, err := mime.ParseMediaType(http.DetectContentType(b.buf))
	if err != nil {
		return ""
	}
	return mt
}

// sniffedExtensions lists the formats http.DetectContentType recognises by magic
// bytes, and the extension each one is saved with. A sniff result outside this
// table (text/plain, application/octet-stream, ...) means the bytes told us
// nothing, so extensionFor falls back to the name- and URL-based order.
var sniffedExtensions = map[string]string{
	"image/png":                ".png",
	"image/jpeg":               ".jpg",
	"image/gif":                ".gif",
	"image/webp":               ".webp",
	"image/bmp":                ".bmp",
	"image/x-icon":             ".ico",
	"image/vnd.microsoft.icon": ".ico",
	"application/pdf":          ".pdf",
}

// contentTypeExtensions maps a declared Content-Type to an extension. It is the
// last step before .bin and covers formats the sniff cannot identify, such as
// SVG, which is XML and has no magic bytes.
var contentTypeExtensions = map[string]string{
	"image/jpeg":               ".jpg",
	"image/png":                ".png",
	"image/gif":                ".gif",
	"image/webp":               ".webp",
	"image/bmp":                ".bmp",
	"image/x-icon":             ".ico",
	"image/vnd.microsoft.icon": ".ico",
	"image/svg+xml":            ".svg",
	"application/pdf":          ".pdf",
}

// extensionFor picks the extension of the saved asset file. The downloaded bytes
// win over whatever the URL or the server claims: a gravatar avatar is served
// from a path ending in .jpg but redirects to a PNG, which used to leave the file
// name, the manifest mimetype and the file contents disagreeing (Issue #183).
// When the sniff is inconclusive the original order applies: the display file
// name Slack gave us, then the URL path, then the Content-Type.
func extensionFor(meta AssetMeta, srcURL, contentType, sniffed string) string {
	if ext, ok := sniffedExtensions[sniffed]; ok {
		return ext
	}
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
		if ext, ok := contentTypeExtensions[mt]; ok {
			return ext
		}
	}
	return ".bin"
}

// mimetypeFor decides the manifest mimetype of an asset that carries no Slack
// file metadata. It follows the same judgement as extensionFor — sniffed bytes
// first, the response Content-Type otherwise — so the recorded mimetype and the
// saved file's extension never contradict each other (Issue #183).
func mimetypeFor(contentType, sniffed string) string {
	if _, ok := sniffedExtensions[sniffed]; ok {
		return sniffed
	}
	return contentType
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
