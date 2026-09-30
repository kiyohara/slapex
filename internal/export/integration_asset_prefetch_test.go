package export

// Integration cases of the asset downloads' prefetch (Issue #279, PF-07 of
// #272; decision log 0069). From the Messages phase on, the downloads of the
// assets the page is certain to show go ahead of the Assets phase, which
// takes what they got. The export must still be the serial export (the
// prefetch off), as for the Web API requests (integration_prefetch_test.go):
// when it succeeds, the same files, the same log once normalized — the asset
// warnings and the download notices in the same order, in the Assets phase —
// and the same requests of each asset, every one of which went ahead of the
// Assets phase (runAhead). The one exception is a URL that the page asks for
// under two size limits, whose download sent ahead stopped at the smaller
// one: the Assets phase downloads it again, one request more. A run that
// fails or is canceled while downloads are under way ahead stops as the
// serial run does, with the same log and files — no temporary file left —
// and has sent only requests that the run without the failure makes too.

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// arrivals counts the asset paths the fake server has been asked for, for a
// case that waits for downloads to come, and passes each request on to next,
// the scenario's own BeforeAsset.
type arrivals struct {
	next func(*http.Request)

	mu      sync.Mutex
	come    map[string]int
	changed chan struct{} // closed at the next arrival
}

func newArrivals(next func(*http.Request)) *arrivals {
	return &arrivals{next: next, come: map[string]int{}, changed: make(chan struct{})}
}

// record is the fake server's BeforeAsset.
func (a *arrivals) record(r *http.Request) {
	a.mu.Lock()
	a.come[r.URL.Path]++
	close(a.changed)
	a.changed = make(chan struct{})
	a.mu.Unlock()
	if a.next != nil {
		a.next(r)
	}
}

// reset forgets what has come.
func (a *arrivals) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	clear(a.come)
}

// wait waits until each path of want has come as many times as want gives,
// and returns, sorted, those that have not in 10s.
func (a *arrivals) wait(want map[string]int) []string {
	timeout := time.After(10 * time.Second)
	for {
		a.mu.Lock()
		var missing []string
		for path, n := range want {
			if a.come[path] < n {
				missing = append(missing, path)
			}
		}
		changed := a.changed
		a.mu.Unlock()
		if len(missing) == 0 {
			return nil
		}
		select {
		case <-changed:
		case <-timeout:
			slices.Sort(missing)
			return missing
		}
	}
}

// eachOnce asks wait for each of paths once.
func eachOnce(paths []string) map[string]int {
	want := make(map[string]int, len(paths))
	for _, p := range paths {
		want[p] = 1
	}
	return want
}

// runAhead runs sc with the prefetch on, like runWithPrefetch, and checks
// that the downloads of the asset paths in ahead went ahead of the Assets
// phase: once the phase has fixed its plan, and before it fetches it, the
// run waits until each of them has come to the server.
func runAhead(t *testing.T, sc exportScenario, opts Options, ahead []string) prefetchRun {
	t.Helper()
	come := newArrivals(sc.BeforeAsset)
	sc.BeforeAsset = come.record
	fake := newFakeSlackServer(t, &sc)
	t.Cleanup(fake.Close)
	return runAheadOn(t, fake, come, opts, ahead)
}

// runAheadOn is runAhead against fake, which may serve other runs before and
// after, and whose BeforeAsset records in come.
func runAheadOn(t *testing.T, fake *fakeSlackServer, come *arrivals, opts Options, ahead []string) prefetchRun {
	t.Helper()
	come.reset()
	observe := func([]output.PlannedAsset) {
		if missing := come.wait(eachOnce(ahead)); len(missing) > 0 {
			t.Errorf("downloads that did not go ahead of the Assets phase: %q", missing)
		}
	}
	ctx := context.WithValue(context.Background(), assetPlanObserverKey{}, observe)
	return runWithPrefetchOn(t, ctx, true, fake, opts, nil)
}

// downloadsOf is each asset path of requests once, sorted.
func downloadsOf(requests []string) []string {
	return requestedPaths(slices.DeleteFunc(slices.Clone(requests), func(r string) bool {
		return strings.HasPrefix(r, "/api/")
	}))
}

func TestRunIntegrationAssetPrefetchMatchesSerial(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		scenario     func() exportScenario
		options      func(*testing.T) Options
		repliesAhead int
		// never are the asset paths the page shows no download of: neither
		// run asks for them.
		never []string
		// check, when set, checks the serial run: that the scenario shows
		// what the case is about. The prefetched run writes the same.
		check func(t *testing.T, serial prefetchRun)
		// hold, when set, holds a request of the prefetched run's scenario.
		hold func(t *testing.T, sc *exportScenario)
	}{
		{
			name:     "sizes over the limit",
			scenario: assetSizesScenario,
			options:  smallAttachmentOptions,
			never:    []string{"/files/big-archive.zip", "/files/huge-original.png"},
			check: func(t *testing.T, serial prefetchRun) {
				assertAssetNotices(t, serial.Logs,
					"WARN: asset skipped by size limit (attachment): download exceeds size limit",
					"WARN: asset skipped by size limit (upload_original): download exceeds size limit")
			},
		},
		{
			name:     "downloads that fail or are rate limited",
			scenario: assetFaultsScenario,
			options:  renderingOptions,
			check: func(t *testing.T, serial prefetchRun) {
				var got []string
				for _, line := range normalizedLogs(serial.exportRunResult) {
					if strings.HasPrefix(line, "WARN: asset ") || strings.HasPrefix(line, "INFO: assets: ") {
						got = append(got, line)
					}
				}
				const retry = "INFO: assets: retrying download in <d> (server error: HTTP 500)"
				// The notices and warnings come in the Assets phase, in
				// plan order: the workspace icon, the avatars, then each
				// message's assets in timeline order.
				want := []string{
					"INFO: assets: downloading assets and rendering HTML ...",
					"WARN: asset failed (workspace_icon): unexpected HTTP 404",
					"INFO: assets: rate limited on download, waiting <d> as instructed by Slack",
					"WARN: asset failed (attachment): unexpected HTTP 404",
					retry, retry, retry, retry, retry,
					"WARN: asset failed (attachment): giving up after 5 retries: server error: HTTP 500",
				}
				if !slices.Equal(got, want) {
					t.Errorf("asset notices = %q\nwant %q", got, want)
				}
			},
		},
		{
			// emoji.list answers once the history page's downloads have
			// gone: the custom emoji images of the page go ahead once the
			// custom emoji are known, the alias's first.
			name:     "one URL shown several times",
			scenario: sharedURLScenario,
			options:  smallAttachmentOptions,
			hold: func(t *testing.T, sc *exportScenario) {
				holdUntilAsset(t, sc, "/api/emoji.list", "/files/preview.png")
			},
			check: func(t *testing.T, serial prefetchRun) {
				assertAssetNotices(t, serial.Logs,
					"WARN: asset skipped by size limit (attachment): download exceeds size limit")
				got := map[string]string{}
				for _, e := range readManifestWithEmoji(t, serial.OutputDir) {
					got[sourcePath(t, e.SourceURL)] = fmt.Sprintf("%s %s %s", e.Kind, e.Status, e.EmojiName)
				}
				want := map[string]string{
					"/files/preview.png":           "attachment skipped_size ",
					"/files/shared.pdf":            "attachment saved ",
					"/files/picture.png":           "upload_thumb saved ",
					"/files/picture-original.png":  "upload_original saved ",
					"/files/emoji-party-sloth.png": "emoji saved party_sloth",
				}
				if !maps.Equal(got, want) {
					t.Errorf("manifest = %q, want %q", got, want)
				}
			},
		},
		{
			name:         "emoji filter excludes messages with files",
			scenario:     filteredFilesScenario,
			options:      droppedThreadsOptions,
			repliesAhead: 2,
			never: []string{
				filteredFilePath("1700000009.000000"), // the broadcast of the thread whose parent is excluded
				filteredFilePath("1700000008.000000"), // the parent whose thread conversations.replies excludes
				filteredFilePath("1700000008.100000"),
				filteredFilePath("1700000006.000000"), // the parent excluded on the second page
				filteredFilePath("1700000006.100000"),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serial := runWithPrefetch(t, context.Background(), false, tc.scenario(), tc.options(t), nil)
			sc := tc.scenario()
			if tc.hold != nil {
				tc.hold(t, &sc)
			}
			prefetched := runAhead(t, sc, tc.options(t), downloadsOf(serial.requests))
			assertPrefetchMatchesSerial(t, serial, prefetched, tc.repliesAhead)
			if tc.check != nil {
				tc.check(t, serial)
			}
			for _, path := range tc.never {
				if slices.Contains(serial.requests, path) || slices.Contains(prefetched.requests, path) {
					t.Errorf("%s was asked for: by the serial run %t, by the prefetched run %t", path,
						slices.Contains(serial.requests, path), slices.Contains(prefetched.requests, path))
				}
			}
		})
	}
}

// TestRunIntegrationAssetPrefetchRefetch: the page asks for one URL under two
// size limits, as a file on the newer message, under --max-attachment-size,
// and as the image of a URL preview on the older one, under the preview
// guard. The download sent ahead, for the newer message, which the history
// page gives first, stops at the smaller limit; the plan, in the page's
// order, asks for the URL under the larger. The Assets phase downloads it
// again: the prefetched run makes that one request more than the serial run,
// and writes the same export and log.
func TestRunIntegrationAssetPrefetchRefetch(t *testing.T) {
	t.Parallel()

	const picture = "/files/picture.png"
	serial := runWithPrefetch(t, context.Background(), false, refetchScenario(), smallAttachmentOptions(t), nil)
	prefetched := runAhead(t, refetchScenario(), smallAttachmentOptions(t), []string{picture})
	for _, run := range []prefetchRun{serial, prefetched} {
		if run.err != nil {
			t.Fatalf("Run() error = %v\nlogs:\n%s", run.err, strings.Join(run.Logs, "\n"))
		}
	}
	assertSameExport(t, "the prefetched run", serial.exportRunResult, prefetched.exportRunResult)
	if n := countPrefix(serial.requests, picture); n != 1 {
		t.Errorf("the serial run asked for the picture %d times, want 1", n)
	}
	want := slices.Sorted(slices.Values(append(slices.Clone(serial.requests), picture)))
	if got := slices.Sorted(slices.Values(prefetched.requests)); !slices.Equal(got, want) {
		t.Errorf("requests of the prefetched run = %q\nwant the serial run's and the picture again: %q", got, want)
	}
	entry, ok := findManifest(readManifestEntries(t, serial.OutputDir), func(e manifestEntryFull) bool {
		return strings.HasSuffix(e.SourceURL, picture)
	})
	if !ok || entry.Kind != output.KindOGImage || entry.Status != output.StatusSaved {
		t.Errorf("manifest entry of the picture = %+v (ok=%t), want a saved og_image", entry, ok)
	}
}

// TestRunIntegrationAssetPrefetchFailsAsSerial: a run that fails while
// downloads are under way ahead of the Assets phase, or have ended there,
// fails as the serial run does, which has downloaded nothing: the downloads
// are stopped, their temporary files removed, and the notices of their
// retries, held for the Assets phase, dropped.
func TestRunIntegrationAssetPrefetchFailsAsSerial(t *testing.T) {
	t.Parallel()

	const (
		runbook     = "/files/runbook.pdf"
		thread      = "/api/conversations.replies?ts=1700000002.000000"
		emojiList   = "/api/emoji.list"
		serverError = "giving up after 5 retries: server error: HTTP 500"
	)
	sticky500 := func() *endpointFault {
		return &endpointFault{sticky: &faultResponse{httpStatus: http.StatusInternalServerError}}
	}
	for _, tc := range []struct {
		name string
		// scenario is the case's scenario, with its faults; for the
		// prefetched run, it holds the request that fails until the
		// runbook's download, sent ahead, has come.
		scenario func(t *testing.T, prefetched bool) exportScenario
		wantErr  string
	}{
		{
			// The thread's replies fail while the runbook's download, sent
			// ahead at the history page, is under way.
			name: "thread replies fail",
			scenario: func(t *testing.T, prefetched bool) exportScenario {
				sc := happyPathScenario()
				sc.APIFaults = map[string]*endpointFault{thread: sticky500()}
				if prefetched {
					come := newArrivals(func(r *http.Request) {
						if r.URL.Path == runbook {
							<-r.Context().Done()
						}
					})
					sc.BeforeAsset = come.record
					sc.BeforeAPI = holdUntilCome(t, thread, come, map[string]int{runbook: 1})
				}
				return sc
			},
			wantErr: "slack api conversations.replies: " + serverError,
		},
		{
			// emoji.list fails once the runbook's download, sent ahead, has
			// been retried after a server error.
			name: "emoji.list fails",
			scenario: func(t *testing.T, prefetched bool) exportScenario {
				sc := happyPathScenario()
				sc.APIFaults = map[string]*endpointFault{emojiList: sticky500()}
				sc.AssetFaults = map[string]*endpointFault{
					runbook: {transient: []faultResponse{{httpStatus: http.StatusInternalServerError}}},
				}
				if prefetched {
					come := newArrivals(nil)
					sc.BeforeAsset = come.record
					sc.BeforeAPI = holdUntilCome(t, emojiList, come, map[string]int{runbook: 2})
				}
				return sc
			},
			wantErr: "slack api emoji.list: " + serverError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serial := runWithPrefetch(t, context.Background(), false, tc.scenario(t, false), renderingOptions(t), nil)
			prefetched := runWithPrefetch(t, context.Background(), true, tc.scenario(t, true), renderingOptions(t), nil)
			complete := runWithPrefetch(t, context.Background(), false, happyPathScenario(), renderingOptions(t), nil)
			assertPrefetchFailsAsSerial(t, serial, prefetched, complete, tc.wantErr, runbook)
		})
	}
}

// holdUntilAsset holds, in sc, the first request named held until the asset
// path has come to the server.
func holdUntilAsset(t *testing.T, sc *exportScenario, held, path string) {
	come := newArrivals(sc.BeforeAsset)
	sc.BeforeAsset = come.record
	sc.BeforeAPI = holdUntilCome(t, held, come, map[string]int{path: 1})
}

// holdUntilCome returns a BeforeAPI hook that holds the first request named
// held until the asset requests of want have come to come.
func holdUntilCome(t *testing.T, held string, come *arrivals, want map[string]int) func(*http.Request) {
	var once sync.Once
	return func(r *http.Request) {
		if requestName(r) != held {
			return
		}
		once.Do(func() {
			if missing := come.wait(want); len(missing) > 0 {
				t.Errorf("%s was held, and %q did not come", held, missing)
			}
		})
	}
}

// TestRunIntegrationAssetPrefetchCanceledAsSerial: a run canceled (Ctrl-C)
// once the Emoji stage has taken emoji.list, while the runbook's download,
// sent ahead, is under way, stops where the serial run canceled there does:
// at the start of the Assets phase, which fetches nothing. The download is
// stopped, and leaves no temporary file.
func TestRunIntegrationAssetPrefetchCanceledAsSerial(t *testing.T) {
	t.Parallel()

	const runbook = "/files/runbook.pdf"
	run := func(prefetched bool) prefetchRun {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		come := newArrivals(func(r *http.Request) {
			if r.URL.Path == runbook {
				<-r.Context().Done()
			}
		})
		sc := happyPathScenario()
		sc.BeforeAsset = come.record
		return runWithPrefetch(t, ctx, prefetched, sc, renderingOptions(t), func(key requestKey) {
			if key != emojiListKey {
				return
			}
			if prefetched {
				if missing := come.wait(map[string]int{runbook: 1}); len(missing) > 0 {
					t.Errorf("the runbook's download did not go ahead")
				}
			}
			cancel()
		})
	}
	serial, prefetched := run(false), run(true)
	complete := runWithPrefetch(t, context.Background(), false, happyPathScenario(), renderingOptions(t), nil)
	assertPrefetchFailsAsSerial(t, serial, prefetched, complete, "context canceled", runbook)
}

// smallAttachmentOptions is renderingOptions with an attachment limit of 100
// bytes, under the 200 of the files the size limit keeps out at the
// download.
func smallAttachmentOptions(t *testing.T) Options {
	t.Helper()
	opts := renderingOptions(t)
	opts.MaxAttachBytes = 100
	return opts
}

// assetSizesScenario shows files that the limit of smallAttachmentOptions
// keeps out: by their size in the file object, which no run downloads, and
// at the download, which stops at the limit — a report, posted twice, and an
// image's original. The thumbnail of each image and a file within the limit
// are saved.
func assetSizesScenario() exportScenario {
	report := slack.File{ID: "F-PDF", Name: "report.pdf", Mimetype: "application/pdf", URLPrivateDownload: "{{base}}/files/report.pdf"}
	sc := baseScenario()
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000004.000000", User: "U02", Text: "Same report again", Files: []slack.File{report}},
		{Type: "message", TS: "1700000003.000000", User: "U01", Text: "Report and photo without sizes", Files: []slack.File{report,
			{ID: "F-PHOTO", Name: "photo.png", Mimetype: "image/png",
				URLPrivateDownload: "{{base}}/files/photo-original.png", Thumb360: "{{base}}/files/photo-thumb.png"}}},
		{Type: "message", TS: "1700000002.000000", User: "U02", Text: "Archive and picture over the limit", Files: []slack.File{
			{ID: "F-ZIP", Name: "big-archive.zip", Mimetype: "application/zip", Size: 5000, URLPrivateDownload: "{{base}}/files/big-archive.zip"},
			{ID: "F-BIGIMG", Name: "huge-photo.png", Mimetype: "image/png", Size: 5000,
				URLPrivateDownload: "{{base}}/files/huge-original.png", Thumb360: "{{base}}/files/huge-thumb.png"}}},
		{Type: "message", TS: "1700000001.000000", User: "U01", Text: "Notes within the limit", Files: []slack.File{
			{ID: "F-TXT", Name: "notes.txt", Mimetype: "text/plain", Size: 10, URLPrivateDownload: "{{base}}/files/notes.txt"}}},
	}
	sc.Assets = map[string]fakeAsset{
		"/files/report.pdf":         {ContentType: "application/pdf", Body: strings.Repeat("r", 200)},
		"/files/photo-original.png": pngAsset(strings.Repeat("p", 200)),
		"/files/photo-thumb.png":    pngAsset("photo thumb"),
		"/files/big-archive.zip":    {ContentType: "application/zip", Body: strings.Repeat("z", 5000)},
		"/files/huge-original.png":  pngAsset(strings.Repeat("h", 5000)),
		"/files/huge-thumb.png":     pngAsset("huge thumb"),
		"/files/notes.txt":          {ContentType: "text/plain", Body: "notes body"},
	}
	return sc
}

// assetFaultsScenario shows assets whose downloads fail: the workspace icon
// and a report at once (not found), and a file after its retries (a server
// error each time); bob's avatar is rate limited once, with Retry-After,
// before it comes. The avatars go ahead only once users.info has returned,
// after the files of the page, and come early in the plan.
func assetFaultsScenario() exportScenario {
	sc := baseScenario()
	sc.TeamInfo = &slack.TeamInfo{ID: "TACME123", Name: "Acme Workspace", Domain: "acme",
		Icon: slack.TeamIcon{Image68: "{{base}}/files/workspace-icon.png"}}
	sc.Users["U01"] = testUser("U01", "alice", "Alice Example", "Alice", "{{base}}/files/avatar-u01.png")
	sc.Users["U02"] = testUser("U02", "bob", "Bob Builder", "Bob", "{{base}}/files/avatar-u02.png")
	file := func(id, name string) slack.File {
		return slack.File{ID: id, Name: name, Mimetype: "application/pdf", Size: 50, URLPrivateDownload: "{{base}}/files/" + name}
	}
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000003.000000", User: "U01", Text: "Flaky report", Files: []slack.File{file("F-FLAKY", "flaky.pdf")}},
		{Type: "message", TS: "1700000002.000000", User: "U02", Text: "Screenshot", Files: []slack.File{
			{ID: "F-SHOT", Name: "shot.png", Mimetype: "image/png", Size: 20,
				URLPrivateDownload: "{{base}}/files/shot-original.png", Thumb360: "{{base}}/files/shot-thumb.png"}}},
		{Type: "message", TS: "1700000001.000000", User: "U01", Text: "Report that went missing", Files: []slack.File{file("F-GONE", "missing.pdf")}},
	}
	sc.Assets = map[string]fakeAsset{
		"/files/avatar-u01.png":    pngAsset("avatar-u01"),
		"/files/avatar-u02.png":    pngAsset("avatar-u02"),
		"/files/shot-original.png": pngAsset("shot original"),
		"/files/shot-thumb.png":    pngAsset("shot thumb"),
		"/files/flaky.pdf":         {ContentType: "application/pdf", Body: "flaky report"},
	}
	sc.AssetFaults = map[string]*endpointFault{
		// Not in Assets: the server does not find them.
		"/files/workspace-icon.png": {},
		"/files/missing.pdf":        {},
		"/files/avatar-u02.png":     {transient: []faultResponse{{httpStatus: http.StatusTooManyRequests, retryAfterSec: 1}}},
		"/files/flaky.pdf":          {sticky: &faultResponse{httpStatus: http.StatusInternalServerError}},
	}
	return sc
}

// sharedURLScenario shows URLs more than once, where the request the history
// page gives first — the newest message's — differs from the one the plan
// takes, the oldest message's: a custom emoji, as its alias and by its own
// name, a file posted twice, and two images shown both in a URL preview and
// as a file, one under a larger limit first, the other under a smaller. The
// preview is 200 bytes, over the limit of smallAttachmentOptions.
func sharedURLScenario() exportScenario {
	shared := slack.File{ID: "F-SHARED", Name: "shared.pdf", Mimetype: "application/pdf", Size: 12, URLPrivateDownload: "{{base}}/files/shared.pdf"}
	sc := baseScenario()
	sc.Emoji = map[string]string{
		"party_sloth": "{{base}}/files/emoji-party-sloth.png",
		"sloth":       "alias:party_sloth",
	}
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000004.000000", User: "U01", Text: "Newest, with :sloth:",
			Attachments: []slack.Attachment{{ServiceName: "Example", Title: "Preview", TitleLink: "https://example.com/preview",
				ImageURL: "{{base}}/files/preview.png"}}},
		{Type: "message", TS: "1700000003.000000", User: "U02", Text: "Shared file with a picture", Files: []slack.File{shared},
			Attachments: []slack.Attachment{{ServiceName: "Example", Title: "Picture", TitleLink: "https://example.com/picture",
				ImageURL: "{{base}}/files/picture.png"}}},
		{Type: "message", TS: "1700000002.000000", User: "U01", Text: "The picture as a file", Files: []slack.File{
			{ID: "F-PIC", Name: "picture.png", Mimetype: "image/png", Size: 16,
				URLPrivateDownload: "{{base}}/files/picture-original.png", Thumb360: "{{base}}/files/picture.png"}},
			Reactions: []slack.Reaction{{Name: "party_sloth", Count: 1}}},
		{Type: "message", TS: "1700000001.000000", User: "U02", Text: "The preview as a file, and the shared file again", Files: []slack.File{
			{ID: "F-PREVIEW", Name: "preview.bin", Mimetype: "application/octet-stream", URLPrivateDownload: "{{base}}/files/preview.png"},
			shared}},
	}
	sc.Assets = map[string]fakeAsset{
		"/files/emoji-party-sloth.png": pngAsset("custom-emoji"),
		"/files/shared.pdf":            {ContentType: "application/pdf", Body: "shared file"},
		"/files/preview.png":           pngAsset(strings.Repeat("v", 200)),
		"/files/picture.png":           pngAsset("picture thumb"),
		"/files/picture-original.png":  pngAsset("picture original"),
	}
	return sc
}

// manifestEntryWithEmoji is a manifest entry with the emoji name, which
// manifestEntryFull leaves out.
type manifestEntryWithEmoji struct {
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	SourceURL string `json:"source_url"`
	EmojiName string `json:"emoji_name"`
}

func readManifestWithEmoji(t *testing.T, dir string) []manifestEntryWithEmoji {
	t.Helper()
	var manifest struct {
		Assets []manifestEntryWithEmoji `json:"assets"`
	}
	readJSON(t, filepath.Join(dir, ".cache/assets_manifest.json"), &manifest)
	return manifest.Assets
}

// refetchScenario: see TestRunIntegrationAssetPrefetchRefetch. The picture is
// 200 bytes, over the limit of smallAttachmentOptions.
func refetchScenario() exportScenario {
	const picture = "{{base}}/files/picture.png"
	sc := baseScenario()
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000002.000000", User: "U01", Text: "The picture as a file", Files: []slack.File{
			{ID: "F-PIC", Name: "picture.bin", Mimetype: "application/octet-stream", URLPrivateDownload: picture}}},
		{Type: "message", TS: "1700000001.000000", User: "U02", Text: "The picture as a preview",
			Attachments: []slack.Attachment{{ServiceName: "Example", Title: "Picture", TitleLink: "https://example.com/picture",
				ImageURL: picture}}},
	}
	sc.Assets["/files/picture.png"] = pngAsset(strings.Repeat("p", 200))
	return sc
}

// filteredFilesScenario is droppedThreadsScenario with a file on each message
// and reply (filteredFilePath).
func filteredFilesScenario() exportScenario {
	sc := droppedThreadsScenario()
	withFile := func(m *slack.Message) {
		body := "file of " + m.TS
		m.Files = []slack.File{{ID: "F" + m.TS, Name: "file-" + m.TS + ".pdf", Mimetype: "application/pdf",
			Size: int64(len(body)), URLPrivateDownload: "{{base}}" + filteredFilePath(m.TS)}}
		sc.Assets[filteredFilePath(m.TS)] = fakeAsset{ContentType: "application/pdf", Body: body}
	}
	for i := range sc.Messages {
		withFile(&sc.Messages[i])
	}
	for _, replies := range sc.Replies {
		for i := range replies {
			withFile(&replies[i])
		}
	}
	return sc
}

// filteredFilePath is the path of the file of filteredFilesScenario's message
// or reply ts, the same on each copy of it.
func filteredFilePath(ts string) string {
	return "/files/file-" + ts + ".pdf"
}

// removeCachedAsset removes, from the output of the run that kept the cache
// in cacheDir, the file of the asset of kind that the manifest lists first:
// the reuse cache cannot copy it.
func removeCachedAsset(t *testing.T, cacheDir, kind string) {
	t.Helper()
	dir := filepath.Dir(cacheDir)
	entry, ok := findManifest(readManifestEntries(t, dir), func(e manifestEntryFull) bool {
		return e.Kind == kind && e.LocalPath != ""
	})
	if !ok {
		t.Fatalf("the kept manifest has no saved %s", kind)
	}
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(entry.LocalPath))); err != nil {
		t.Fatalf("remove the cached %s: %v", kind, err)
	}
}
