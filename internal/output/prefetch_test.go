package output

// Prefetch tests (Issue #279, PF-07 of #272): the downloads Prefetch sends
// ahead stand for Fetch's own, so that an Assets that prefetched records the
// same manifest, files, notices and warnings as one that did not, with the
// planned kind and metadata; a download sent ahead that stopped at a smaller
// size limit than the plan's is made again, and one the plan does not take
// leaves no file behind.

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
)

// servedFile is how the fake file server answers the requests for a URL.
type servedFile struct {
	body string
	// retry makes the first request of the URL fail with a 503, so that the
	// download is retried once, with a notice.
	retry bool
	// status is the status of the answer after the retry; 0 is 200.
	status int
	// hold makes each answer wait until it is closed.
	hold chan struct{}
}

// fileServer serves files by URL through a transport, and counts the requests
// for each URL.
type fileServer struct {
	files map[string]servedFile

	mu       sync.Mutex
	requests map[string]int
	started  chan string // the URL of each request, once it has come
}

func newFileServer(files map[string]servedFile) *fileServer {
	return &fileServer{files: files, requests: map[string]int{}, started: make(chan string, 64)}
}

func (s *fileServer) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	s.mu.Lock()
	s.requests[u]++
	n := s.requests[u]
	s.mu.Unlock()
	s.started <- u
	f, ok := s.files[u]
	if !ok {
		return nil, fmt.Errorf("unexpected request")
	}
	if f.hold != nil {
		select {
		case <-f.hold:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	if f.retry && n == 1 {
		return fakeResponse(req, http.StatusServiceUnavailable, ""), nil
	}
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	return fakeResponse(req, status, f.body), nil
}

func (s *fileServer) counts() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.requests)
}

// assetRequest is one request of a render for an asset: Save, or SkipTooLarge
// with skipSize.
type assetRequest struct {
	kind     string
	url      string
	meta     AssetMeta
	skipSize bool
}

// planOf is the plan a planner of an Assets with the per-file limit limit
// records for requests.
func planOf(limit int64, requests []assetRequest) []PlannedAsset {
	planner := NewAssets(context.Background(), nil, "", limit).Planner()
	for _, r := range requests {
		if r.skipSize {
			planner.SkipTooLarge(r.kind, r.url, r.meta)
		} else {
			planner.Save(r.kind, r.url, r.meta)
		}
	}
	return planner.Plan()
}

// fetchResult is what an Assets recorded and left: its manifest, the files in
// its directory, the notices and warnings it passed on, and the requests of
// each URL.
type fetchResult struct {
	entries  []ManifestEntry
	files    map[string]string
	out      []string
	requests map[string]int
}

// fetchWithPrefetch sends ahead what ahead asks for, then acquires what
// requests ask for as the export does — Fetch of their plan, then a Save or
// SkipTooLarge of each in turn — and closes the Assets. before runs between
// the Prefetch and the Fetch.
func fetchWithPrefetch(t *testing.T, srv *fileServer, limit int64, ahead, requests []assetRequest, before func(a *Assets)) fetchResult {
	t.Helper()
	dir := t.TempDir()
	a := NewAssets(context.Background(), newFetchClient(srv), dir, limit)
	var mu sync.Mutex
	var out []string
	a.Logf = func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		out = append(out, "warning: "+fmt.Sprintf(format, args...))
	}
	a.Notef = func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		out = append(out, "notice: "+retryWait.ReplaceAllString(fmt.Sprintf(format, args...), "in <d> "))
	}
	if ahead != nil {
		a.Prefetch(planOf(limit, ahead))
	}
	if before != nil {
		before(a)
	}
	a.Fetch(planOf(limit, requests))
	for _, r := range requests {
		if r.skipSize {
			a.SkipTooLarge(r.kind, r.url, r.meta)
		} else {
			a.Save(r.kind, r.url, r.meta)
		}
	}
	a.Close()
	mu.Lock()
	defer mu.Unlock()
	return fetchResult{entries: a.Entries(), files: collectFiles(t, dir), out: slices.Clone(out), requests: srv.counts()}
}

// assertSameFetch fails the test when got records or leaves anything other
// than want does.
func assertSameFetch(t *testing.T, got, want fetchResult) {
	t.Helper()
	if !slices.Equal(got.entries, want.entries) {
		t.Errorf("manifest with the prefetch =\n%+v\nwant\n%+v", got.entries, want.entries)
	}
	if !maps.Equal(got.files, want.files) {
		t.Errorf("files with the prefetch = %q\nwant %q", slices.Sorted(maps.Keys(got.files)), slices.Sorted(maps.Keys(want.files)))
	}
	if !slices.Equal(got.out, want.out) {
		t.Errorf("passed on with the prefetch = %q\nwant %q", got.out, want.out)
	}
}

// TestAssetsPrefetchMatchesFetch: an asset whose download went ahead is
// recorded, saved, and reported as Fetch's own download of it would be — a
// saved file, a failure, a download the size limit stopped, each with its
// retry notices, in plan order — under the planned kind and metadata, with
// one request.
func TestAssetsPrefetchMatchesFetch(t *testing.T) {
	t.Parallel()

	const limit = 16
	files := func() map[string]servedFile {
		return map[string]servedFile{
			"https://a.example/avatar.png":     {body: "avatar", retry: true},
			"https://e.example/party.gif":      {body: "party parrot", retry: true},
			"https://files.slack.com/f/report": {status: http.StatusNotFound, retry: true},
			"https://files.slack.com/f/big":    {body: strings.Repeat("x", 2*limit)},
			"https://files.slack.com/f/fits":   {body: strings.Repeat("y", limit)},
			"https://u.example/og.png":         {body: "og image"},
		}
	}
	requests := []assetRequest{
		{kind: KindAvatar, url: "https://a.example/avatar.png"},
		// The emoji is sent ahead under another name of it (an alias).
		{kind: KindEmoji, url: "https://e.example/party.gif", meta: AssetMeta{EmojiName: "party_alias"}},
		{kind: KindAttachment, url: "https://files.slack.com/f/report", meta: AssetMeta{FileID: "F1", OriginalName: "report.pdf"}},
		{kind: KindUploadOriginal, url: "https://files.slack.com/f/big", meta: AssetMeta{FileID: "F2", OriginalName: "big.png"}},
		{kind: KindAttachment, url: "https://files.slack.com/f/fits", meta: AssetMeta{FileID: "F3", OriginalName: "fits.txt"}},
		{kind: KindOGImage, url: "https://u.example/og.png"},
	}
	// The downloads go ahead in another order than the plan's, and one of
	// them is not sent ahead.
	ahead := []assetRequest{
		{kind: KindUploadOriginal, url: "https://files.slack.com/f/big", meta: AssetMeta{FileID: "F2", OriginalName: "big.png", SizeBytes: 1}},
		{kind: KindEmoji, url: "https://e.example/party.gif", meta: AssetMeta{EmojiName: "party"}},
		{kind: KindAttachment, url: "https://files.slack.com/f/fits", meta: AssetMeta{FileID: "F3", OriginalName: "fits.txt"}},
		{kind: KindAttachment, url: "https://files.slack.com/f/report", meta: AssetMeta{FileID: "F1", OriginalName: "report.pdf"}},
		{kind: KindAvatar, url: "https://a.example/avatar.png"},
	}
	serial := fetchWithPrefetch(t, newFileServer(files()), limit, nil, requests, nil)
	if len(serial.out) == 0 {
		t.Fatal("the serial fetch passed nothing on; the scenario lost its notices and warnings")
	}
	srv := newFileServer(files())
	got := fetchWithPrefetch(t, srv, limit, ahead, requests, func(*Assets) {
		// Fetch comes once the downloads sent ahead have all been requested.
		for range 5 {
			select {
			case <-srv.started:
			case <-time.After(10 * time.Second):
				t.Fatal("timed out waiting for the downloads sent ahead")
			}
		}
	})
	assertSameFetch(t, got, serial)
	if !maps.Equal(got.requests, serial.requests) {
		t.Errorf("requests with the prefetch = %v, want %v", got.requests, serial.requests)
	}
	if e := findEntry(t, got.entries, "https://e.example/party.gif"); e.EmojiName != "party_alias" {
		t.Errorf("emoji entry names %q, want the planned name party_alias", e.EmojiName)
	}
}

// TestAssetsPrefetchUnderWay: Fetch waits for a download sent ahead that is
// still under way, and takes its result.
func TestAssetsPrefetchUnderWay(t *testing.T) {
	t.Parallel()

	hold := make(chan struct{})
	files := map[string]servedFile{"https://a.example/avatar.png": {body: "avatar", retry: true, hold: hold}}
	requests := []assetRequest{{kind: KindAvatar, url: "https://a.example/avatar.png"}}
	serial := fetchWithPrefetch(t, newFileServer(map[string]servedFile{"https://a.example/avatar.png": {body: "avatar", retry: true}}), 0, nil, requests, nil)
	srv := newFileServer(files)
	got := fetchWithPrefetch(t, srv, 0, requests, requests, func(*Assets) {
		select {
		case <-srv.started:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for the download sent ahead")
		}
		time.AfterFunc(20*time.Millisecond, func() { close(hold) })
	})
	assertSameFetch(t, got, serial)
	if n := got.requests["https://a.example/avatar.png"]; n != 2 {
		t.Errorf("%d requests, want the download sent ahead and its retry", n)
	}
}

// TestAssetsPrefetchSizeLimits: the size limit is the planned kind's. A
// download sent ahead with a larger limit than the plan's, or none, stands
// for the plan's when its content is over the plan's limit — the plan's
// download would have stopped — and one that stopped at a smaller limit than
// the plan's is made again, with the plan's. The one the plan keeps out by
// size, and the one it does not ask for, go unused and leave no file.
func TestAssetsPrefetchSizeLimits(t *testing.T) {
	t.Parallel()

	const limit = 16
	files := func() map[string]servedFile {
		return map[string]servedFile{
			"https://x.example/over-limit.png":      {body: strings.Repeat("o", 2*limit)},
			"https://x.example/stopped-short.png":   {body: strings.Repeat("s", 2*limit)},
			"https://files.slack.com/f/kept-out":    {body: strings.Repeat("k", 2*limit)},
			"https://x.example/not-in-the-plan.png": {body: "unused"},
		}
	}
	requests := []assetRequest{
		{kind: KindAttachment, url: "https://x.example/over-limit.png"},
		{kind: KindAvatar, url: "https://x.example/stopped-short.png"},
		{kind: KindAttachment, url: "https://files.slack.com/f/kept-out", meta: AssetMeta{FileID: "F1", SizeBytes: 2 * limit}, skipSize: true},
	}
	ahead := []assetRequest{
		// An avatar has no limit, an attachment the limit given.
		{kind: KindAvatar, url: "https://x.example/over-limit.png"},
		{kind: KindAttachment, url: "https://x.example/stopped-short.png"},
		{kind: KindAttachment, url: "https://files.slack.com/f/kept-out", meta: AssetMeta{FileID: "F1"}},
		{kind: KindAvatar, url: "https://x.example/not-in-the-plan.png"},
	}
	serial := fetchWithPrefetch(t, newFileServer(files()), limit, nil, requests, nil)
	got := fetchWithPrefetch(t, newFileServer(files()), limit, ahead, requests, nil)
	assertSameFetch(t, got, serial)
	want := maps.Clone(serial.requests)
	// The download that stopped short is made again, and the unused ones
	// were made: each is a request more than the serial fetch makes.
	want["https://x.example/stopped-short.png"]++
	want["https://files.slack.com/f/kept-out"]++
	want["https://x.example/not-in-the-plan.png"]++
	if !maps.Equal(got.requests, want) {
		t.Errorf("requests with the prefetch = %v, want %v", got.requests, want)
	}
	if e := findEntry(t, got.entries, "https://x.example/over-limit.png"); e.Status != StatusSkippedSize {
		t.Errorf("the download over the planned limit is %q, want %q", e.Status, StatusSkippedSize)
	}
	if e := findEntry(t, got.entries, "https://x.example/stopped-short.png"); e.Status != StatusSaved {
		t.Errorf("the download made again is %q, want %q", e.Status, StatusSaved)
	}
}

// TestAssetsPrefetchSkipsReusable: a URL the reuse source can copy is not
// sent ahead: Fetch copies it.
func TestAssetsPrefetchSkipsReusable(t *testing.T) {
	t.Parallel()

	oldDir := t.TempDir()
	rel := "assets/avatars/old.png"
	if err := os.MkdirAll(filepath.Join(oldDir, "assets/avatars"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, rel), []byte("old avatar"), 0o644); err != nil {
		t.Fatal(err)
	}
	const u = "https://a.example/avatar.png"
	srv := newFileServer(map[string]servedFile{u: {body: "avatar"}})
	dir := t.TempDir()
	a := NewAssets(context.Background(), newFetchClient(srv), dir, 0)
	a.SetReuseSource(&ReuseSource{OldDir: oldDir, Entries: map[string]ManifestEntry{
		u: {Kind: KindAvatar, SourceURL: u, LocalPath: rel, Status: StatusSaved},
	}})
	requests := []assetRequest{{kind: KindAvatar, url: u}}
	a.Prefetch(planOf(0, requests))
	a.Fetch(planOf(0, requests))
	if _, ok := a.Save(KindAvatar, u, AssetMeta{}); !ok {
		t.Error("Save ok = false, want the copy")
	}
	a.Close()
	if n := srv.counts()[u]; n != 0 {
		t.Errorf("%d requests, want none: the reuse source has the asset", n)
	}
	if a.Reused() != 1 {
		t.Errorf("Reused = %d, want 1", a.Reused())
	}
}

// TestAssetsCloseStopsPrefetch: Close stops the downloads sent ahead that no
// Fetch took, waits for them, and removes their temporary files; a Prefetch
// after it sends nothing.
func TestAssetsCloseStopsPrefetch(t *testing.T) {
	t.Parallel()

	hold := make(chan struct{})
	defer close(hold)
	srv := newFileServer(map[string]servedFile{
		"https://a.example/done.png": {body: "done"},
		"https://b.example/held.png": {body: "held", hold: hold},
		"https://c.example/late.png": {body: "late"},
	})
	dir := t.TempDir()
	a := NewAssets(context.Background(), newFetchClient(srv), dir, 0)
	a.Prefetch(planOf(0, []assetRequest{
		{kind: KindAvatar, url: "https://a.example/done.png"},
		{kind: KindAvatar, url: "https://b.example/held.png"},
	}))
	for range 2 {
		select {
		case <-srv.started:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for the downloads sent ahead")
		}
	}
	// The first download has ended, and its content waits in its temporary
	// file.
	waitForContent(t, dir, "done")
	a.Close()
	if files := collectFiles(t, dir); len(files) != 0 {
		t.Errorf("files left = %q, want none", slices.Sorted(maps.Keys(files)))
	}
	a.Prefetch(planOf(0, []assetRequest{{kind: KindAvatar, url: "https://c.example/late.png"}}))
	time.Sleep(10 * time.Millisecond)
	if n := srv.counts()["https://c.example/late.png"]; n != 0 {
		t.Errorf("%d requests after Close, want none", n)
	}
}

// TestAssetsPrefetchCanceled: once the context is done, the downloads sent
// ahead stop and remove their temporary files, and Fetch takes nothing.
func TestAssetsPrefetchCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hold := make(chan struct{})
	defer close(hold)
	srv := newFileServer(map[string]servedFile{"https://b.example/held.png": {body: "held", hold: hold}})
	dir := t.TempDir()
	a := NewAssets(ctx, newFetchClient(srv), dir, 0)
	var reported []string
	a.Logf = func(format string, args ...any) { reported = append(reported, fmt.Sprintf(format, args...)) }
	a.Notef = a.Logf
	requests := []assetRequest{{kind: KindAvatar, url: "https://b.example/held.png"}}
	a.Prefetch(planOf(0, requests))
	select {
	case <-srv.started:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the download sent ahead")
	}
	cancel()
	a.Fetch(planOf(0, requests))
	a.Close()
	if files := collectFiles(t, dir); len(files) != 0 {
		t.Errorf("files left = %q, want none", slices.Sorted(maps.Keys(files)))
	}
	if len(reported) != 0 {
		t.Errorf("passed on %q, want nothing", reported)
	}
	if n := srv.counts()["https://b.example/held.png"]; n != 1 {
		t.Errorf("%d requests, want the one sent ahead", n)
	}
}

// waitForContent waits until a file in dir holds content.
func waitForContent(t *testing.T, dir, content string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !slices.Contains(slices.Collect(maps.Values(collectFiles(t, dir))), content) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for a file of %q in %s", content, dir)
		}
		time.Sleep(time.Millisecond)
	}
}
