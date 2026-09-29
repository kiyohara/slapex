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
	"sync"
	"time"

	"github.com/kiyohara/slapex/internal/lane"
	"github.com/kiyohara/slapex/internal/slack"
)

// Downloader is the asset fetch dependency (implemented by *slack.Client).
// Fetch calls Download on several goroutines at once.
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

// Status values stored in the manifest (doc/design/cache.md).
const (
	StatusSaved       = "saved"
	StatusSkippedSize = "skipped_size"
	StatusFailed      = "failed"
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
//
// The export asks for its assets while it renders the page, and renders twice
// (Issue #274): a Planner records what the first render asks for, Fetch
// acquires that plan, downloading in parallel (Issue #275), and the second
// render's Save and SkipTooLarge record the results in the order it asks. A
// Save for a URL no Fetch acquired — outside the plan, or with no plan at all
// — acquires it then.
type Assets struct {
	ctx     context.Context
	dl      Downloader
	dir     string
	limit   int64 // per-file byte limit, 0 = unlimited
	known   map[string]string
	status  map[string]string // manifest status last recorded per source URL
	entries []ManifestEntry
	reuse   *ReuseSource // previous run's assets to copy instead of downloading
	reused  int          // assets taken from the reuse source instead of downloaded
	// planning marks a planner (Planner): Save and SkipTooLarge add each new
	// source URL to plan instead of acquiring or recording it.
	planning bool
	plan     []PlannedAsset
	// fetched holds what Fetch acquired, by source URL, for Save to record when
	// the render asks for it.
	fetched map[string]ManifestEntry
	// Lanes bounds the downloads Fetch runs at a time (internal/lane).
	Lanes lane.Limits
	// Logf receives a warning for each download that did not complete, worded
	// after its manifest status: a failure, or a download the size limit
	// stopped. SkipTooLarge warns of nothing: a file its pre-check keeps out
	// is the limit working as configured, and the Assets counts report it
	// (doc/design/output-format.md). The warning comes when the download ends;
	// after a Fetch, once the downloads before it in the plan have ended too,
	// so it follows the plan's order, which is the order the render asks in.
	Logf func(format string, args ...any)
	// Notef receives the progress notices of the downloads Fetch runs — the
	// Downloader's retries and waits, which *slack.Client reports through
	// slack.WithNotices — each download's before its warning, in the plan's
	// order like the warnings. The downloads Save runs report theirs as the
	// Downloader does by default.
	Notef func(format string, args ...any)
}

// PlannedAsset is one asset a planning render asked for (Assets.Planner): the
// first request for its source URL, with that request's kind and metadata and
// the per-file byte limit the kind gets (0 = unlimited). SkipSize marks a file
// the size pre-check kept out (SkipTooLarge), which Fetch leaves alone.
type PlannedAsset struct {
	Kind      string
	SourceURL string
	Limit     int64
	Meta      AssetMeta
	SkipSize  bool
}

// ReuseSource lets Fetch and Save copy an already-saved asset from a previous
// run's output instead of downloading it again, for --reuse-cache
// (doc/design/cache.md, decision log 0030). OldDir is the previous run's
// channel directory (the parent of the reused .cache/); Entries maps each
// previously saved source_url to its manifest entry, whose LocalPath is
// relative to OldDir.
type ReuseSource struct {
	OldDir  string
	Entries map[string]ManifestEntry
}

func NewAssets(ctx context.Context, dl Downloader, dir string, limit int64) *Assets {
	return &Assets{
		ctx:    ctx,
		dl:     dl,
		dir:    dir,
		limit:  limit,
		known:  map[string]string{},
		status: map[string]string{},
		Lanes:  lane.Defaults,
		Logf:   func(string, ...any) {},
		Notef:  func(string, ...any) {},
	}
}

// SetReuseSource enables copy-from-previous-run behaviour in Fetch and Save
// (--reuse-cache).
func (a *Assets) SetReuseSource(r *ReuseSource) { a.reuse = r }

// Reused returns how many assets came from the reuse source instead of being
// downloaded. An asset that already sits at its destination — the reuse source
// is this run's own output directory — counts too, even though copyFromReuse
// copied nothing.
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

// Planner returns an Assets for a render that only plans (Issue #274). Its
// Save and SkipTooLarge take each request as a's would — an empty URL is
// ignored, a source URL counts once, at its first request, and the kind of
// that request sets the size limit — and add it to Plan instead of acquiring
// it. A planner writes nothing to the output directory, copies nothing from
// the reuse source and warns of nothing. Its Save reports every asset
// unavailable, and its Status reports skipped_size only for a file
// SkipTooLarge planned. Which assets a render asks for does not depend on
// those answers, so a planning render asks for what the render after Fetch
// will.
func (a *Assets) Planner() *Assets {
	return &Assets{
		limit:    a.limit,
		known:    map[string]string{},
		status:   map[string]string{},
		planning: true,
		Logf:     func(string, ...any) {},
	}
}

// Plan returns what a planner was asked for: one PlannedAsset per source URL,
// in the order first requested.
func (a *Assets) Plan() []PlannedAsset { return a.plan }

// Fetch acquires the planned assets, each the way Save would at the first
// request for it: a copy from the reuse source when that has the asset, a
// download otherwise, with the same warning for a download that did not
// complete. The copies come first, in plan order. The downloads then run in
// parallel, in one lane per origin within a.Lanes (internal/lane, Issue
// #275), and each download's notices and warning are held until the
// downloads before it in the plan have passed theirs on, so they come in the
// order a serial fetch in plan order gives them (heldNotices). Fetch records
// nothing in the manifest: Save records each result when the render asks for
// it, so the manifest follows the order of the render's requests, whatever
// order the downloads end in. A file SkipTooLarge planned has nothing to
// fetch.
//
// Once a's context is done, Fetch starts no more copies or downloads, and
// returns when the downloads under way have stopped, each having removed its
// temporary file. The assets it did not get stay unfetched, and the notices
// not passed on by then are dropped.
func (a *Assets) Fetch(plan []PlannedAsset) {
	if a.fetched == nil {
		a.fetched = map[string]ManifestEntry{}
	}
	var downloads []PlannedAsset
	queued := map[string]bool{}
	for _, p := range plan {
		if a.ctx.Err() != nil {
			break
		}
		if p.SkipSize || p.SourceURL == "" || queued[p.SourceURL] {
			continue
		}
		if _, done := a.fetched[p.SourceURL]; done {
			continue
		}
		if _, seen := a.known[p.SourceURL]; seen {
			continue
		}
		if a.reuse != nil {
			if e, ok := a.copyFromReuse(p.Kind, p.SourceURL, p.Limit, p.Meta); ok {
				a.fetched[p.SourceURL] = e
				continue
			}
		}
		queued[p.SourceURL] = true
		downloads = append(downloads, p)
	}

	jobs := make([]lane.Job, len(downloads))
	for i, p := range downloads {
		jobs[i] = lane.Job{URL: p.SourceURL, Size: p.Meta.SizeBytes}
	}
	results := make([]*ManifestEntry, len(downloads))
	held := newHeldNotices(a, len(downloads))
	lane.Run(a.ctx, jobs, a.Lanes, func(ctx context.Context, i int) {
		p := downloads[i]
		e := a.download(slack.WithNotices(ctx, held.notef(i)), p.Kind, p.SourceURL, p.Limit, p.Meta, held.warnf(i))
		results[i] = &e
		held.done(i)
	})
	for i, e := range results {
		if e != nil {
			a.fetched[downloads[i].SourceURL] = *e
		}
	}
}

// heldNotices holds what the downloads of one Fetch report — the notices
// through Notef and the warnings through Logf — and passes each download's on
// once the downloads before it in the plan have passed theirs on. Each
// download adds to its own lines only, before done.
type heldNotices struct {
	a *Assets

	mu    sync.Mutex
	lines [][]heldLine // by download
	ended []bool
	next  int // the first download whose lines are not passed on
}

type heldLine struct {
	warning bool
	format  string
	args    []any
}

func newHeldNotices(a *Assets, n int) *heldNotices {
	return &heldNotices{a: a, lines: make([][]heldLine, n), ended: make([]bool, n)}
}

func (h *heldNotices) notef(i int) func(string, ...any) {
	return func(format string, args ...any) {
		h.lines[i] = append(h.lines[i], heldLine{format: format, args: args})
	}
}

func (h *heldNotices) warnf(i int) func(string, ...any) {
	return func(format string, args ...any) {
		h.lines[i] = append(h.lines[i], heldLine{warning: true, format: format, args: args})
	}
}

// done marks download i ended, and passes on the lines of the downloads that
// have nothing before them in the plan left to wait for. Once the context is
// done, it drops them instead: the run is stopping.
func (h *heldNotices) done(i int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ended[i] = true
	for ; h.next < len(h.ended) && h.ended[h.next]; h.next++ {
		if h.a.ctx.Err() == nil {
			for _, l := range h.lines[h.next] {
				if l.warning {
					h.a.Logf(l.format, l.args...)
				} else {
					h.a.Notef(l.format, l.args...)
				}
			}
		}
		h.lines[h.next] = nil
	}
}

// SkipTooLarge records a file that was not downloaded due to the size limit.
// Like Save, it records each source URL once, so a file shown again — a
// thread_broadcast rendered on the timeline and in its thread, or one file in
// several posts — is counted once (Issue #249). A later Save of the URL, where
// Slack gave no size or a smaller one, reports it unavailable rather than
// downloading and recording it again. Like Save, it records nothing for an
// empty srcURL: a file with no URL has nothing to download, so nothing for the
// size limit to keep out (Issue #247).
func (a *Assets) SkipTooLarge(kind, srcURL string, meta AssetMeta) {
	if srcURL == "" {
		return
	}
	if _, seen := a.status[srcURL]; seen {
		return
	}
	if a.planning {
		a.addToPlan(kind, srcURL, meta, true)
		return
	}
	a.record(ManifestEntry{
		Kind: kind, SourceURL: srcURL, Status: StatusSkippedSize,
		FileID: meta.FileID, OriginalName: meta.OriginalName,
		Mimetype: meta.Mimetype, SizeBytes: meta.SizeBytes,
	})
}

// addToPlan records a planner's first request for srcURL. skipSize marks one
// from SkipTooLarge, which Status then reports as skipped_size.
func (a *Assets) addToPlan(kind, srcURL string, meta AssetMeta, skipSize bool) {
	a.known[srcURL] = ""
	a.status[srcURL] = ""
	if skipSize {
		a.status[srcURL] = StatusSkippedSize
	}
	a.plan = append(a.plan, PlannedAsset{
		Kind: kind, SourceURL: srcURL, Limit: a.limitFor(kind), Meta: meta, SkipSize: skipSize,
	})
}

// Status returns the manifest status last recorded for srcURL (StatusSaved,
// StatusSkippedSize or StatusFailed), or "" when nothing was recorded for it.
// Save reports only ok, so a caller that got ok == false asks Status whether
// the size limit kept the asset out or the download really failed (Issue
// #203). It is looked up by URL rather than read off the last manifest entry,
// because a repeated Save of a known URL records no new entry.
func (a *Assets) Status(srcURL string) string { return a.status[srcURL] }

// Save records srcURL (unless an earlier Save or SkipTooLarge already handled
// it) and returns the path relative to the output directory. ok is false when
// the asset is unavailable. It records what Fetch acquired for srcURL, and
// acquires the asset itself when no Fetch did. On a planner it only adds
// srcURL to the plan (Planner).
func (a *Assets) Save(kind, srcURL string, meta AssetMeta) (relPath string, ok bool) {
	if srcURL == "" {
		return "", false
	}
	if rel, seen := a.known[srcURL]; seen {
		return rel, rel != ""
	}
	if a.planning {
		a.addToPlan(kind, srcURL, meta, false)
		return "", false
	}
	e, fetched := a.fetched[srcURL]
	if !fetched {
		e = a.acquire(kind, srcURL, a.limitFor(kind), meta)
	}
	a.record(e)
	return e.LocalPath, e.Status == StatusSaved
}

// acquire gets srcURL the way Save does at its first request: a copy from the
// reuse source when that has the asset, a download otherwise. limit is the
// per-file byte limit for kind. It returns the manifest entry for Save to
// record.
func (a *Assets) acquire(kind, srcURL string, limit int64, meta AssetMeta) ManifestEntry {
	if a.reuse != nil {
		if e, ok := a.copyFromReuse(kind, srcURL, limit, meta); ok {
			return e
		}
	}
	return a.download(a.ctx, kind, srcURL, limit, meta, a.Logf)
}

// download fetches srcURL with ctx into kind's directory under its content
// hash and returns the manifest entry, warning of a download that did not
// complete through warnf. Fetch runs it on several goroutines at once: it
// changes nothing in a, and its temporary file has a name of its own.
func (a *Assets) download(ctx context.Context, kind, srcURL string, limit int64, meta AssetMeta, warnf func(string, ...any)) ManifestEntry {
	tmp, err := os.CreateTemp(a.dir, "asset-*")
	if err != nil {
		return newEntry(kind, srcURL, meta, "", StatusFailed, err.Error())
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
	// The kind labels the download in the HTTP trace (Issue #273).
	ctx = slack.WithAssetKind(ctx, kind)
	size, contentType, err := a.dl.Download(ctx, srcURL, limit, io.MultiWriter(tmp, h, &head))
	tmp.Close()
	if err != nil {
		status, warning := StatusFailed, "asset failed"
		if errors.Is(err, slack.ErrTooLarge) {
			// A download the size limit stopped is a size skip, not a failure,
			// in the warning as in the manifest and the counts (Issue #250).
			status, warning = StatusSkippedSize, "asset skipped by size limit"
		}
		warnf("%s (%s): %s", warning, kind, err)
		return newEntry(kind, srcURL, meta, "", status, err.Error())
	}

	sniffed := head.detect()
	base := hex.EncodeToString(h.Sum(nil))
	rel := filepath.Join(kindDirs[kind], base+extensionFor(meta, srcURL, contentType, sniffed))
	dst := filepath.Join(a.dir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return newEntry(kind, srcURL, meta, "", StatusFailed, err.Error())
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return newEntry(kind, srcURL, meta, "", StatusFailed, err.Error())
	}
	if meta.SizeBytes == 0 {
		meta.SizeBytes = size
	}
	if meta.Mimetype == "" {
		meta.Mimetype = mimetypeFor(contentType, sniffed)
	}
	return newEntry(kind, srcURL, meta, filepath.ToSlash(rel), StatusSaved, "")
}

// newEntry is the manifest entry for srcURL with the given outcome.
func newEntry(kind, srcURL string, meta AssetMeta, rel, status, errMsg string) ManifestEntry {
	return ManifestEntry{
		Kind: kind, SourceURL: srcURL, LocalPath: rel,
		FileID: meta.FileID, EmojiName: meta.EmojiName, OriginalName: meta.OriginalName,
		Mimetype: meta.Mimetype, SizeBytes: meta.SizeBytes,
		Status: status, Error: errMsg,
	}
}

// record adds e to the manifest, and makes its source URL known to later
// requests and to Status.
func (a *Assets) record(e ManifestEntry) {
	a.known[e.SourceURL] = e.LocalPath
	a.status[e.SourceURL] = e.Status
	a.entries = append(a.entries, e)
}

// copyFromReuse copies an asset that a previous run already saved into this
// run's output instead of downloading it, when --reuse-cache matched (decision
// log 0030). It reuses the old LocalPath verbatim so the previous run's file
// name (and therefore the HTML references) stays identical across runs. Because
// the asset content is unchanged, a fresh download would resolve to the same
// content hash anyway (decision log 0052); reusing a cache written by an older
// URL-hash build simply keeps that build's names for the copied assets. It
// returns the manifest entry to record, or false — so acquire falls back to a
// normal download — when srcURL was not a saved asset before or the previous
// file is gone. limit is the per-file byte limit for kind.
func (a *Assets) copyFromReuse(kind, srcURL string, limit int64, meta AssetMeta) (ManifestEntry, bool) {
	entry, ok := a.reuse.Entries[srcURL]
	if !ok || entry.LocalPath == "" {
		return ManifestEntry{}, false
	}
	// LocalPath comes from a previous run's manifest. Reject anything that is not
	// a contained relative path so a corrupted or untrusted cache cannot read or
	// write outside the old / new output directories (path traversal); such an
	// asset falls back to a normal download.
	if !filepath.IsLocal(filepath.FromSlash(entry.LocalPath)) {
		return ManifestEntry{}, false
	}
	src := filepath.Join(a.reuse.OldDir, filepath.FromSlash(entry.LocalPath))
	info, err := os.Stat(src)
	if err != nil || info.IsDir() {
		return ManifestEntry{}, false
	}
	// A previous run may have saved this asset under a larger --max-attachment-size.
	// If its real size now exceeds this run's limit, do not copy it: fall back to a
	// normal download so it is enforced and recorded as skipped_size, exactly like a
	// fresh run (the export messageViewBuilder pre-check uses Slack's file.size, which
	// can be absent or understated).
	if limit > 0 && info.Size() > limit {
		return ManifestEntry{}, false
	}
	dst := filepath.Join(a.dir, filepath.FromSlash(entry.LocalPath))
	// --reuse-cache may point at the directory this run is writing to: re-running
	// with the same --output and channel resolves OldDir and a.dir to one
	// directory, so src and dst are the same file. Copying it onto itself would
	// truncate it to 0 bytes and destroy both runs' asset, so leave the file
	// alone and count it as reused — its bytes are already what a copy would
	// have produced (Issue #202).
	if !sameFile(info, dst) {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return ManifestEntry{}, false
		}
		if err := copyFile(src, dst); err != nil {
			return ManifestEntry{}, false
		}
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
	return newEntry(kind, srcURL, meta, entry.LocalPath, StatusSaved, ""), true
}

// sameFile reports whether dst is the same existing file as the one srcInfo
// describes, following the identity os.SameFile defines (hard links and
// different spellings of one path included) rather than comparing path strings.
func sameFile(srcInfo os.FileInfo, dst string) bool {
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return false
	}
	return os.SameFile(srcInfo, dstInfo)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	// Never copy a file onto itself: os.Create would truncate the source that is
	// open for reading and io.Copy would then write nothing, leaving 0 bytes. The
	// destination already holds the intended content, so this is a no-op success.
	// The check uses the open handle's own metadata, so a path that starts
	// resolving elsewhere between the open and the check cannot slip past it.
	if srcInfo, err := in.Stat(); err == nil && sameFile(srcInfo, dst) {
		return nil
	}
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
		case StatusSaved:
			saved++
		case StatusSkippedSize:
			skipped++
		default:
			failed++
		}
	}
	return
}

const sniffLen = 512 // http.DetectContentType looks at no more than this many bytes.

// headBuffer keeps the first sniffLen bytes written through it so the download's
// real format can be detected from the bytes themselves. It sits in download's
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
// file metadata: the sniffed type when the bytes identified themselves, the
// response Content-Type otherwise. A sniffed asset therefore gets its mimetype
// and its extension from the one judgement, which is what the gravatar case
// needed (Issue #183). An asset the sniff could not identify still takes its
// extension from the display name or the URL, so the two can differ there.
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
