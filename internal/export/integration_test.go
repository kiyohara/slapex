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
		"/api/conversations.list":    1,
		"/api/conversations.history": 1,
		"/api/conversations.replies": 1,
		"/api/users.info":            2,
		"/api/emoji.list":            1,
	})
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
			"/files/emoji-party-sloth.png":    {ContentType: "image/png", Body: "custom-emoji"},
			"/files/service-example-news.png": {ContentType: "image/png", Body: "service-icon"},
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
	if got := r.Header.Get("Authorization"); got != "Bearer "+integrationTestToken {
		http.Error(w, "missing auth", http.StatusUnauthorized)
		return
	}
	f.record(r)
	if resp := f.nextFault(f.sc.AssetFaults, r.URL.Path); resp != nil && f.writeFault(w, resp) {
		return
	}
	asset, ok := f.sc.Assets[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
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
		`assets/emoji/`,
		`assets/service-icons/`,
		`assets/og-images/`,
		`assets/uploads/thumbs/`,
		`assets/uploads/originals/`,
		`assets/attachments/`,
		"runbook.pdf",
		"Launch checklist",
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
	if metadata.Counts.AssetsSaved < 7 {
		t.Fatalf("metadata assets_saved = %d, want at least 7", metadata.Counts.AssetsSaved)
	}

	var manifest struct {
		Assets []manifestAsset `json:"assets"`
	}
	readJSON(t, filepath.Join(dir, ".cache/assets_manifest.json"), &manifest)
	for _, kind := range []string{"avatar", "emoji", "service_icon", "og_image", "upload_thumb", "upload_original", "attachment"} {
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
