package export

// Integration cases of the prefetch (Issue #278, PF-06 of #272; decision log
// 0069). An export whose later stages' requests go out ahead of them must be
// the export that makes each request when its stage asks for it, as the
// serial export did (the prefetch off, prefetchOffKey): the same files, the
// same log once its durations are normalized and, when it succeeds, the same
// requests of each method, which the stages take in the same order. When it
// fails or is canceled, it stops with the same error at the same point, with
// the same log and files, and it has sent ahead only requests that it makes
// when nothing fails. A synctest case times a whole export with the prefetch
// and without it, over in-memory connections.
//
// With the prefetch, the downloads of the assets the page is certain to show
// go ahead too (Issue #279): each case that succeeds checks that every
// download of the serial run went ahead of the Assets phase (runAhead,
// integration_asset_prefetch_test.go, which holds the cases of the downloads).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// takenRequest is a request whose result a stage took, and whether the
// request went ahead of the stage (requestTakenKey).
type takenRequest struct {
	key   requestKey
	ahead bool
}

// prefetchRun is a run of the export with the prefetch on or off.
type prefetchRun struct {
	exportRunResult
	err      error
	root     string         // opts.OutputDir
	requests []string       // the requestName of each request the run sent
	taken    []takenRequest // in the order the stages took them
}

// runWithPrefetch runs the export of sc against a server of its own, with
// the prefetch on or off. hook, when set, is called on the driver each time a
// stage has taken the result of a request.
func runWithPrefetch(t *testing.T, ctx context.Context, on bool, sc exportScenario, opts Options, hook func(requestKey)) prefetchRun {
	t.Helper()
	fake := newFakeSlackServer(t, &sc)
	t.Cleanup(fake.Close)
	return runWithPrefetchOn(t, ctx, on, fake, opts, hook)
}

// runWithPrefetchOn is runWithPrefetch against fake, which may serve other
// runs before and after.
func runWithPrefetchOn(t *testing.T, ctx context.Context, on bool, fake *fakeSlackServer, opts Options, hook func(requestKey), clientOpts ...slack.Option) prefetchRun {
	t.Helper()
	run := prefetchRun{root: opts.OutputDir}
	if !on {
		ctx = context.WithValue(ctx, prefetchOffKey{}, true)
	}
	ctx = context.WithValue(ctx, requestTakenKey{}, func(key requestKey, ahead bool) {
		run.taken = append(run.taken, takenRequest{key: key, ahead: ahead})
		if hook != nil {
			hook(key)
		}
	})
	before := len(fake.Requests())
	run.exportRunResult, _, run.err = runExportOn(t, ctx, fake, opts, clientOpts...)
	run.requests = fake.Requests()[before:]
	return run
}

// assertPrefetchMatchesSerial checks two runs that succeed: the prefetched
// run writes the serial run's export and log, makes its requests, and its
// stages take the same requests in the same order. None of the serial run's
// went ahead; of the prefetched run's, every users.info, bots.info and
// emoji.list did, and repliesAhead conversations.replies did: those of the
// thread parents the history pages retained.
func assertPrefetchMatchesSerial(t *testing.T, serial, prefetched prefetchRun, repliesAhead int) {
	t.Helper()
	for _, run := range []prefetchRun{serial, prefetched} {
		if run.err != nil {
			t.Fatalf("Run() error = %v\nlogs:\n%s", run.err, strings.Join(run.Logs, "\n"))
		}
	}
	assertSameExport(t, "the prefetched run", serial.exportRunResult, prefetched.exportRunResult)
	if a, b := slices.Sorted(slices.Values(serial.requests)), slices.Sorted(slices.Values(prefetched.requests)); !slices.Equal(a, b) {
		t.Errorf("requests differ:\nserial:     %q\nprefetched: %q", a, b)
	}
	if a, b := takenKeys(serial.taken), takenKeys(prefetched.taken); !slices.Equal(a, b) {
		t.Errorf("the stages took other requests:\nserial:     %v\nprefetched: %v", a, b)
	}
	for _, r := range serial.taken {
		if r.ahead {
			t.Errorf("the serial run sent %v ahead", r.key)
		}
	}
	replies := 0
	for _, r := range prefetched.taken {
		switch {
		case r.key.method == "conversations.replies":
			if r.ahead {
				replies++
			}
		case !r.ahead:
			t.Errorf("the prefetched run did not send %v ahead", r.key)
		}
	}
	if replies != repliesAhead {
		t.Errorf("conversations.replies sent ahead = %d, want %d", replies, repliesAhead)
	}
}

func takenKeys(taken []takenRequest) []requestKey {
	keys := make([]requestKey, len(taken))
	for i, r := range taken {
		keys[i] = r.key
	}
	return keys
}

func TestRunIntegrationPrefetchMatchesSerial(t *testing.T) {
	t.Parallel()

	maxPosts := func(n int) func(*testing.T) Options {
		return func(t *testing.T) Options { return integrationOptions(t, n) }
	}
	// A broadcast whose parent never reaches the timeline and carries an
	// excluded emoji (TestRunIntegrationBroadcastParentOffTimeline).
	offTimelineParent := func(parentTS string, replyTS [2]string) func() exportScenario {
		return func() exportScenario {
			return offTimelineParentScenario(parentTS, replyTS, "root of the broadcast thread :shushing_face:")
		}
	}
	offTimelineOptions := func(t *testing.T) Options {
		opts := integrationOptions(t, 3)
		opts.Days = 1
		opts.ExcludeBodyEmoji = []string{"shushing_face"}
		return opts
	}
	for _, tc := range []struct {
		name         string
		scenario     func() exportScenario
		options      func(*testing.T) Options
		repliesAhead int
	}{
		{name: "happy path", scenario: happyPathScenario, options: maxPosts(10), repliesAhead: 1},
		{name: "--max-posts cut after the thread", scenario: happyPathScenario, options: maxPosts(2), repliesAhead: 1},
		{name: "--max-posts cut before the thread", scenario: happyPathScenario, options: maxPosts(1)},
		{name: "emoji filters exclude a message and a reply", scenario: filteredReplyScenario, options: filteredReplyOptions, repliesAhead: 1},
		{name: "threads across history pages", scenario: threadsAcrossPagesScenario, options: threadsAcrossPagesOptions, repliesAhead: 1},
		{name: "thread messages the filter drops later", scenario: droppedThreadsScenario, options: droppedThreadsOptions, repliesAhead: 2},
		{name: "parent excluded on a later page", scenario: laterPageExclusionScenario, options: laterPageExclusionOptions},
		{name: "broadcast whose excluded parent is before the range",
			scenario: offTimelineParent("1699800000.000000", [2]string{"1699800000.100000", "1699800000.200000"}),
			options:  offTimelineOptions},
		{name: "broadcast whose excluded parent is beyond --max-posts",
			scenario: offTimelineParent("1700000001.000000", [2]string{"1700000001.100000", "1700000001.200000"}),
			options:  offTimelineOptions},
		{name: "replies over the cap", scenario: repliesOverCapScenario, options: renderingOptions, repliesAhead: 1},
		{name: "users and bots resolved late or never", scenario: peopleScenario, options: renderingOptions, repliesAhead: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serial := runWithPrefetch(t, context.Background(), false, tc.scenario(), tc.options(t), nil)
			prefetched := runAhead(t, tc.scenario(), tc.options(t), downloadsOf(serial.requests))
			assertPrefetchMatchesSerial(t, serial, prefetched, tc.repliesAhead)
		})
	}
}

// TestRunIntegrationPrefetchMatchesSerialWithReuseCache: with --reuse-cache,
// the users the cache holds are neither sent ahead nor asked for, and neither
// is emoji.list; a user the cache lacks is. The assets the cache holds are
// copied, and neither sent ahead nor asked for; an asset the cache lacks is
// downloaded ahead (Issue #279), the avatar of a user or bot the cache holds
// as soon as the Users stage has its ID.
func TestRunIntegrationPrefetchMatchesSerialWithReuseCache(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		scenario     func() exportScenario // happyPathScenario when nil
		repliesAhead int                   // the conversations.replies sent ahead
		tamper       func(t *testing.T, cacheDir string)
		users        int      // the users.info requests
		downloads    []string // the assets the cache lacks
	}{
		{name: "cache holds every user", repliesAhead: 1},
		{name: "cache lacks a user", repliesAhead: 1, tamper: func(t *testing.T, cacheDir string) {
			rewriteJSON(t, filepath.Join(cacheDir, "slack_api_cache.json"), func(m map[string]any) {
				delete(m["users"].(map[string]any), "U02")
			})
		}, users: 1},
		{name: "cache lacks an asset", repliesAhead: 1, tamper: func(t *testing.T, cacheDir string) {
			removeCachedAsset(t, cacheDir, "/files/runbook.pdf")
		}, downloads: []string{"/files/runbook.pdf"}},
		{name: "cache lacks the avatar of a user it holds", repliesAhead: 1, tamper: func(t *testing.T, cacheDir string) {
			removeCachedAsset(t, cacheDir, "/files/avatar-u01.png")
		}, downloads: []string{"/files/avatar-u01.png"}},
		{name: "cache lacks the icon of a bot it holds", scenario: botReuseScenario, tamper: func(t *testing.T, cacheDir string) {
			removeCachedAsset(t, cacheDir, "/files/bot-icon.png")
		}, downloads: []string{"/files/bot-icon.png"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sc := happyPathScenario()
			if tc.scenario != nil {
				sc = tc.scenario()
			}
			come := newArrivals(nil)
			sc.BeforeAsset = come.record
			fake := newFakeSlackServer(t, &sc)
			t.Cleanup(fake.Close)
			first, _, err := runExportOn(t, context.Background(), fake, reuseOptions(t, true))
			if err != nil {
				t.Fatalf("run to keep the cache: %v", err)
			}
			cacheDir := filepath.Join(first.OutputDir, ".cache")
			if tc.tamper != nil {
				tc.tamper(t, cacheDir)
			}
			reuse := func() Options {
				opts := reuseOptions(t, false)
				opts.ReuseCache = cacheDir
				return opts
			}
			serial := runWithPrefetchOn(t, context.Background(), false, fake, reuse(), nil)
			prefetched := runAheadOn(t, fake, come, reuse(), downloadsOf(serial.requests))
			assertPrefetchMatchesSerial(t, serial, prefetched, tc.repliesAhead)
			if slices.Contains(prefetched.requests, "/api/emoji.list") {
				t.Errorf("emoji.list went out with the reuse cache")
			}
			if n := countPrefix(prefetched.requests, "/api/users.info"); n != tc.users {
				t.Errorf("users.info requests = %d, want %d", n, tc.users)
			}
			if got := downloadsOf(prefetched.requests); !slices.Equal(got, tc.downloads) {
				t.Errorf("downloads = %q, want %q: the reuse cache holds the rest", got, tc.downloads)
			}
		})
	}
}

func countPrefix(requests []string, prefix string) int {
	n := 0
	for _, r := range requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

// assertPrefetchFailsAsSerial checks two runs that fail: the prefetched run
// fails with the serial run's error, which cmd/slapex maps to the same exit
// code, and with its log and output. It has sent ahead only requests that the
// complete run, which nothing fails, makes too, and among them aheadOnly,
// which the serial run never got to.
func assertPrefetchFailsAsSerial(t *testing.T, serial, prefetched, complete prefetchRun, wantErr string, aheadOnly ...string) {
	t.Helper()
	if complete.err != nil {
		t.Fatalf("the run without the failure: %v\nlogs:\n%s", complete.err, strings.Join(complete.Logs, "\n"))
	}
	if serial.err == nil || serial.err.Error() != wantErr {
		t.Fatalf("serial Run() error = %v, want %s\nlogs:\n%s", serial.err, wantErr, strings.Join(serial.Logs, "\n"))
	}
	if prefetched.err == nil || prefetched.err.Error() != serial.err.Error() {
		t.Fatalf("prefetched Run() error = %v, want %v\nlogs:\n%s", prefetched.err, serial.err, strings.Join(prefetched.Logs, "\n"))
	}
	if a, b := exitClass(serial.err), exitClass(prefetched.err); a != b {
		t.Errorf("exit class = %s, want %s", b, a)
	}
	// Run returns no directory when it fails: compare what it left under
	// the output root.
	serial.OutputDir, prefetched.OutputDir = serial.root, prefetched.root
	assertSameExport(t, "the prefetched run", serial.exportRunResult, prefetched.exportRunResult)
	if a, b := outputTree(t, serial.root), outputTree(t, prefetched.root); !slices.Equal(a, b) {
		t.Errorf("output tree differs:\nserial:     %q\nprefetched: %q", a, b)
	}
	made := map[string]bool{}
	for _, r := range complete.requests {
		made[r] = true
	}
	for _, r := range prefetched.requests {
		if !made[r] {
			t.Errorf("the prefetched run sent %s, which the run without the failure does not", r)
		}
	}
	for _, r := range aheadOnly {
		if slices.Contains(serial.requests, r) || !slices.Contains(prefetched.requests, r) {
			t.Errorf("%s: in the serial run %t, in the prefetched run %t; want it sent ahead only",
				r, slices.Contains(serial.requests, r), slices.Contains(prefetched.requests, r))
		}
	}
}

// exitClass is what cmd/slapex picks the exit code of Run's error by
// (classify): a usage error, a Slack API error and its code, a cancellation,
// or any other error.
func exitClass(err error) string {
	var usage *UsageError
	var api *slack.APIError
	switch {
	case errors.As(err, &usage):
		return "usage"
	case errors.As(err, &api):
		return "slack api " + api.Code
	case errors.Is(err, context.Canceled):
		return "canceled"
	}
	return "other"
}

// outputTree lists the directories and files under root, relative to it.
func outputTree(t *testing.T, root string) []string {
	t.Helper()
	var tree []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			rel += "/"
		}
		tree = append(tree, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return tree
}

// holdUntil returns a BeforeAPI hook that holds the first request named held
// until the request named awaited has come count times.
func holdUntil(t *testing.T, held, awaited string, count int) func(*http.Request) {
	var (
		mu      sync.Mutex
		seen    = map[string]int{}
		arrived = make(chan struct{})
	)
	return func(r *http.Request) {
		name := requestName(r)
		mu.Lock()
		seen[name]++
		n := seen[name]
		mu.Unlock()
		if name == awaited && n == count {
			close(arrived)
		}
		if name == held && n == 1 {
			select {
			case <-arrived:
			case <-time.After(10 * time.Second):
				t.Errorf("%s was held, and %s did not come %d times", held, awaited, count)
			}
		}
	}
}

func TestRunIntegrationPrefetchFailsAsSerial(t *testing.T) {
	t.Parallel()

	const (
		failingThread = "/api/conversations.replies?ts=1700000002.000000"
		carol         = "/api/users.info?user=U03"
		refill        = "/api/conversations.history?latest=1700000010.000000"
		emojiList     = "/api/emoji.list"
		serverError   = "giving up after 5 retries: server error: HTTP 500"
	)
	sticky500 := func() *endpointFault {
		return &endpointFault{sticky: &faultResponse{httpStatus: http.StatusInternalServerError}}
	}
	for _, tc := range []struct {
		name string
		// scenario is the case's scenario, with its faults; for the
		// prefetched run, it holds a request that fails until another has
		// gone ahead.
		scenario  func(t *testing.T, prefetched bool) exportScenario
		options   func(*testing.T) Options
		wantErr   string
		aheadOnly []string
	}{
		{
			// The users.info of carol, rate limited once, has been retried
			// when the thread fails: the notice of its 429 is dropped.
			name: "thread replies fail",
			scenario: func(t *testing.T, prefetched bool) exportScenario {
				sc := twoThreadsScenario()
				sc.APIFaults = map[string]*endpointFault{
					failingThread: sticky500(),
					carol:         {transient: []faultResponse{{httpStatus: http.StatusTooManyRequests, retryAfterSec: 1}}},
				}
				if prefetched {
					sc.BeforeAPI = holdUntil(t, failingThread, carol, 2)
				}
				return sc
			},
			options:   renderingOptions,
			wantErr:   "slack api conversations.replies: " + serverError,
			aheadOnly: []string{carol},
		},
		{
			// The refill after the first page fails while emoji.list, which
			// went ahead when the Messages phase started, has come.
			name: "history refill fails",
			scenario: func(t *testing.T, prefetched bool) exportScenario {
				sc := threadsAcrossPagesScenario()
				sc.APIFaults = map[string]*endpointFault{refill: sticky500()}
				if prefetched {
					sc.BeforeAPI = holdUntil(t, refill, emojiList, 1)
				}
				return sc
			},
			options:   threadsAcrossPagesOptions,
			wantErr:   "slack api conversations.history: " + serverError,
			aheadOnly: []string{emojiList},
		},
		{
			// emoji.list fails ahead of the Emoji phase: its retries and its
			// error come there.
			name: "emoji.list fails",
			scenario: func(*testing.T, bool) exportScenario {
				sc := happyPathScenario()
				sc.APIFaults = map[string]*endpointFault{emojiList: sticky500()}
				return sc
			},
			options: func(t *testing.T) Options { return integrationOptions(t, 10) },
			wantErr: "slack api emoji.list: " + serverError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serial := runWithPrefetch(t, context.Background(), false, tc.scenario(t, false), tc.options(t), nil)
			prefetched := runWithPrefetch(t, context.Background(), true, tc.scenario(t, true), tc.options(t), nil)
			sc := tc.scenario(t, false)
			sc.APIFaults = nil
			complete := runWithPrefetch(t, context.Background(), false, sc, tc.options(t), nil)
			assertPrefetchFailsAsSerial(t, serial, prefetched, complete, tc.wantErr, tc.aheadOnly...)
		})
	}
}

// TestRunIntegrationPrefetchCanceledAsSerial: a run canceled (Ctrl-C) while
// emoji.list is under way ahead of its stage stops at the same point of the
// driver as the serial run: the refill after the first page, which does not
// start. The request under way is stopped, and the notices of its retries
// are dropped.
func TestRunIntegrationPrefetchCanceledAsSerial(t *testing.T) {
	t.Parallel()

	// The thread the first page fetches last: the stages take its result
	// just before the refill.
	lastThread := threadKey("C123", "1700000010.000000")
	run := func(prefetched bool) prefetchRun {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		arrived := make(chan struct{})
		var once sync.Once
		sc := threadsAcrossPagesScenario()
		sc.BeforeAPI = func(r *http.Request) {
			if r.URL.Path == "/api/emoji.list" {
				once.Do(func() { close(arrived) })
				<-r.Context().Done()
			}
		}
		return runWithPrefetch(t, ctx, prefetched, sc, threadsAcrossPagesOptions(t), func(key requestKey) {
			if key != lastThread {
				return
			}
			if prefetched {
				select {
				case <-arrived:
				case <-time.After(10 * time.Second):
					t.Errorf("emoji.list did not go ahead")
				}
			}
			cancel()
		})
	}
	serial, prefetched := run(false), run(true)
	complete := runWithPrefetch(t, context.Background(), false, threadsAcrossPagesScenario(), threadsAcrossPagesOptions(t), nil)
	assertPrefetchFailsAsSerial(t, serial, prefetched, complete, "slack api conversations.history: context canceled", "/api/emoji.list")
}

// TestRunIntegrationPrefetchTiming times exports on the clock of a synctest
// bubble, where each request takes delay at the server — a message's file
// fileDelay — and nothing else takes time, the pacing waits included. Without
// the prefetch, an export makes one request at a time: auth.test, team.info,
// conversations.list, conversations.history, each conversations.replies at
// least 1s after the previous one started, then each users.info, then
// emoji.list, then the downloads. With the Web API's (Issue #278),
// emoji.list runs alongside conversations.history, and the replies and the
// users.info run alongside each other from the history page on, the
// users.info of a user who shows only in a reply once the reply has come.
// The users.info, one second apart, take longest, so the Web API requests end
// with the last of them: the time to the end of the history page (4 delays),
// then a second for each users.info but the last, which takes a delay (the
// shape of decision log 0069's estimate). The downloads start once the Web
// API requests have ended, and take the longest of them, a file's or an
// avatar's. With the assets' too (Issue #279), the files download from the
// history page on, alongside the users.info, and each avatar once its
// users.info has returned: only the avatar of the last is left once the Web
// API requests have ended.
func TestRunIntegrationPrefetchTiming(t *testing.T) {
	t.Parallel()

	const delay, fileDelay = 100 * time.Millisecond, time.Second
	type timing struct {
		api   time.Duration // until the last Web API request ended
		total time.Duration // until Run returned
	}
	for _, tc := range []struct {
		name        string
		authors     int  // users who post on the page
		threads     int  // thread parents on the page
		newRepliers bool // whether the replies are by users who show nowhere else
		files       bool // whether each message on the page carries a file
	}{
		{name: "users on the page", authors: 5, threads: 3},
		{name: "users only in replies", authors: 1, threads: 3, newRepliers: true},
		{name: "files on the page", authors: 5, threads: 3, files: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			users := tc.authors
			if tc.newRepliers {
				users += tc.threads
			}
			// run times the export with the prefetch off, of the Web API
			// requests only, or of the downloads too.
			run := func(prefetched, assetsAhead bool) timing {
				var got timing
				synctest.Test(t, func(t *testing.T) {
					sc := timingScenario(tc.authors, tc.threads, tc.newRepliers, tc.files)
					fake := newFakeSlackHandler(t, &sc)
					h := &timedHandler{next: fake.mux, delay: delay, slow: map[string]time.Duration{}}
					for path := range sc.Assets {
						if strings.HasPrefix(path, "/files/doc-") {
							h.slow[path] = fileDelay
						}
					}
					srv := startPipeServer(h)
					tr := srv.transport()
					defer func() {
						tr.CloseIdleConnections()
						srv.stop()
					}()
					fake.base, fake.transport = "http://slack.test", tr
					sc.replaceBaseURL(fake.base)

					ctx := context.WithValue(context.Background(), assetPrefetchOffKey{}, !assetsAhead)
					start := time.Now()
					res := runWithPrefetchOn(t, ctx, prefetched, fake, integrationOptions(t, 10), nil, slack.WithSleeper(sleepFor))
					got.total = time.Since(start)
					if res.err != nil {
						t.Fatalf("Run() error = %v\nlogs:\n%s", res.err, strings.Join(res.Logs, "\n"))
					}
					if n := countPrefix(res.requests, "/api/users.info"); n != users {
						t.Fatalf("users.info requests = %d, want %d", n, users)
					}
					got.api = h.lastAPIEnd().Sub(start)
				})
				return got
			}
			serial, apiAhead, prefetched := run(false, false), run(true, false), run(true, true)
			t.Logf("serial: Web API %s, run %s; Web API ahead: Web API %s, run %s; downloads ahead too: Web API %s, run %s",
				serial.api, serial.total, apiAhead.api, apiAhead.total, prefetched.api, prefetched.total)

			usersInfo := time.Duration(users-1)*time.Second + delay
			if want := 4*delay + time.Duration(tc.threads-1)*time.Second + delay + usersInfo + delay; serial.api != want {
				t.Errorf("serial Web API time = %s, want %s", serial.api, want)
			}
			for name, r := range map[string]timing{"Web API ahead": apiAhead, "downloads ahead too": prefetched} {
				if want := 4*delay + usersInfo; r.api != want {
					t.Errorf("%s: Web API time = %s, want %s", name, r.api, want)
				}
			}
			// The downloads after the Web API requests.
			downloads := delay
			if tc.files {
				downloads = fileDelay
			}
			for name, r := range map[string]timing{"serial": serial, "Web API ahead": apiAhead} {
				if got := r.total - r.api; got != downloads {
					t.Errorf("%s: download time after the Web API = %s, want %s", name, got, downloads)
				}
			}
			if got := prefetched.total - prefetched.api; got != delay {
				t.Errorf("downloads ahead too: download time after the Web API = %s, want the last avatar's %s", got, delay)
			}
			if tc.files && prefetched.total >= apiAhead.total {
				t.Errorf("run with the downloads ahead too = %s, want it shorter than with the Web API ahead, %s", prefetched.total, apiAhead.total)
			}
		})
	}
}

// timingScenario is a page of messages by the first authors users in turn,
// one each or, with fewer authors than threads, one per thread. The oldest
// threads of them are thread parents, each with a reply: by a user of the
// page or, with newRepliers, by a user who shows nowhere else. Every user
// has an avatar, and with files, each message on the page a file.
func timingScenario(authors, threads int, newRepliers, files bool) exportScenario {
	sc := baseScenario()
	sc.Users = map[string]slack.User{}
	user := func(n int) string {
		id := fmt.Sprintf("U%02d", n)
		avatar := "/files/avatar-" + id + ".png"
		sc.Users[id] = testUser(id, "user"+id, "User "+id, "User "+id, "{{base}}"+avatar)
		sc.Assets[avatar] = pngAsset("avatar " + id)
		return id
	}
	for i := max(authors, threads); i >= 1; i-- { // newest first, as conversations.history returns them
		ts := fmt.Sprintf("17000000%02d.000000", i)
		m := slack.Message{Type: "message", TS: ts, User: user((i-1)%authors + 1), Text: fmt.Sprintf("message %d", i)}
		if files {
			doc := fmt.Sprintf("/files/doc-%d.pdf", i)
			m.Files = []slack.File{{ID: fmt.Sprintf("F%02d", i), Name: fmt.Sprintf("doc-%d.pdf", i), Mimetype: "application/pdf",
				Size: 5, URLPrivateDownload: "{{base}}" + doc}}
			sc.Assets[doc] = fakeAsset{ContentType: "application/pdf", Body: fmt.Sprintf("doc %d", i)}
		}
		if i <= threads {
			m.ThreadTS, m.ReplyCount = ts, 1
			replier := user(i%authors + 1)
			if newRepliers {
				replier = user(authors + i)
			}
			sc.Replies[ts] = []slack.Message{m, {
				Type: "message", TS: fmt.Sprintf("17000000%02d.100000", i), ThreadTS: ts, User: replier, Text: fmt.Sprintf("reply %d", i),
			}}
		}
		sc.Messages = append(sc.Messages, m)
	}
	return sc
}

// timedHandler answers each request through next once delay has passed, or
// for a path in slow, the time slow gives it, and records when the last Web
// API request was answered.
type timedHandler struct {
	next  http.Handler
	delay time.Duration
	slow  map[string]time.Duration

	mu  sync.Mutex
	end time.Time // when the last Web API request was answered
}

func (h *timedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d, ok := h.slow[r.URL.Path]
	if !ok {
		d = h.delay
	}
	time.Sleep(d)
	h.next.ServeHTTP(w, r)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		h.mu.Lock()
		if now := time.Now(); now.After(h.end) {
			h.end = now
		}
		h.mu.Unlock()
	}
}

func (h *timedHandler) lastAPIEnd() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.end
}

// sleepFor is the client's sleeper for the timing case: it waits d on the
// bubble's clock, or until ctx ends.
func sleepFor(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pipeServer serves an HTTP handler over in-memory connections (net.Pipe),
// on which a goroutine in a synctest bubble waits durably, unlike on a
// socket, as the one of internal/slack/methodlane_test.go does. Its transport
// dials every host to the server.
type pipeServer struct {
	srv   *http.Server
	conns chan net.Conn
	done  chan struct{}
	close sync.Once
}

func startPipeServer(h http.Handler) *pipeServer {
	s := &pipeServer{
		srv:   &http.Server{Handler: h, ErrorLog: log.New(io.Discard, "", 0)},
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}
	go s.srv.Serve(pipeListener{s})
	return s
}

func (s *pipeServer) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	select {
	case s.conns <- server:
		return client, nil
	case <-s.done:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *pipeServer) transport() *http.Transport {
	return &http.Transport{DialContext: s.dial, DialTLSContext: s.dial}
}

// stop closes the server and its connections, which ends the goroutines of
// the server and of the transports that dialed it.
func (s *pipeServer) stop() { s.srv.Close() }

type pipeListener struct{ s *pipeServer }

func (l pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.s.conns:
		return c, nil
	case <-l.s.done:
		return nil, net.ErrClosed
	}
}

func (l pipeListener) Close() error {
	l.s.close.Do(func() { close(l.s.done) })
	return nil
}

func (l pipeListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// twoThreadsScenario is a page of two thread parents, a message between them
// and a message by carol, who shows nowhere else, with a reply in each thread.
func twoThreadsScenario() exportScenario {
	const (
		firstTS  = "1700000004.000000"
		secondTS = "1700000002.000000"
	)
	first := slack.Message{Type: "message", TS: firstTS, ThreadTS: firstTS, User: "U01", Text: "first thread", ReplyCount: 1}
	second := slack.Message{Type: "message", TS: secondTS, ThreadTS: secondTS, User: "U02", Text: "second thread", ReplyCount: 1}

	sc := baseScenario()
	sc.Users["U03"] = testUser("U03", "carol", "Carol Example", "Carol", "")
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000005.000000", User: "U03", Text: "message by carol"},
		first,
		{Type: "message", TS: "1700000003.000000", User: "U01", Text: "message between the threads"},
		second,
	}
	sc.Replies[firstTS] = []slack.Message{first,
		{Type: "message", TS: "1700000004.100000", ThreadTS: firstTS, User: "U02", Text: "reply in the first thread"}}
	sc.Replies[secondTS] = []slack.Message{second,
		{Type: "message", TS: "1700000002.100000", ThreadTS: secondTS, User: "U01", Text: "reply in the second thread"}}
	return sc
}

// filteredReplyScenario is the happy path whose newest message and one reply
// the emoji filters of filteredReplyOptions exclude, as in
// TestRunIntegrationExcludeBodyEmojiReplyAndMaxPosts.
func filteredReplyScenario() exportScenario {
	sc := happyPathScenario()
	sc.Messages[0].Text = "private newest :do_not_archive:"
	sc.Replies["1700000002.000000"][2].Text = "private reply :speak_no_evil:"
	return sc
}

func filteredReplyOptions(t *testing.T) Options {
	opts := integrationOptions(t, 2)
	opts.ExcludeBodyEmoji = []string{"do_not_archive", "speak_no_evil"}
	return opts
}

// droppedThreadsScenario is three history pages, of two messages each, that
// one History call pages through, with the thread messages that
// droppedThreadsOptions' filter drops only once the page they are on has
// been handed over: the broadcast of a thread whose parent is excluded on the
// next page, and a parent whose thread conversations.replies excludes. Their
// authors show nowhere else, and neither goes ahead: nor does the thread of
// the broadcast. The replies of the thread that stays are by a user who
// shows nowhere else either, whom the Users stage resolves.
func droppedThreadsScenario() exportScenario {
	const (
		excludedTS = "1700000006.000000" // parent excluded on the second page
		repliesTS  = "1700000008.000000" // parent conversations.replies excludes
		keptTS     = "1700000005.000000" // parent of the thread that stays
	)
	thread := func(ts, user, text string) slack.Message {
		return slack.Message{Type: "message", TS: ts, ThreadTS: ts, User: user, Text: text, ReplyCount: 1}
	}
	excludedParent := thread(excludedTS, "U02", "parent excluded on the next page :shushing_face:")
	broadcast := slack.Message{Type: "message", Subtype: "thread_broadcast", TS: "1700000009.000000", ThreadTS: excludedTS, User: "U05", Text: "broadcast ahead of its excluded parent"}
	repliesParent := thread(repliesTS, "U04", "parent whose thread copy is excluded")
	repliesParentLater := repliesParent
	repliesParentLater.Text += " :shushing_face:"
	keptParent := thread(keptTS, "U01", "parent of the thread that stays")

	sc := baseScenario()
	sc.HistoryPageSize = 2
	for _, u := range []slack.User{
		testUser("U03", "carol", "Carol Example", "Carol", ""),
		testUser("U04", "dave", "Dave Example", "Dave", ""),
		testUser("U05", "erin", "Erin Example", "Erin", ""),
	} {
		sc.Users[u.ID] = u
	}
	sc.Messages = []slack.Message{
		broadcast,
		repliesParent,
		{Type: "message", TS: "1700000007.000000", User: "U01", Text: "message on the second page"},
		excludedParent,
		keptParent,
		{Type: "message", TS: "1700000004.000000", User: "U02", Text: "message on the third page"},
	}
	sc.Replies[excludedTS] = []slack.Message{excludedParent,
		{Type: "message", TS: "1700000006.100000", ThreadTS: excludedTS, User: "U05", Text: "reply in the excluded thread"}, broadcast}
	sc.Replies[repliesTS] = []slack.Message{repliesParentLater,
		{Type: "message", TS: "1700000008.100000", ThreadTS: repliesTS, User: "U04", Text: "reply in the thread excluded late"}}
	sc.Replies[keptTS] = []slack.Message{keptParent,
		{Type: "message", TS: "1700000005.100000", ThreadTS: keptTS, User: "U03", Text: "reply in the thread that stays"}}
	return sc
}

// droppedThreadsOptions: --max-posts 10 and the body emoji filter of
// droppedThreadsScenario.
func droppedThreadsOptions(t *testing.T) Options {
	opts := integrationOptions(t, 10)
	opts.ExcludeBodyEmoji = []string{"shushing_face"}
	return opts
}

// repliesOverCapScenario is a thread of maxThreadReplies + 1 replies, whose
// last, which the cap drops, is by a user who shows nowhere else.
func repliesOverCapScenario() exportScenario {
	const parentTS = "1700001200.000000"
	parent := slack.Message{Type: "message", TS: parentTS, ThreadTS: parentTS, User: "U01", Text: "Big thread", ReplyCount: maxThreadReplies + 1}
	sc := baseScenario()
	sc.Users["U03"] = testUser("U03", "carol", "Carol Example", "Carol", "")
	sc.Messages = []slack.Message{parent}
	replies := []slack.Message{parent}
	for i := 1; i <= maxThreadReplies+1; i++ {
		user := "U02"
		if i > maxThreadReplies {
			user = "U03"
		}
		replies = append(replies, slack.Message{
			Type: "message", TS: fmt.Sprintf("1700001200.%06d", i), ThreadTS: parentTS, User: user, Text: fmt.Sprintf("reply %d", i),
		})
	}
	sc.Replies[parentTS] = replies
	return sc
}

// peopleScenario shows users and bots that users.info and bots.info resolve
// after a rate limit or a server error, or never: an unknown user mentioned,
// a user whose users.info keeps failing, and a bot unknown to bots.info. The
// one user of the thread's reply shows nowhere else.
func peopleScenario() exportScenario {
	const threadTS = "1700000006.000000"
	parent := slack.Message{Type: "message", TS: threadTS, ThreadTS: threadTS, User: "U01", Text: "thread with a reply by dave", ReplyCount: 1}

	sc := baseScenario()
	sc.Users["U03"] = testUser("U03", "carol", "Carol Example", "Carol", "")
	sc.Users["U04"] = testUser("U04", "dave", "Dave Example", "Dave", "")
	sc.Users["U05"] = testUser("U05", "erin", "Erin Example", "Erin", "")
	sc.Bots = map[string]slack.Bot{"B01": {ID: "B01", Name: "Deploy Bot", Icons: botIcons("{{base}}/files/bot-b01.png")}}
	sc.Assets["/files/bot-b01.png"] = pngAsset("bot-b01")
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000008.000000", User: "U01", Text: "hello <@U404> and <@U05>"},
		{Type: "message", Subtype: "bot_message", TS: "1700000007.000000", BotID: "B404", Text: "bot unknown to bots.info"},
		parent,
		{Type: "message", Subtype: "bot_message", TS: "1700000005.000000", BotID: "B01", Text: "bot that bots.info resolves"},
		{Type: "message", TS: "1700000004.000000", User: "U02", Text: "message by a rate limited user"},
		{Type: "message", TS: "1700000003.000000", User: "U03", Text: "message by a user behind a server error"},
	}
	sc.Replies[threadTS] = []slack.Message{parent,
		{Type: "message", TS: "1700000006.100000", ThreadTS: threadTS, User: "U04", Text: "reply by dave"}}
	rateLimited := func() *endpointFault {
		return &endpointFault{transient: []faultResponse{{httpStatus: http.StatusTooManyRequests, retryAfterSec: 1}}}
	}
	sc.APIFaults = map[string]*endpointFault{
		"/api/users.info?user=U02": rateLimited(),
		"/api/users.info?user=U03": {transient: []faultResponse{{httpStatus: http.StatusInternalServerError}}},
		"/api/users.info?user=U05": {sticky: &faultResponse{httpStatus: http.StatusInternalServerError}},
		"/api/bots.info?bot=B01":   rateLimited(),
	}
	return sc
}
