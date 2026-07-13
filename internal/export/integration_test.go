package export

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

const integrationTestToken = "xoxb-integration-test-token"

func TestRunIntegrationHappyPath(t *testing.T) {
	t.Parallel()

	sc := happyPathScenario()
	got := runExportScenario(t, sc, Options{
		ChannelKeyword: "project-alpha",
		OutputDir:      t.TempDir(),
		MaxPosts:       10,
		Days:           90,
		MaxAttachBytes: 1 << 20,
		KeepCache:      true,
		ToolVersion:    "test",
	})

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
		"/api/emoji.list":            1,
	})
}

func TestRunIntegrationExcludeBodyEmojiParentAndThread(t *testing.T) {
	t.Parallel()

	sc := happyPathScenario()
	sc.Messages[1].Text += " :shushing_face:"
	got := runExportScenario(t, sc, Options{
		ChannelKeyword:   "project-alpha",
		OutputDir:        t.TempDir(),
		MaxPosts:         10,
		Days:             90,
		ExcludeBodyEmoji: []string{"shushing_face"},
		MaxAttachBytes:   1 << 20,
		KeepCache:        true,
		ToolVersion:      "test",
	})

	bodyBytes, err := os.ReadFile(filepath.Join(got.OutputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(bodyBytes)
	for _, excluded := range []string{"Starting the launch thread", "Reply with screenshot", "Thread is wrapped up"} {
		if strings.Contains(body, excluded) {
			t.Fatalf("index.html contains excluded content %q", excluded)
		}
	}
	if !strings.Contains(body, "--exclude-body-emoji shushing_face") {
		t.Fatal("index.html does not show the active normalized filter")
	}
	assertEndpointCounts(t, got.Server, map[string]int{"/api/conversations.replies": 0})
	assertExcludedMetadata(t, got.OutputDir, 2, 0, 0, 1, []string{"shushing_face"})
	assertCacheOmits(t, got.OutputDir, "U01", "screenshot-original.png", "og-launch.png")
}

func TestRunIntegrationExcludeBodyEmojiParentDropsBroadcastAndRefillsMaxPosts(t *testing.T) {
	t.Parallel()

	const parentTS = "1700000003.000000"
	parent := slack.Message{
		Type:       "message",
		TS:         parentTS,
		ThreadTS:   parentTS,
		User:       "U01",
		Text:       "private parent :shushing_face:",
		ReplyCount: 1,
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
	opts := renderingOptions(t)
	opts.MaxPosts = 2
	opts.ExcludeBodyEmoji = []string{"shushing_face"}

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
	assertExcludedMetadata(t, got.OutputDir, 2, 0, 0, 2, []string{"shushing_face"})
	assertCacheOmits(t, got.OutputDir, "private parent", "private broadcast")
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
	got := runExportScenario(t, sc, Options{
		ChannelKeyword:   "project-alpha",
		OutputDir:        t.TempDir(),
		MaxPosts:         2,
		Days:             90,
		ExcludeBodyEmoji: []string{"do_not_archive", "speak_no_evil"},
		MaxAttachBytes:   1 << 20,
		KeepCache:        true,
		ToolVersion:      "test",
	})

	bodyBytes, err := os.ReadFile(filepath.Join(got.OutputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(bodyBytes)
	for _, excluded := range []string{"private newest", "private reply", "screenshot.png"} {
		if strings.Contains(body, excluded) {
			t.Fatalf("index.html contains excluded content %q", excluded)
		}
	}
	for _, included := range []string{"Starting the launch thread", "First timeline note", "Thread is wrapped up"} {
		if !strings.Contains(body, included) {
			t.Fatalf("index.html is missing retained content %q", included)
		}
	}
	assertExcludedMetadata(t, got.OutputDir, 2, 1, 1, 2, []string{"do_not_archive", "speak_no_evil"})
	assertCacheOmits(t, got.OutputDir, "screenshot-original.png", "screenshot-thumb.png")
}

func TestRunIntegrationExcludeBodyEmojiHidesEmptyThread(t *testing.T) {
	t.Parallel()

	sc := happyPathScenario()
	sc.Replies["1700000002.000000"][1].Text += " :speak_no_evil:"
	sc.Replies["1700000002.000000"][2].Text += " :speak_no_evil:"
	got := runExportScenario(t, sc, Options{
		ChannelKeyword:   "project-alpha",
		OutputDir:        t.TempDir(),
		MaxPosts:         10,
		Days:             90,
		ExcludeBodyEmoji: []string{"speak_no_evil"},
		MaxAttachBytes:   1 << 20,
		KeepCache:        true,
		ToolVersion:      "test",
	})

	bodyBytes, err := os.ReadFile(filepath.Join(got.OutputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "Starting the launch thread") {
		t.Fatal("index.html is missing the retained parent message")
	}
	if strings.Contains(body, `summary class="thread-label"`) {
		t.Fatal("index.html shows thread UI after every reply was excluded")
	}
	assertExcludedMetadata(t, got.OutputDir, 3, 0, 0, 2, []string{"speak_no_evil"})
}

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

func assertExcludedMetadata(t *testing.T, dir string, timeline, threads, replies, excluded int, names []string) {
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
				ExcludeBodyEmoji []string `json:"exclude_body_emoji"`
			} `json:"options"`
		} `json:"fetch"`
		Users map[string]any `json:"users"`
	}
	readJSON(t, filepath.Join(dir, ".cache/metadata.json"), &metadata)
	if metadata.Counts.Timeline != timeline || metadata.Counts.Threads != threads || metadata.Counts.Replies != replies || metadata.Counts.Excluded != excluded {
		t.Fatalf("metadata counts = %+v, want timeline=%d threads=%d replies=%d excluded=%d", metadata.Counts, timeline, threads, replies, excluded)
	}
	if !slices.Equal(metadata.Fetch.Options.ExcludeBodyEmoji, names) {
		t.Fatalf("exclude_body_emoji = %v, want %v", metadata.Fetch.Options.ExcludeBodyEmoji, names)
	}
}

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

	got := runExportScenario(t, sc, Options{
		ChannelKeyword: "project-alpha",
		OutputDir:      t.TempDir(),
		MaxPosts:       2,
		Date:           input,
		MaxAttachBytes: 1 << 20,
		KeepCache:      true,
		ToolVersion:    "test",
	})

	htmlBytes, err := os.ReadFile(filepath.Join(got.OutputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlBytes)
	for _, want := range []string{
		"First timeline note",
		"Starting the launch thread",
		fmt.Sprintf("From %s (included); to %s (not included)", start.UTC().Format(time.RFC3339), start.AddDate(0, 0, 1).UTC().Format(time.RFC3339)),
		"<dt>Options</dt><dd>--date &#34;" + input + "&#34;",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing %q", want)
		}
	}
	if strings.Contains(html, "Final timeline update") {
		t.Fatal("HTML contains message at the exclusive end boundary")
	}

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
	got := runExportScenario(t, sc, Options{
		ChannelKeyword: "project-alpha",
		OutputDir:      t.TempDir(),
		MaxPosts:       2,
		From:           fromInput,
		To:             toInput,
		MaxAttachBytes: 1 << 20,
		KeepCache:      true,
		ToolVersion:    "test",
	})

	htmlBytes, err := os.ReadFile(filepath.Join(got.OutputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlBytes)
	for _, want := range []string{
		"First timeline note",
		"Starting the launch thread",
		fmt.Sprintf("From %s (included); to %s (not included)", start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)),
		"<dt>Options</dt><dd>--from &#34;" + fromInput + "&#34;, --to &#34;" + toInput + "&#34;",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing %q", want)
		}
	}
	if strings.Contains(html, "Final timeline update") {
		t.Fatal("HTML contains message at the exclusive end boundary")
	}

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

// runExportScenario is the integration-test entry point for happy-path
// scenarios: define an exportScenario fixture, call this helper, then assert on
// the returned output directory and fake Slack request counters. It fails the
// test if Run returns an error; error-path scenarios use runExportScenarioRaw
// (integration_error_test.go) instead.
func runExportScenario(t *testing.T, sc exportScenario, opts Options) exportRunResult {
	t.Helper()

	got, _, err := runExportScenarioRaw(t, sc, opts)
	if err != nil {
		t.Fatalf("Run() error = %v\nlogs:\n%s", err, strings.Join(got.Logs, "\n"))
	}
	return got
}

type exportRunResult struct {
	OutputDir string
	Server    *fakeSlackServer
	Logs      []string
}

type exportScenario struct {
	Auth     slack.AuthTest
	TeamInfo *slack.TeamInfo
	Channels []slack.Channel
	Messages []slack.Message
	Replies  map[string][]slack.Message
	Users    map[string]slack.User
	Emoji    map[string]string
	Assets   map[string]fakeAsset

	// APIFaults / AssetFaults inject error and rate-limit behaviour for the
	// v1-09 error scenarios, keyed by request path (e.g.
	// "/api/conversations.history" or "/files/flaky.pdf"). A nil/empty map
	// keeps the happy behaviour, so v1-07 / v1-08 fixtures are unaffected.
	APIFaults   map[string]*endpointFault
	AssetFaults map[string]*endpointFault
}

type fakeAsset struct {
	ContentType string
	Body        string
	RejectAuth  bool
}

// endpointFault injects error / rate-limit behaviour for one fake server
// endpoint (an API path or an asset path) in the v1-09 error scenarios.
type endpointFault struct {
	// transient responses are emitted one per call, in order, before the
	// endpoint falls through to its normal handler. Used for "429 once then
	// succeed" and "5xx then succeed".
	transient []faultResponse
	// sticky, when non-nil, is returned on every call once transient responses
	// are drained. Used for persistent Slack errors (invalid_auth,
	// missing_scope, not_in_channel), a persistent 429 (retry-limit reached)
	// and a persistent download failure.
	sticky *faultResponse
}

// faultResponse is a single fake response. A non-zero httpStatus (429 or 5xx)
// is written directly, with Retry-After taken from retryAfterSec when > 0. An
// httpStatus of 0 with slackError set yields an {"ok":false,...} body;
// otherwise the endpoint's normal handler runs.
type faultResponse struct {
	httpStatus    int
	retryAfterSec int
	slackError    string
}

func happyPathScenario() exportScenario {
	return exportScenario{
		Auth: slack.AuthTest{
			URL:    "https://acme.example.slack.com/",
			Team:   "Acme Workspace",
			TeamID: "TACME123",
			User:   "slapex",
			UserID: "USLAPEX",
			BotID:  "BSLAPEX",
		},
		TeamInfo: &slack.TeamInfo{
			ID:     "TACME123",
			Name:   "Acme Workspace",
			Domain: "acme",
			Icon: slack.TeamIcon{
				Image68: "{{base}}/files/workspace-icon.png",
			},
		},
		Channels: []slack.Channel{
			{ID: "C999", Name: "random", IsMember: true},
			{ID: "C123", Name: "project-alpha", IsMember: true},
		},
		Messages: []slack.Message{
			{
				Type: "message",
				TS:   "1700000003.000000",
				User: "U02",
				Text: "Final timeline update",
			},
			{
				Type:       "message",
				TS:         "1700000002.000000",
				ThreadTS:   "1700000002.000000",
				User:       "U01",
				Text:       "Starting the launch thread with :party_sloth: and <@U02>",
				ReplyCount: 2,
				Attachments: []slack.Attachment{
					{
						ServiceName: "Example News",
						ServiceIcon: "{{base}}/files/service-example-news.png",
						Title:       "Launch checklist",
						TitleLink:   "https://example.com/launch-checklist",
						Text:        "Read <@U02>'s notes",
						ImageURL:    "{{base}}/files/og-launch.png",
					},
				},
				Reactions: []slack.Reaction{
					{Name: "smile", Count: 3},
					{Name: "party_sloth", Count: 2},
				},
			},
			{
				Type: "message",
				TS:   "1700000001.000000",
				User: "U02",
				Text: "First timeline note",
				Files: []slack.File{
					{
						ID:                 "F-DOC",
						Name:               "runbook.pdf",
						Mimetype:           "application/pdf",
						Size:               18,
						URLPrivateDownload: "{{base}}/files/runbook.pdf",
					},
				},
				Reactions: []slack.Reaction{{Name: "smile", Count: 1}},
			},
		},
		Replies: map[string][]slack.Message{
			"1700000002.000000": {
				{
					Type:       "message",
					TS:         "1700000002.000000",
					ThreadTS:   "1700000002.000000",
					User:       "U01",
					Text:       "Starting the launch thread with :party_sloth: and <@U02>",
					ReplyCount: 2,
				},
				{
					Type:     "message",
					TS:       "1700000002.200000",
					ThreadTS: "1700000002.000000",
					User:     "U01",
					Text:     "Thread is wrapped up :party_sloth:",
				},
				{
					Type:     "message",
					TS:       "1700000002.100000",
					ThreadTS: "1700000002.000000",
					User:     "U02",
					Text:     "Reply with screenshot",
					Files: []slack.File{
						{
							ID:                 "F-IMG",
							Name:               "screenshot.png",
							Mimetype:           "image/png",
							Size:               32,
							URLPrivateDownload: "{{base}}/files/screenshot-original.png",
							Thumb360:           "{{base}}/files/screenshot-thumb.png",
						},
					},
				},
			},
		},
		Users: map[string]slack.User{
			"U01": testUser("U01", "alice", "Alice Example", "Alice", "{{base}}/files/avatar-u01.png"),
			"U02": testUser("U02", "bob", "Bob Builder", "Bob", "{{base}}/files/avatar-u02.png"),
		},
		Emoji: map[string]string{
			"party_sloth": "{{base}}/files/emoji-party-sloth.png",
		},
		Assets: map[string]fakeAsset{
			"/files/avatar-u01.png":           {ContentType: "image/png", Body: "avatar-u01"},
			"/files/avatar-u02.png":           {ContentType: "image/png", Body: "avatar-u02"},
			"/files/workspace-icon.png":       {ContentType: "image/png", Body: "workspace-icon"},
			"/files/emoji-party-sloth.png":    {ContentType: "image/png", Body: "custom-emoji"},
			"/files/service-example-news.png": {ContentType: "image/png", Body: "service-icon", RejectAuth: true},
			"/files/og-launch.png":            {ContentType: "image/png", Body: "og-image"},
			"/files/runbook.pdf":              {ContentType: "application/pdf", Body: "runbook attachment"},
			"/files/screenshot-original.png":  {ContentType: "image/png", Body: "screenshot original"},
			"/files/screenshot-thumb.png":     {ContentType: "image/png", Body: "screenshot thumb"},
		},
	}
}

func testUser(id, name, realName, displayName, imageURL string) slack.User {
	u := slack.User{ID: id, Name: name, RealName: realName}
	u.Profile.DisplayName = displayName
	u.Profile.RealName = realName
	u.Profile.Image48 = imageURL
	u.Profile.Image72 = imageURL
	return u
}

type fakeSlackServer struct {
	t   *testing.T
	srv *httptest.Server
	sc  *exportScenario

	mu     sync.Mutex
	counts map[string]int
}

func newFakeSlackServer(t *testing.T, sc *exportScenario) *fakeSlackServer {
	t.Helper()

	f := &fakeSlackServer{
		t:      t,
		sc:     sc,
		counts: map[string]int{},
	}
	mux := http.NewServeMux()
	for path := range sc.Assets {
		mux.HandleFunc(path, f.handleAsset)
	}
	// Asset paths that only exist to inject a download failure (no body in
	// Assets) still need a handler so the fault response is served.
	for path := range sc.AssetFaults {
		if _, ok := sc.Assets[path]; !ok {
			mux.HandleFunc(path, f.handleAsset)
		}
	}
	for _, path := range []string{
		"/api/auth.test",
		"/api/team.info",
		"/api/conversations.list",
		"/api/conversations.history",
		"/api/conversations.replies",
		"/api/users.info",
		"/api/emoji.list",
	} {
		mux.HandleFunc(path, f.handleAPI)
	}
	f.srv = httptest.NewServer(mux)
	sc.replaceBaseURL(f.srv.URL)
	return f
}

func (f *fakeSlackServer) URL() string { return f.srv.URL }

func (f *fakeSlackServer) Close() {
	f.srv.Close()
}

func (f *fakeSlackServer) Count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[path]
}

func (f *fakeSlackServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+integrationTestToken {
		http.Error(w, "missing auth", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.record(r)
	if resp := f.nextFault(f.sc.APIFaults, r.URL.Path); resp != nil && f.writeFault(w, resp) {
		return
	}

	switch r.URL.Path {
	case "/api/auth.test":
		writeSlackOK(w, f.sc.Auth)
	case "/api/team.info":
		team := f.sc.TeamInfo
		if team == nil {
			team = &slack.TeamInfo{
				ID:     f.sc.Auth.TeamID,
				Name:   f.sc.Auth.Team,
				Domain: strings.TrimSuffix(hostOf(f.sc.Auth.URL), ".slack.com"),
				Icon:   slack.TeamIcon{ImageDefault: true},
			}
		}
		writeSlackOK(w, map[string]any{"team": team})
	case "/api/conversations.list":
		writeSlackOK(w, map[string]any{"channels": f.sc.Channels})
	case "/api/conversations.history":
		if got := r.PostForm.Get("channel"); !f.hasChannel(got) {
			writeSlackError(w, "channel_not_found")
			return
		}
		writeSlackOK(w, map[string]any{"messages": f.sc.Messages})
	case "/api/conversations.replies":
		replies, ok := f.sc.Replies[r.PostForm.Get("ts")]
		if !ok {
			writeSlackError(w, "thread_not_found")
			return
		}
		writeSlackOK(w, map[string]any{"messages": replies})
	case "/api/users.info":
		user, ok := f.sc.Users[r.PostForm.Get("user")]
		if !ok {
			writeSlackError(w, "user_not_found")
			return
		}
		writeSlackOK(w, map[string]any{"user": user})
	case "/api/emoji.list":
		writeSlackOK(w, map[string]any{"emoji": f.sc.Emoji})
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeSlackServer) hasChannel(id string) bool {
	return slices.ContainsFunc(f.sc.Channels, func(ch slack.Channel) bool {
		return ch.ID == id
	})
}

func (f *fakeSlackServer) handleAsset(w http.ResponseWriter, r *http.Request) {
	f.record(r)
	if resp := f.nextFault(f.sc.AssetFaults, r.URL.Path); resp != nil && f.writeFault(w, resp) {
		return
	}
	asset, ok := f.sc.Assets[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if asset.RejectAuth && r.Header.Get("Authorization") != "" {
		http.Error(w, "unexpected auth header", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	fmt.Fprint(w, asset.Body)
}

func (f *fakeSlackServer) record(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[r.URL.Path]++
}

// nextFault returns the fault response the endpoint should emit for this call,
// or nil to fall through to the normal handler. transient responses are
// consumed first; once drained, the sticky response (if any) applies to every
// later call.
func (f *fakeSlackServer) nextFault(faults map[string]*endpointFault, path string) *faultResponse {
	if faults == nil {
		return nil
	}
	fault := faults[path]
	if fault == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(fault.transient) > 0 {
		resp := fault.transient[0]
		fault.transient = fault.transient[1:]
		return &resp
	}
	return fault.sticky
}

// writeFault emits resp and reports whether it handled the request. A 429 or
// 5xx status is written directly (with Retry-After for 429); httpStatus 0 with
// slackError yields an {"ok":false} body. Any other shape returns false so the
// caller runs the endpoint's normal handler.
func (f *fakeSlackServer) writeFault(w http.ResponseWriter, resp *faultResponse) bool {
	switch {
	case resp.httpStatus == http.StatusTooManyRequests:
		if resp.retryAfterSec > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(resp.retryAfterSec))
		}
		w.WriteHeader(http.StatusTooManyRequests)
		return true
	case resp.httpStatus >= 500:
		w.WriteHeader(resp.httpStatus)
		return true
	case resp.slackError != "":
		writeSlackError(w, resp.slackError)
		return true
	default:
		return false
	}
}

func writeSlackOK(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	body := map[string]any{"ok": true}
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var fields map[string]any
		if err := json.Unmarshal(data, &fields); err == nil {
			for key, value := range fields {
				if key != "ok" {
					body[key] = value
				}
			}
		} else {
			body["payload"] = payload
		}
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeSlackError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":false,"error":%q}`, code)
}

func (sc *exportScenario) replaceBaseURL(baseURL string) {
	repl := func(s string) string {
		return strings.ReplaceAll(s, "{{base}}", baseURL)
	}
	for i := range sc.Messages {
		replaceMessageBaseURL(&sc.Messages[i], repl)
	}
	for threadTS, replies := range sc.Replies {
		for i := range replies {
			replaceMessageBaseURL(&replies[i], repl)
		}
		sc.Replies[threadTS] = replies
	}
	for id, u := range sc.Users {
		u.Profile.Image48 = repl(u.Profile.Image48)
		u.Profile.Image72 = repl(u.Profile.Image72)
		sc.Users[id] = u
	}
	if sc.TeamInfo != nil {
		sc.TeamInfo.Icon.Image34 = repl(sc.TeamInfo.Icon.Image34)
		sc.TeamInfo.Icon.Image44 = repl(sc.TeamInfo.Icon.Image44)
		sc.TeamInfo.Icon.Image68 = repl(sc.TeamInfo.Icon.Image68)
		sc.TeamInfo.Icon.Image88 = repl(sc.TeamInfo.Icon.Image88)
		sc.TeamInfo.Icon.Image102 = repl(sc.TeamInfo.Icon.Image102)
		sc.TeamInfo.Icon.Image132 = repl(sc.TeamInfo.Icon.Image132)
		sc.TeamInfo.Icon.Image230 = repl(sc.TeamInfo.Icon.Image230)
	}
	for name, rawURL := range sc.Emoji {
		sc.Emoji[name] = repl(rawURL)
	}
}

func replaceMessageBaseURL(m *slack.Message, repl func(string) string) {
	for i := range m.Files {
		m.Files[i].URLPrivate = repl(m.Files[i].URLPrivate)
		m.Files[i].URLPrivateDownload = repl(m.Files[i].URLPrivateDownload)
		m.Files[i].Thumb360 = repl(m.Files[i].Thumb360)
		m.Files[i].Thumb480 = repl(m.Files[i].Thumb480)
		m.Files[i].Thumb160 = repl(m.Files[i].Thumb160)
		m.Files[i].Thumb64 = repl(m.Files[i].Thumb64)
	}
	for i := range m.Attachments {
		m.Attachments[i].ImageURL = repl(m.Attachments[i].ImageURL)
		m.Attachments[i].ThumbURL = repl(m.Attachments[i].ThumbURL)
		m.Attachments[i].ServiceIcon = repl(m.Attachments[i].ServiceIcon)
	}
}

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

func assertOrder(t *testing.T, body string, markers ...string) {
	t.Helper()

	last := -1
	for _, marker := range markers {
		idx := strings.Index(body, marker)
		if idx < 0 {
			t.Fatalf("missing marker %q", marker)
		}
		if idx <= last {
			t.Fatalf("marker %q appeared out of order", marker)
		}
		last = idx
	}
}

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

	var manifest struct {
		Assets []manifestAsset `json:"assets"`
	}
	readJSON(t, filepath.Join(dir, ".cache/assets_manifest.json"), &manifest)
	for _, kind := range []string{"workspace_icon", "avatar", "emoji", "service_icon", "og_image", "upload_thumb", "upload_original", "attachment"} {
		if !hasSavedAsset(manifest.Assets, kind) {
			t.Fatalf("assets_manifest.json missing saved %s asset: %+v", kind, manifest.Assets)
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

func readJSON(t *testing.T, path string, out any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

type manifestAsset struct {
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

func hasSavedAsset(assets []manifestAsset, kind string) bool {
	return slices.ContainsFunc(assets, func(asset manifestAsset) bool {
		return asset.Kind == kind && asset.Status == "saved"
	})
}

func assertEndpointCounts(t *testing.T, fake *fakeSlackServer, want map[string]int) {
	t.Helper()

	for path, count := range want {
		if got := fake.Count(path); got != count {
			t.Fatalf("%s count = %d, want %d", path, got, count)
		}
	}
}
