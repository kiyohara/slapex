package export

// Integration cases for the asset plan (Issue #274). The Assets stage renders
// the timeline twice: the first render only plans the assets the page asks
// for, the plan is fetched, and the second render records each asset from what
// the fetch acquired (renderWithAssets in page.go). These cases hand Run an
// observer through its context (assetPlanObserverKey) and check, on a scenario
// that asks for assets in every way the page does, that the plan is what the
// second render asks for:
//
//   - the manifest lists the plan's assets, in the plan's order;
//   - nothing was downloaded or copied into the output directory before the
//     plan was fixed;
//   - every asset the plan acquires was downloaded, in parallel and so in any
//     order (Issue #275), and the warnings came in the plan's order, each
//     after its own download's retry notices.

import (
	"context"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

func TestRunIntegrationAssetPlanMatchesRender(t *testing.T) {
	t.Parallel()

	opts := allAssetPathsOptions(t)
	obs := &planObservation{}
	got, _, err := runExportScenarioContext(t, obs.context(t, opts.OutputDir), allAssetPathsScenario(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v\nlogs:\n%s", err, strings.Join(got.Logs, "\n"))
	}

	assertPlan(t, obs)
	assertManifestFollowsPlan(t, readManifestEntries(t, got.OutputDir), obs.plan)

	// Each asset the plan acquires was downloaded, and none the plan skips.
	if downloads := requestedPaths(got.Server.AssetRequests()); !slices.Equal(downloads, slices.Sorted(slices.Values(downloadPaths(t, obs.plan)))) {
		t.Fatalf("downloads = %q, want the plan's %q", downloads, downloadPaths(t, obs.plan))
	}
	// Each warning came in the plan's order, whatever order the downloads
	// ended in: the skip of the diagram's original before the report's
	// retries, the report's failure after them.
	assertAssetNotices(t, got.Logs,
		"WARN: asset skipped by size limit (upload_original): download exceeds size limit",
		retryNotice, retryNotice, retryNotice, retryNotice, retryNotice,
		"WARN: asset failed (attachment): giving up after 5 retries: server error: HTTP 500")
	assertAssetsPhaseLine(t, got.Logs, "WARN: assets: 14 saved, 3 skipped by size limit, 1 failed")
}

// TestRunIntegrationAssetPlanWithReuseCache runs allAssetPathsScenario again
// with --reuse-cache: the cache changes how the assets are acquired, not which
// ones the page asks for, so run 2 plans the same assets, and copies the ones
// the cache has and downloads the others.
func TestRunIntegrationAssetPlanWithReuseCache(t *testing.T) {
	t.Parallel()

	opts2 := allAssetPathsOptions(t)
	obs := &planObservation{}
	r := runReuseScenarioContext(t, obs.context(t, opts2.OutputDir), allAssetPathsScenario(),
		allAssetPathsOptions(t), opts2, nil, func(_, cacheDir string) string { return cacheDir })

	assertPlan(t, obs)
	assertManifestFollowsPlan(t, readManifestEntries(t, r.dir2), obs.plan)

	// Run 1 saved every asset but the two it could not, which run 2 downloads
	// again; it copies the rest from run 1.
	want := []string{"/files/diagram-original.png", "/files/missing-report.pdf"}
	if downloads := requestedPaths(r.requests2); !slices.Equal(downloads, want) {
		t.Fatalf("run 2 downloads = %q, want %q", downloads, want)
	}
	assertAssetsPhaseLine(t, r.logs2,
		"WARN: assets: 14 saved, 3 skipped by size limit, 1 failed (14 reused from cache, no download)")
	assertAssetsIdentical(t, r.dir1, r.dir2)
}

// --- fixture -------------------------------------------------------------------

// allAssetPathsScenario is happyPathScenario plus the ways of asking for an
// asset it lacks: an app icon from bots.info (B001) and one from an inline
// bot_profile (B002), an attachment and an image original the size pre-check
// keeps out, an image original whose download hits the size limit (Slack gave
// no file.size), an attachment whose download fails for good, a custom emoji
// only a reaction shows, and runbook.pdf and :party_sloth: asked for again.
// Every file is served, the ones the plan skips included, so a request for one
// would show in the server's requests. Run it with allAssetPathsOptions, whose
// limit keeps the large files out.
func allAssetPathsScenario() exportScenario {
	sc := happyPathScenario()
	sc.Bots = map[string]slack.Bot{
		"B001": {ID: "B001", Name: "Deploy Bot", AppID: "A001", Icons: botIcons("{{base}}/files/bot-b001.png")},
	}
	sc.Messages = append([]slack.Message{
		{
			Type: "message", TS: "1700000006.000000", User: "U01", Text: "Files for the review",
			Files: []slack.File{
				{ID: "F-ZIP", Name: "bundle.zip", Mimetype: "application/zip", Size: 5000,
					URLPrivateDownload: "{{base}}/files/bundle.zip"},
				{ID: "F-POSTER", Name: "poster.png", Mimetype: "image/png", Size: 5000,
					URLPrivateDownload: "{{base}}/files/poster-original.png", Thumb360: "{{base}}/files/poster-thumb.png"},
				{ID: "F-DIAGRAM", Name: "diagram.png", Mimetype: "image/png", // no file.size
					URLPrivateDownload: "{{base}}/files/diagram-original.png", Thumb360: "{{base}}/files/diagram-thumb.png"},
				{ID: "F-FAIL", Name: "missing-report.pdf", Mimetype: "application/pdf", Size: 50,
					URLPrivateDownload: "{{base}}/files/missing-report.pdf"},
				{ID: "F-DOC", Name: "runbook.pdf", Mimetype: "application/pdf", Size: 18,
					URLPrivateDownload: "{{base}}/files/runbook.pdf"},
			},
			Reactions: []slack.Reaction{{Name: "shipit", Count: 1}},
		},
		{Type: "message", Subtype: "bot_message", TS: "1700000005.000000", BotID: "B002",
			BotProfile: botProfileFull("Alert Bot", "{{base}}/files/bot-b002.png"), Text: "Alert resolved"},
		{Type: "message", Subtype: "bot_message", TS: "1700000004.000000", BotID: "B001",
			Text: "Deployment finished :party_sloth:"},
	}, sc.Messages...)
	sc.Emoji["shipit"] = "{{base}}/files/emoji-shipit.png"
	sc.Assets["/files/emoji-shipit.png"] = pngAsset("shipit")
	sc.Assets["/files/bot-b001.png"] = pngAsset("bot-b001")
	sc.Assets["/files/bot-b002.png"] = pngAsset("bot-b002")
	sc.Assets["/files/bundle.zip"] = fakeAsset{ContentType: "application/zip", Body: "bundle"}
	sc.Assets["/files/poster-original.png"] = pngAsset("poster original")
	sc.Assets["/files/poster-thumb.png"] = pngAsset("poster thumb")
	sc.Assets["/files/diagram-original.png"] = pngAsset(strings.Repeat("d", 2000))
	sc.Assets["/files/diagram-thumb.png"] = pngAsset("diagram thumb")
	sc.AssetFaults = map[string]*endpointFault{
		"/files/missing-report.pdf": {sticky: &faultResponse{httpStatus: http.StatusInternalServerError}},
	}
	return sc
}

// allAssetPathsOptions are the Options for allAssetPathsScenario: an attachment
// limit of 1000 bytes, under the large files' 5000 and the diagram original's
// 2000, and a clock one hour after the newest message, pinned here because the
// --reuse-cache harness calls Run directly.
func allAssetPathsOptions(t *testing.T) Options {
	t.Helper()
	opts := renderingOptions(t)
	opts.MaxAttachBytes = 1000
	opts.Now = time.Unix(1700000006, 0).Add(time.Hour)
	return opts
}

// plannedPath is one plan entry as the fake server sees it: the kind, the
// source URL's path and whether the size pre-check kept it out.
type plannedPath struct {
	kind     string
	path     string
	skipSize bool
}

// allAssetPathsPlan is the plan of allAssetPathsScenario: the workspace icon,
// the users' avatars and the bots.info app icons in ID order, then each
// message's assets in timeline order, each source URL once.
func allAssetPathsPlan() []plannedPath {
	return []plannedPath{
		{output.KindWorkspaceIcon, "/files/workspace-icon.png", false},
		{output.KindAvatar, "/files/avatar-u01.png", false},
		{output.KindAvatar, "/files/avatar-u02.png", false},
		{output.KindAvatar, "/files/bot-b001.png", false},
		{output.KindAttachment, "/files/runbook.pdf", false},
		{output.KindEmoji, "/files/emoji-party-sloth.png", false},
		{output.KindServiceIcon, "/files/service-example-news.png", false},
		{output.KindOGImage, "/files/og-launch.png", false},
		{output.KindUploadThumb, "/files/screenshot-thumb.png", false},
		{output.KindUploadOriginal, "/files/screenshot-original.png", false},
		{output.KindAvatar, "/files/bot-b002.png", false},
		{output.KindAttachment, "/files/bundle.zip", true},
		{output.KindUploadThumb, "/files/poster-thumb.png", false},
		{output.KindUploadOriginal, "/files/poster-original.png", true},
		{output.KindUploadThumb, "/files/diagram-thumb.png", false},
		{output.KindUploadOriginal, "/files/diagram-original.png", false},
		{output.KindAttachment, "/files/missing-report.pdf", false},
		{output.KindEmoji, "/files/emoji-shipit.png", false},
	}
}

// --- observer and assertions -----------------------------------------------------

// planObservation is what the plan observer saw: the plan renderWithAssets
// fixed, and the files under the output root at that moment.
type planObservation struct {
	plan  []output.PlannedAsset
	files []string
}

// context returns a context carrying the observer, which records the plan and
// every regular file under root when renderWithAssets hands the plan over.
func (o *planObservation) context(t *testing.T, root string) context.Context {
	t.Helper()
	observe := func(plan []output.PlannedAsset) {
		o.plan = plan
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type().IsRegular() {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				o.files = append(o.files, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Errorf("walk %s: %v", root, err)
		}
	}
	return context.WithValue(context.Background(), assetPlanObserverKey{}, observe)
}

// assertPlan checks that the observer saw allAssetPathsPlan, and nothing under
// the output root yet: the plan was fixed before any asset was downloaded or
// copied.
func assertPlan(t *testing.T, obs *planObservation) {
	t.Helper()
	if obs.plan == nil {
		t.Fatalf("the plan observer was not called")
	}
	var got []plannedPath
	for _, p := range obs.plan {
		got = append(got, plannedPath{p.Kind, sourcePath(t, p.SourceURL), p.SkipSize})
	}
	if want := allAssetPathsPlan(); !slices.Equal(got, want) {
		t.Fatalf("plan = %v\nwant   %v", got, want)
	}
	if len(obs.files) != 0 {
		t.Fatalf("files under the output root when the plan was fixed = %q, want none", obs.files)
	}
}

// assertManifestFollowsPlan checks that the manifest holds the plan's assets in
// the plan's order, with the kind and the file each was planned with, and each
// asset the plan skips recorded as skipped by size with no file.
func assertManifestFollowsPlan(t *testing.T, manifest []manifestEntryFull, plan []output.PlannedAsset) {
	t.Helper()
	if len(manifest) != len(plan) {
		t.Fatalf("manifest has %d entries, want the plan's %d\nmanifest: %+v", len(manifest), len(plan), manifest)
	}
	for i, p := range plan {
		e := manifest[i]
		if e.SourceURL != p.SourceURL || e.Kind != p.Kind || e.FileID != p.Meta.FileID {
			t.Fatalf("manifest[%d] = %s %s (file %q), want the plan's %s %s (file %q)",
				i, e.Kind, e.SourceURL, e.FileID, p.Kind, p.SourceURL, p.Meta.FileID)
		}
		if p.SkipSize && (e.Status != output.StatusSkippedSize || e.LocalPath != "") {
			t.Fatalf("manifest[%d] = %+v, want %s with no local_path", i, e, output.StatusSkippedSize)
		}
	}
}

// downloadPaths are the source URL paths of the plan's assets that are
// acquired, in the plan's order: every one the size pre-check did not keep
// out.
func downloadPaths(t *testing.T, plan []output.PlannedAsset) []string {
	t.Helper()
	var paths []string
	for _, p := range plan {
		if !p.SkipSize {
			paths = append(paths, sourcePath(t, p.SourceURL))
		}
	}
	return paths
}

// requestedPaths is each path of requests once, sorted.
func requestedPaths(requests []string) []string {
	return slices.Compact(slices.Sorted(slices.Values(requests)))
}

func sourcePath(t *testing.T, sourceURL string) string {
	t.Helper()
	u, err := url.Parse(sourceURL)
	if err != nil {
		t.Fatalf("parse %q: %v", sourceURL, err)
	}
	return u.Path
}

// retryNotice stands for any download retry notice in assertAssetNotices.
const retryNotice = "(retry notice)"

// assertAssetNotices checks the asset warnings and the download retry notices,
// in the order they were printed. A retryNotice in want matches one retry
// notice, whatever its wait and cause.
func assertAssetNotices(t *testing.T, logs []string, want ...string) {
	t.Helper()
	var got []string
	for _, line := range logs {
		switch {
		case strings.HasPrefix(line, "WARN: asset "):
			got = append(got, line)
		case strings.HasPrefix(line, "INFO: assets: retrying download in "):
			got = append(got, retryNotice)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("asset notices = %q\nwant %q\nlogs:\n%s", got, want, strings.Join(logs, "\n"))
	}
}
