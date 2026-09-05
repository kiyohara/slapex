package export

// Integration cases for the export as a whole (v1-07, Issue #21) plus the
// emoji-filter (Issue #155 / #156) and fetch-range (Issue #153 / #154) cases
// that build on the same happy-path fixture. Shared test infrastructure lives
// in the files named for it:
//
//   - integration_fixture_test.go:    exportScenario, happyPathScenario, baseScenario
//   - integration_fakeserver_test.go: the fake Slack server and its request counts
//   - integration_harness_test.go:    runExportScenario / runExportScenarioRaw, integrationOptions
//   - integration_assert_test.go:     readIndexHTML, mustContain, manifest / log helpers
//
// The helpers kept here encode this file's own expectations: the happy-path
// output / HTML / cache assertions and the emoji-filter metadata checks.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// --- happy path ---------------------------------------------------------------

func TestRunIntegrationHappyPath(t *testing.T) {
	t.Parallel()

	got := runExportScenario(t, happyPathScenario(), integrationOptions(t, 10))

	assertOutputFiles(t, got.OutputDir)
	assertHTMLMarkers(t, filepath.Join(got.OutputDir, "index.html"))
	assertCacheFiles(t, got.OutputDir)
	assertEndpointCounts(t, got.Server, map[string]int{
		"/api/auth.test":             1,
		"/api/team.info":             1,
		"/api/conversations.list":    1,
		"/api/conversations.history": 1,
		"/api/conversations.replies": 1,
		"/api/users.info":            2,
		"/api/bots.info":             0,
		"/api/emoji.list":            1,
	})
}

// TestRunIntegrationPhaseOrder is a characterization test for the stage
// sequence Run reports on a normal successful export. Each phase closes with
// one plain-mode "OK: <label>: ..." line, and the labels must complete in the
// order Workspace → Channel → Messages → Users → Emoji → Assets → Done. Only
// the label sequence is pinned — not the phase text, counts, paths or times —
// so a later reorganisation of Run's stages (Issue #191) is checked against
// the observable order without a brittle whole-log snapshot. Individual phase
// lines keep their own assertions in the cases that care about them.
//
// Run passes the labels capitalised ("Workspace", ...); the lowercase values
// below are what ui.Printer.EndPhase prints in plain mode, which lowercases the
// label. The test reads that plain-mode output, hence the lowercase list.
func TestRunIntegrationPhaseOrder(t *testing.T) {
	t.Parallel()

	got := runExportScenario(t, happyPathScenario(), integrationOptions(t, 10))

	// Plain-mode labels: lowercase, as EndPhase prints them.
	want := []string{"workspace", "channel", "messages", "users", "emoji", "assets", "done"}
	isPhase := map[string]bool{}
	for _, label := range want {
		isPhase[label] = true
	}
	var completed []string
	for _, line := range got.Logs {
		rest, ok := strings.CutPrefix(line, "OK: ")
		if !ok {
			continue
		}
		label, _, ok := strings.Cut(rest, ": ")
		if ok && isPhase[label] {
			completed = append(completed, label)
		}
	}
	if !slices.Equal(completed, want) {
		t.Fatalf("phase completion order = %v, want %v\nlogs:\n%s", completed, want, strings.Join(got.Logs, "\n"))
	}
}

// TestRunIntegrationAssetExtensionFromContent covers the gravatar shape end to
// end: an avatar URL whose path ends in .jpg but whose bytes are a PNG, because
// gravatar redirects to the PNG default image. The saved file takes its
// extension from the content, and the manifest mimetype agrees with it
// (Issue #183). The fixture's other assets keep their extensions, since their
// bodies are plain strings that cannot be sniffed.
func TestRunIntegrationAssetExtensionFromContent(t *testing.T) {
	t.Parallel()

	const gravatarPath = "/files/avatar-gravatar.jpg"
	// The shape Slack's users.info returns for a gravatar user: a .jpg path with
	// the Slack default image as the d= fallback. The hash is a placeholder.
	const gravatarQuery = "?s=72&d=https%3A%2F%2Fexample.com%2Fdefault-72.png"

	sc := happyPathScenario()
	sc.Users["U01"] = testUser("U01", "alice", "Alice Example", "Alice", "{{base}}"+gravatarPath+gravatarQuery)
	sc.Assets[gravatarPath] = fakeAsset{ContentType: "image/png", Body: "\x89PNG\r\n\x1a\nfake png body"}

	got := runExportScenario(t, sc, integrationOptions(t, 10))

	var checked int
	for _, asset := range readManifestEntries(t, got.OutputDir) {
		if asset.Status != "saved" {
			continue
		}
		switch {
		case strings.Contains(asset.SourceURL, gravatarPath):
			if filepath.Ext(asset.LocalPath) != ".png" || asset.Mimetype != "image/png" {
				t.Fatalf("gravatar avatar = local_path:%q mimetype:%q, want a .png path and image/png", asset.LocalPath, asset.Mimetype)
			}
			if _, err := os.Stat(filepath.Join(got.OutputDir, filepath.FromSlash(asset.LocalPath))); err != nil {
				t.Fatalf("gravatar avatar %q missing: %v", asset.LocalPath, err)
			}
			checked++
		case strings.Contains(asset.SourceURL, "/files/runbook.pdf"):
			// Unchanged: the body is a plain string, so the sniff says nothing and
			// the original display name still decides the extension.
			if filepath.Ext(asset.LocalPath) != ".pdf" {
				t.Fatalf("attachment local_path = %q, want a .pdf path", asset.LocalPath)
			}
			checked++
		}
	}
	if checked != 2 {
		t.Fatalf("checked %d manifest assets, want 2 (gravatar avatar and attachment)", checked)
	}
}

// --- emoji filters (--exclude-body-emoji / --exclude-reaction-emoji) ----------

// TestRunIntegrationExcludeEmojiParentAndThread: marking the thread parent of
// the happy-path fixture for exclusion — by a body shortcode or by a reaction —
// drops the parent and its whole thread from the HTML, skips the replies fetch,
// records exactly one excluded message under the right option name, and keeps
// the excluded content out of every cache file. The two filters share every
// expectation except how the message is marked and which option / summary
// label names it, so they run as named subtests over one fixture shape.
func TestRunIntegrationExcludeEmojiParentAndThread(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		mark          func(parent *slack.Message) // marks Messages[1], the thread parent
		apply         func(opts *Options)
		optionsLine   string
		bodyNames     []string
		reactionNames []string
		summaryLabel  string // the one summary label the run must print
		otherLabel    string // the single-filter label it must not print
	}{
		{
			name:          "body",
			mark:          func(parent *slack.Message) { parent.Text += " :shushing_face:" },
			apply:         func(opts *Options) { opts.ExcludeBodyEmoji = []string{"shushing_face"} },
			optionsLine:   "--exclude-body-emoji shushing_face",
			bodyNames:     []string{"shushing_face"},
			reactionNames: nil,
			summaryLabel:  "excluded by body emoji: 1",
			otherLabel:    "excluded by reaction emoji",
		},
		{
			name: "reaction",
			mark: func(parent *slack.Message) {
				parent.Reactions = append(parent.Reactions, slack.Reaction{Name: "speak_no_evil", Count: 1})
			},
			apply:         func(opts *Options) { opts.ExcludeReactionEmoji = []string{"speak_no_evil"} },
			optionsLine:   "--exclude-reaction-emoji speak_no_evil",
			bodyNames:     nil,
			reactionNames: []string{"speak_no_evil"},
			summaryLabel:  "excluded by reaction emoji: 1",
			otherLabel:    "excluded by body emoji",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sc := happyPathScenario()
			tc.mark(&sc.Messages[1])
			opts := integrationOptions(t, 10)
			tc.apply(&opts)

			got := runExportScenario(t, sc, opts)
			body := readIndexHTML(t, got.OutputDir)

			for _, excluded := range []string{"Starting the launch thread", "Reply with screenshot", "Thread is wrapped up"} {
				mustNotContain(t, body, excluded)
			}
			mustContain(t, body, tc.optionsLine)
			assertEndpointCounts(t, got.Server, map[string]int{"/api/conversations.replies": 0})
			assertExcludedMetadata(t, got.OutputDir, 2, 0, 0, 1, tc.bodyNames, tc.reactionNames)
			assertCacheOmits(t, got.OutputDir, "U01", "screenshot-original.png", "og-launch.png")
			if !logsContain(got.Logs, tc.summaryLabel) {
				t.Fatalf("summary missing %q: %v", tc.summaryLabel, got.Logs)
			}
			if logsContain(got.Logs, tc.otherLabel) {
				t.Fatalf("%s-only logs contain the other filter's label %q: %v", tc.name, tc.otherLabel, got.Logs)
			}
		})
	}
}

// TestRunIntegrationExcludeEmojiParentDropsBroadcastAndRefillsMaxPosts: when
// the newest thread parent is excluded, its thread_broadcast copy on the
// timeline goes with it, and --max-posts 2 is refilled from the next history
// page so two retained messages still render. Body and reaction marking share
// the expectation, so both run as named subtests.
func TestRunIntegrationExcludeEmojiParentDropsBroadcastAndRefillsMaxPosts(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		parentText    string
		reactions     []slack.Reaction
		apply         func(opts *Options)
		bodyNames     []string
		reactionNames []string
	}{
		{
			name:       "body",
			parentText: "private parent :shushing_face:",
			apply:      func(opts *Options) { opts.ExcludeBodyEmoji = []string{"shushing_face"} },
			bodyNames:  []string{"shushing_face"},
		},
		{
			name:          "reaction",
			parentText:    "private parent",
			reactions:     []slack.Reaction{{Name: "speak_no_evil", Count: 1}},
			apply:         func(opts *Options) { opts.ExcludeReactionEmoji = []string{"speak_no_evil"} },
			reactionNames: []string{"speak_no_evil"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			const parentTS = "1700000003.000000"
			parent := slack.Message{
				Type:       "message",
				TS:         parentTS,
				ThreadTS:   parentTS,
				User:       "U01",
				Text:       tc.parentText,
				ReplyCount: 1,
				Reactions:  tc.reactions,
			}
			broadcast := slack.Message{
				Type:     "message",
				Subtype:  "thread_broadcast",
				TS:       "1700000004.000000",
				ThreadTS: parentTS,
				User:     "U02",
				Text:     "private broadcast",
			}
			sc := baseScenario()
			sc.Messages = []slack.Message{
				broadcast,
				parent,
				{Type: "message", TS: "1700000002.000000", User: "U01", Text: "retained newer"},
				{Type: "message", TS: "1700000001.000000", User: "U02", Text: "retained older"},
			}
			sc.Replies[parentTS] = []slack.Message{parent, broadcast}
			opts := integrationOptions(t, 2)
			tc.apply(&opts)

			got := runExportScenario(t, sc, opts)
			body := readIndexHTML(t, got.OutputDir)

			for _, excluded := range []string{"private parent", "private broadcast"} {
				mustNotContain(t, body, excluded)
			}
			for _, retained := range []string{"retained newer", "retained older"} {
				mustContain(t, body, retained)
			}
			assertEndpointCounts(t, got.Server, map[string]int{
				"/api/conversations.history": 2,
				"/api/conversations.replies": 1,
			})
			assertExcludedMetadata(t, got.OutputDir, 2, 0, 0, 2, tc.bodyNames, tc.reactionNames)
			assertCacheOmits(t, got.OutputDir, "private parent", "private broadcast")
		})
	}
}

func TestRunIntegrationThreadProgressAdvancesWhenRepliesExcluded(t *testing.T) {
	t.Parallel()

	const (
		firstTS  = "1700000002.000000"
		secondTS = "1700000001.000000"
	)
	first := slack.Message{Type: "message", TS: firstTS, ThreadTS: firstTS, User: "U01", Text: "first parent", ReplyCount: 1}
	second := slack.Message{Type: "message", TS: secondTS, ThreadTS: secondTS, User: "U02", Text: "second parent", ReplyCount: 1}
	sc := baseScenario()
	sc.Messages = []slack.Message{first, second}
	sc.Replies[firstTS] = []slack.Message{
		first,
		{Type: "message", TS: "1700000002.100000", ThreadTS: firstTS, User: "U02", Text: "excluded :shushing_face:"},
	}
	sc.Replies[secondTS] = []slack.Message{
		second,
		{Type: "message", TS: "1700000001.100000", ThreadTS: secondTS, User: "U01", Text: "retained reply"},
	}
	opts := renderingOptions(t)
	opts.ExcludeBodyEmoji = []string{"shushing_face"}

	got := runExportScenario(t, sc, opts)
	if !logsContain(got.Logs, "fetching thread replies ... 1/2") || !logsContain(got.Logs, "fetching thread replies ... 2/2") {
		t.Fatalf("thread progress did not advance monotonically: %v", got.Logs)
	}
}

func TestRunIntegrationExcludeBodyEmojiReplyAndMaxPosts(t *testing.T) {
	t.Parallel()

	sc := happyPathScenario()
	sc.Messages[0].Text = "private newest :do_not_archive:"
	sc.Replies["1700000002.000000"][2].Text = "private reply :speak_no_evil:"
	opts := integrationOptions(t, 2)
	opts.ExcludeBodyEmoji = []string{"do_not_archive", "speak_no_evil"}

	got := runExportScenario(t, sc, opts)
	body := readIndexHTML(t, got.OutputDir)

	for _, excluded := range []string{"private newest", "private reply", "screenshot.png"} {
		mustNotContain(t, body, excluded)
	}
	for _, included := range []string{"Starting the launch thread", "First timeline note", "Thread is wrapped up"} {
		mustContain(t, body, included)
	}
	assertExcludedMetadata(t, got.OutputDir, 2, 1, 1, 2, []string{"do_not_archive", "speak_no_evil"}, nil)
	assertCacheOmits(t, got.OutputDir, "screenshot-original.png", "screenshot-thumb.png")
}

func TestRunIntegrationExcludeBodyEmojiHidesEmptyThread(t *testing.T) {
	t.Parallel()

	sc := happyPathScenario()
	sc.Replies["1700000002.000000"][1].Text += " :speak_no_evil:"
	sc.Replies["1700000002.000000"][2].Text += " :speak_no_evil:"
	opts := integrationOptions(t, 10)
	opts.ExcludeBodyEmoji = []string{"speak_no_evil"}

	got := runExportScenario(t, sc, opts)
	body := readIndexHTML(t, got.OutputDir)

	mustContain(t, body, "Starting the launch thread")
	mustNotContain(t, body, `summary class="thread-label"`)
	assertExcludedMetadata(t, got.OutputDir, 3, 0, 0, 2, []string{"speak_no_evil"}, nil)
}

func TestRunIntegrationEmojiFiltersORReplyCustomAndMaxPosts(t *testing.T) {
	t.Parallel()

	sc := happyPathScenario()
	sc.Messages[0].Text += " :speak_no_evil:"
	sc.Messages[0].Reactions = []slack.Reaction{{Name: "do_not_archive", Count: 1}}
	sc.Replies["1700000002.000000"][2].Reactions = []slack.Reaction{{Name: "shushing_face", Count: 1}}
	opts := integrationOptions(t, 2)
	opts.ExcludeBodyEmoji = []string{"speak_no_evil"}
	opts.ExcludeReactionEmoji = []string{"do_not_archive", "shushing_face"}

	got := runExportScenario(t, sc, opts)
	body := readIndexHTML(t, got.OutputDir)

	for _, excluded := range []string{"Final timeline update", "Reply with screenshot", "screenshot.png"} {
		mustNotContain(t, body, excluded)
	}
	for _, retained := range []string{"Starting the launch thread", "First timeline note", "Thread is wrapped up"} {
		mustContain(t, body, retained)
	}
	mustContain(t, body, "--exclude-body-emoji speak_no_evil")
	mustContain(t, body, "--exclude-reaction-emoji do_not_archive,shushing_face")
	assertExcludedMetadata(t, got.OutputDir, 2, 1, 1, 2, []string{"speak_no_evil"}, []string{"do_not_archive", "shushing_face"})
	assertCacheOmits(t, got.OutputDir, "screenshot-original.png", "screenshot-thumb.png")
	if !logsContain(got.Logs, "excluded by emoji filters: 2") {
		t.Fatalf("summary missing combined exclusion count: %v", got.Logs)
	}
	for _, wrong := range []string{"excluded by body emoji", "excluded by reaction emoji"} {
		if logsContain(got.Logs, wrong) {
			t.Fatalf("combined-filter logs contain the single-filter label %q: %v", wrong, got.Logs)
		}
	}
}

// assertCacheOmits fails if any of the values (excluded-only text, user IDs or
// asset names) leaked into one of the three .cache/ files.
func assertCacheOmits(t *testing.T, dir string, values ...string) {
	t.Helper()
	for _, name := range []string{"metadata.json", "assets_manifest.json", "slack_api_cache.json"} {
		body, err := os.ReadFile(filepath.Join(dir, ".cache", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range values {
			if strings.Contains(string(body), value) {
				t.Fatalf("%s contains excluded-only value %q", name, value)
			}
		}
	}
}

// assertExcludedMetadata checks metadata.json's counts and the recorded
// exclude_body_emoji / exclude_reaction_emoji option values.
func assertExcludedMetadata(t *testing.T, dir string, timeline, threads, replies, excluded int, bodyNames, reactionNames []string) {
	t.Helper()
	var metadata struct {
		Counts struct {
			Timeline int `json:"timeline_messages"`
			Threads  int `json:"threads"`
			Replies  int `json:"replies"`
			Excluded int `json:"excluded_messages"`
		} `json:"counts"`
		Fetch struct {
			Options struct {
				ExcludeBodyEmoji     []string `json:"exclude_body_emoji"`
				ExcludeReactionEmoji []string `json:"exclude_reaction_emoji"`
			} `json:"options"`
		} `json:"fetch"`
		Users map[string]any `json:"users"`
	}
	readJSON(t, filepath.Join(dir, ".cache/metadata.json"), &metadata)
	if metadata.Counts.Timeline != timeline || metadata.Counts.Threads != threads || metadata.Counts.Replies != replies || metadata.Counts.Excluded != excluded {
		t.Fatalf("metadata counts = %+v, want timeline=%d threads=%d replies=%d excluded=%d", metadata.Counts, timeline, threads, replies, excluded)
	}
	if !slices.Equal(metadata.Fetch.Options.ExcludeBodyEmoji, bodyNames) {
		t.Fatalf("exclude_body_emoji = %v, want %v", metadata.Fetch.Options.ExcludeBodyEmoji, bodyNames)
	}
	if !slices.Equal(metadata.Fetch.Options.ExcludeReactionEmoji, reactionNames) {
		t.Fatalf("exclude_reaction_emoji = %v, want %v", metadata.Fetch.Options.ExcludeReactionEmoji, reactionNames)
	}
}

// --- fetch range (--date / --from / --to) -------------------------------------

func TestRunIntegrationDateRange(t *testing.T) {
	t.Parallel()

	offsetInput := "2026-07-03T23:30:15-07:00"
	offsetInstant, err := time.Parse(time.RFC3339, offsetInput)
	if err != nil {
		t.Fatal(err)
	}
	offsetLocal := offsetInstant.In(time.Local)
	offsetStart := time.Date(offsetLocal.Year(), offsetLocal.Month(), offsetLocal.Day(), 0, 0, 0, 0, time.Local)

	for _, tt := range []struct {
		name  string
		input string
		start time.Time
	}{
		{name: "loose local input", input: "2026/07/03 09:30", start: time.Date(2026, 7, 3, 0, 0, 0, 0, time.Local)},
		{name: "offset input", input: offsetInput, start: offsetStart},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertDateRangeExport(t, tt.input, tt.start)
		})
	}
}

func assertDateRangeExport(t *testing.T, input string, start time.Time) {
	t.Helper()

	sc := happyPathScenario()
	sc.Messages[0].TS = slack.FormatTS(start.AddDate(0, 0, 1).Unix())
	sc.Messages[1].TS = slack.FormatTS(start.Add(2 * time.Second).Unix())
	sc.Messages[1].ThreadTS = sc.Messages[1].TS
	sc.Messages[2].TS = slack.FormatTS(start.Unix())
	oldReplies := sc.Replies["1700000002.000000"]
	delete(sc.Replies, "1700000002.000000")
	for i := range oldReplies {
		oldReplies[i].ThreadTS = sc.Messages[1].TS
	}
	oldReplies[0].TS = sc.Messages[1].TS
	sc.Replies[sc.Messages[1].TS] = oldReplies

	opts := integrationOptions(t, 2)
	opts.Days = 0 // --date replaces the --days window; metadata records days as given
	opts.Date = input

	got := runExportScenario(t, sc, opts)
	html := readIndexHTML(t, got.OutputDir)

	displayTimezone := environmentRangeDisplayTimezone(time.Local)
	for _, want := range []string{
		"First timeline note",
		"Starting the launch thread",
		escapePlusForHTMLAssertion(fmt.Sprintf(
			"From %s (included); to %s (not included); timezone: %s",
			start.In(displayTimezone.location).Format(time.RFC3339),
			start.AddDate(0, 0, 1).In(displayTimezone.location).Format(time.RFC3339),
			displayTimezone.label,
		)),
		escapePlusForHTMLAssertion("<dt>Options</dt><dd>--date &#34;" + input + "&#34;"),
	} {
		mustContain(t, html, want)
	}
	mustNotContain(t, html, "Final timeline update") // the exclusive end boundary

	var metadata struct {
		Fetch struct {
			TargetRange struct {
				Start        string `json:"start"`
				End          string `json:"end"`
				StartSlackTS string `json:"start_slack_ts"`
				EndSlackTS   string `json:"end_slack_ts"`
			} `json:"target_range"`
			Options struct {
				RangeMode string `json:"range_mode"`
				Date      string `json:"date"`
				MaxPosts  int    `json:"max_posts"`
			} `json:"options"`
		} `json:"fetch"`
		Counts struct {
			TimelineMessages int `json:"timeline_messages"`
			Replies          int `json:"replies"`
		} `json:"counts"`
	}
	readJSON(t, filepath.Join(got.OutputDir, ".cache/metadata.json"), &metadata)
	if metadata.Fetch.Options.RangeMode != "date" || metadata.Fetch.Options.Date != input || metadata.Fetch.Options.MaxPosts != 2 {
		t.Fatalf("metadata fetch = %+v", metadata.Fetch)
	}
	wantEnd := start.AddDate(0, 0, 1)
	if metadata.Fetch.TargetRange.Start != start.UTC().Format(time.RFC3339) || metadata.Fetch.TargetRange.End != wantEnd.UTC().Format(time.RFC3339) ||
		metadata.Fetch.TargetRange.StartSlackTS != slack.FormatTS(start.Unix()) || metadata.Fetch.TargetRange.EndSlackTS != slack.FormatTS(wantEnd.Unix()) {
		t.Fatalf("metadata target range = %+v", metadata.Fetch.TargetRange)
	}
	if metadata.Counts.TimelineMessages != 2 || metadata.Counts.Replies != 2 {
		t.Fatalf("metadata counts = %+v, want 2 timeline / 2 replies", metadata.Counts)
	}
}

func TestRunIntegrationDateTimeRange(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 3, 9, 30, 0, 0, time.Local)
	end := start.Add(30 * time.Minute)
	sc := happyPathScenario()
	sc.Messages[0].TS = slack.FormatTS(end.Unix())
	sc.Messages[1].TS = slack.FormatTS(start.Add(2 * time.Second).Unix())
	sc.Messages[1].ThreadTS = sc.Messages[1].TS
	sc.Messages[2].TS = slack.FormatTS(start.Unix())
	oldReplies := sc.Replies["1700000002.000000"]
	delete(sc.Replies, "1700000002.000000")
	for i := range oldReplies {
		oldReplies[i].ThreadTS = sc.Messages[1].TS
	}
	oldReplies[0].TS = sc.Messages[1].TS
	sc.Replies[sc.Messages[1].TS] = oldReplies

	fromInput := "2026-07-03T09:30"
	toInput := end.Format(time.RFC3339)
	opts := integrationOptions(t, 2)
	opts.Days = 0 // --from / --to replace the --days window; metadata records days as given
	opts.From = fromInput
	opts.To = toInput

	got := runExportScenario(t, sc, opts)
	html := readIndexHTML(t, got.OutputDir)

	displayTimezone := chooseDateTimeRangeDisplayTimezone(fromInput, toInput, time.Local)
	for _, want := range []string{
		"First timeline note",
		"Starting the launch thread",
		escapePlusForHTMLAssertion(fmt.Sprintf(
			"From %s (included); to %s (not included); timezone: %s",
			start.In(displayTimezone.location).Format(time.RFC3339),
			end.In(displayTimezone.location).Format(time.RFC3339),
			displayTimezone.label,
		)),
		escapePlusForHTMLAssertion("<dt>Options</dt><dd>--from &#34;" + fromInput + "&#34;, --to &#34;" + toInput + "&#34;"),
	} {
		mustContain(t, html, want)
	}
	mustNotContain(t, html, "Final timeline update") // the exclusive end boundary

	var metadata struct {
		Fetch struct {
			OldestTS    string `json:"oldest_ts"`
			LatestTS    string `json:"latest_ts"`
			TargetRange struct {
				Start        string `json:"start"`
				End          string `json:"end"`
				StartSlackTS string `json:"start_slack_ts"`
				EndSlackTS   string `json:"end_slack_ts"`
			} `json:"target_range"`
			Options struct {
				RangeMode string `json:"range_mode"`
				From      string `json:"from"`
				To        string `json:"to"`
				MaxPosts  int    `json:"max_posts"`
			} `json:"options"`
		} `json:"fetch"`
		Counts struct {
			TimelineMessages int `json:"timeline_messages"`
			Replies          int `json:"replies"`
		} `json:"counts"`
	}
	readJSON(t, filepath.Join(got.OutputDir, ".cache/metadata.json"), &metadata)
	if metadata.Fetch.Options.RangeMode != "datetime-range" || metadata.Fetch.Options.From != fromInput || metadata.Fetch.Options.To != toInput || metadata.Fetch.Options.MaxPosts != 2 {
		t.Fatalf("metadata fetch = %+v", metadata.Fetch)
	}
	wantOldest := slack.FormatTS(start.Unix())
	wantLatest := slack.FormatTS(end.Unix())
	if metadata.Fetch.OldestTS != wantOldest || metadata.Fetch.LatestTS != wantLatest ||
		metadata.Fetch.TargetRange.Start != start.UTC().Format(time.RFC3339) || metadata.Fetch.TargetRange.End != end.UTC().Format(time.RFC3339) ||
		metadata.Fetch.TargetRange.StartSlackTS != wantOldest || metadata.Fetch.TargetRange.EndSlackTS != wantLatest {
		t.Fatalf("metadata target range = %+v", metadata.Fetch.TargetRange)
	}
	if metadata.Counts.TimelineMessages != 2 || metadata.Counts.Replies != 2 {
		t.Fatalf("metadata counts = %+v, want 2 timeline / 2 replies", metadata.Counts)
	}
}

func escapePlusForHTMLAssertion(value string) string {
	return strings.ReplaceAll(value, "+", "&#43;")
}

// --- happy-path expectations --------------------------------------------------

// assertOutputFiles checks that every output path the happy-path fixture must
// produce exists (one directory per asset kind, the three .cache/ files).
func assertOutputFiles(t *testing.T, dir string) {
	t.Helper()

	for _, rel := range []string{
		"index.html",
		"style.css",
		"assets/slapex-logo.svg",
		"assets/workspace-icons",
		"assets/avatars",
		"assets/emoji",
		"assets/service-icons",
		"assets/og-images",
		"assets/uploads/thumbs",
		"assets/uploads/originals",
		"assets/attachments",
		".cache/metadata.json",
		".cache/assets_manifest.json",
		".cache/slack_api_cache.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("expected output path %s: %v", rel, err)
		}
	}
}

// assertHTMLMarkers checks the happy-path index.html for the header, every
// message, the resolved mention, reactions, the date divider, each asset
// directory reference and the timeline / thread order.
func assertHTMLMarkers(t *testing.T, htmlPath string) {
	t.Helper()

	data, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	body := string(data)
	dateDivider := fmt.Sprintf(`<div class="date-divider"><span>%s</span></div>`,
		tsTime("1700000001.000000").Format("2006-01-02"))
	for _, marker := range []string{
		"Acme Workspace (acme.example.slack.com, TACME123)",
		"#project-alpha (C123, public, active, member)",
		`<a class="title-link workspace-name" href="https://acme.example.slack.com/" target="_blank" rel="noopener noreferrer">Acme Workspace</a><a class="title-link channel-title" href="https://acme.example.slack.com/archives/C123" target="_blank" rel="noopener noreferrer"><span class="channel-hash">#</span>project-alpha</a>`,
		"First timeline note",
		"Starting the launch thread with",
		"Bob",
		`<div class="thread">`,
		"Reply with screenshot",
		"Thread is wrapped up",
		`<span class="reaction-count">3</span>`,
		`<span class="reaction-count">2</span>`,
		dateDivider,
		`assets/avatars/`,
		`assets/workspace-icons/`,
		`assets/emoji/`,
		`assets/service-icons/`,
		`assets/og-images/`,
		`assets/uploads/thumbs/`,
		`assets/uploads/originals/`,
		`assets/attachments/`,
		"runbook.pdf",
		"Launch checklist",
		`<dt>Options</dt>`,
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("index.html missing marker %q", marker)
		}
	}

	assertOrder(t, body,
		"First timeline note",
		"Starting the launch thread",
		"Reply with screenshot",
		"Thread is wrapped up",
		"Final timeline update",
	)
	if !strings.Contains(body, `Read <span class="mention">@Bob</span>'s notes`) {
		t.Fatalf("index.html missing resolved unfurl text")
	}
}

// assertCacheFiles checks the happy-path .cache/ files: workspace / channel
// identity and counts in metadata.json, one saved asset of every kind in
// assets_manifest.json, and the resolved users / emoji in slack_api_cache.json.
func assertCacheFiles(t *testing.T, dir string) {
	t.Helper()

	var metadata struct {
		Workspace struct {
			TeamID string `json:"team_id"`
			Name   string `json:"name"`
		} `json:"workspace"`
		Channel struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"channel"`
		Counts struct {
			TimelineMessages int `json:"timeline_messages"`
			Threads          int `json:"threads"`
			Replies          int `json:"replies"`
			AssetsSaved      int `json:"assets_saved"`
		} `json:"counts"`
		Fetch struct {
			TargetRange struct {
				Start string  `json:"start"`
				End   *string `json:"end"`
			} `json:"target_range"`
			Options struct {
				RangeMode string `json:"range_mode"`
				Days      int    `json:"days"`
			} `json:"options"`
		} `json:"fetch"`
	}
	readJSON(t, filepath.Join(dir, ".cache/metadata.json"), &metadata)
	if metadata.Workspace.TeamID != "TACME123" || metadata.Workspace.Name != "Acme Workspace" {
		t.Fatalf("metadata workspace = %+v, want Acme Workspace/TACME123", metadata.Workspace)
	}
	if metadata.Channel.ID != "C123" || metadata.Channel.Name != "project-alpha" {
		t.Fatalf("metadata channel = %+v, want project-alpha/C123", metadata.Channel)
	}
	if metadata.Counts.TimelineMessages != 3 || metadata.Counts.Threads != 1 || metadata.Counts.Replies != 2 {
		t.Fatalf("metadata counts = %+v, want 3 timeline / 1 thread / 2 replies", metadata.Counts)
	}
	if metadata.Counts.AssetsSaved < 8 {
		t.Fatalf("metadata assets_saved = %d, want at least 8", metadata.Counts.AssetsSaved)
	}
	if metadata.Fetch.TargetRange.Start == "" || metadata.Fetch.TargetRange.End == nil || *metadata.Fetch.TargetRange.End == "" || metadata.Fetch.Options.RangeMode != "days" || metadata.Fetch.Options.Days != 90 {
		t.Fatalf("metadata fetch = %+v, want absolute start/end and --days 90", metadata.Fetch)
	}

	manifest := readManifestEntries(t, dir)
	for _, kind := range []string{"workspace_icon", "avatar", "emoji", "service_icon", "og_image", "upload_thumb", "upload_original", "attachment"} {
		if !hasSavedAsset(manifest, kind) {
			t.Fatalf("assets_manifest.json missing saved %s asset: %+v", kind, manifest)
		}
	}

	var apiCache struct {
		Users map[string]struct {
			DisplayName string `json:"display_name"`
		} `json:"users"`
		Emoji map[string]string `json:"emoji"`
	}
	readJSON(t, filepath.Join(dir, ".cache/slack_api_cache.json"), &apiCache)
	if apiCache.Users["U01"].DisplayName != "Alice" || apiCache.Users["U02"].DisplayName != "Bob" {
		t.Fatalf("slack_api_cache users = %+v, want U01/Alice and U02/Bob", apiCache.Users)
	}
	if !strings.Contains(apiCache.Emoji["party_sloth"], "/files/emoji-party-sloth.png") {
		t.Fatalf("slack_api_cache emoji = %+v, want party_sloth URL", apiCache.Emoji)
	}
}
