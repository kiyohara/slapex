package export

// Fake Slack server for the integration tests. newFakeSlackServer serves one
// exportScenario (integration_fixture_test.go) over httptest: the Web API
// endpoints the exporter calls, plus the asset paths the scenario declares. It
// records a per-path request count (Count) that the cases assert on, injects
// the scenario's APIFaults / AssetFaults ahead of the normal handlers, and
// returns conversations.history unfiltered — the range narrowing is the
// client's job, so the tests exercise it against raw responses.
//
// This server is deliberately separate from the production demo server
// (internal/demo): fault injection and request counting are test-only concerns.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kiyohara/slapex/internal/slack"
)

const integrationTestToken = "xoxb-integration-test-token"

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
		"/api/bots.info",
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
	case "/api/bots.info":
		bot, ok := f.sc.Bots[r.PostForm.Get("bot")]
		if !ok {
			writeSlackError(w, "bot_not_found")
			return
		}
		writeSlackOK(w, map[string]any{"bot": bot})
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

// replaceBaseURL rewrites every "{{base}}" placeholder in the scenario's URLs
// to the fake server's URL once that URL is known.
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
	for id, b := range sc.Bots {
		b.Icons.Image36 = repl(b.Icons.Image36)
		b.Icons.Image48 = repl(b.Icons.Image48)
		b.Icons.Image72 = repl(b.Icons.Image72)
		sc.Bots[id] = b
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
	if m.BotProfile != nil {
		m.BotProfile.Icons.Image36 = repl(m.BotProfile.Icons.Image36)
		m.BotProfile.Icons.Image48 = repl(m.BotProfile.Icons.Image48)
		m.BotProfile.Icons.Image72 = repl(m.BotProfile.Icons.Image72)
	}
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
