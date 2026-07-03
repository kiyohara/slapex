package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// scenario is one fictional workspace fixture. Asset URLs inside the fixture
// use the "{{base}}" placeholder, replaced with the fake server URL once it
// is listening (same convention as the integration-test harness in
// internal/export).
type scenario struct {
	Lang        string // output subdirectory name ("ja" / "en")
	ChannelName string // exact channel name passed as the channel keyword

	Auth     slack.AuthTest
	TeamInfo *slack.TeamInfo
	Channels []slack.Channel
	Messages []slack.Message
	Replies  map[string][]slack.Message
	Users    map[string]slack.User
	Emoji    map[string]string
	Assets   map[string]sampleAsset
}

type sampleAsset struct {
	ContentType string
	Body        []byte
}

// fakeSlackServer serves the subset of the Slack Web API and file downloads
// that the export pipeline uses, backed by a scenario fixture.
type fakeSlackServer struct {
	srv *httptest.Server
	sc  *scenario
	// assetDelay is an artificial per-asset response delay used by -serve so
	// the download progress indicator stays visible in demo recordings.
	assetDelay time.Duration
	// anyBearer accepts any non-empty Bearer token instead of the exact
	// fakeToken. Only -serve sets it: demo recordings type an arbitrary fake
	// value at the token prompt.
	anyBearer bool
}

func newFakeSlackServer(sc *scenario) *fakeSlackServer {
	f := &fakeSlackServer{sc: sc}
	f.srv = httptest.NewServer(f.mux())
	sc.replaceBaseURL(f.srv.URL)
	return f
}

func (f *fakeSlackServer) mux() *http.ServeMux {
	mux := http.NewServeMux()
	for path := range f.sc.Assets {
		mux.HandleFunc(path, f.handleAsset)
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
	return mux
}

func (f *fakeSlackServer) URL() string { return f.srv.URL }
func (f *fakeSlackServer) Close()      { f.srv.Close() }

func (f *fakeSlackServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !f.authorized(r.Header.Get("Authorization")) {
		http.Error(w, "missing auth", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.URL.Path {
	case "/api/auth.test":
		writeSlackOK(w, f.sc.Auth)
	case "/api/team.info":
		writeSlackOK(w, map[string]any{"team": f.sc.TeamInfo})
	case "/api/conversations.list":
		writeSlackOK(w, map[string]any{"channels": f.sc.Channels})
	case "/api/conversations.history":
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

// authorized reports whether the Authorization header may use the API. Sample
// generation requires the exact in-process fakeToken; -serve (anyBearer)
// accepts any non-empty Bearer value.
func (f *fakeSlackServer) authorized(header string) bool {
	if f.anyBearer {
		token, ok := strings.CutPrefix(header, "Bearer ")
		return ok && strings.TrimSpace(token) != ""
	}
	return header == "Bearer "+fakeToken
}

func (f *fakeSlackServer) handleAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := f.sc.Assets[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if f.assetDelay > 0 {
		time.Sleep(f.assetDelay)
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Write(asset.Body)
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

func (sc *scenario) replaceBaseURL(baseURL string) {
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
		sc.TeamInfo.Icon.Image68 = repl(sc.TeamInfo.Icon.Image68)
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
	}
	for i := range m.Attachments {
		m.Attachments[i].ImageURL = repl(m.Attachments[i].ImageURL)
		m.Attachments[i].ServiceIcon = repl(m.Attachments[i].ServiceIcon)
	}
}

// --- fixture helpers ---------------------------------------------------------

// at returns a time on day at hh:mm:ss in the local timezone.
func at(day time.Time, hh, mm, ss int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), hh, mm, ss, 0, time.Local)
}

// ts renders t as a Slack message ts value.
func ts(t time.Time) string { return fmt.Sprintf("%d.000000", t.Unix()) }

func editedAt(t time.Time) *struct {
	TS string `json:"ts"`
} {
	return &struct {
		TS string `json:"ts"`
	}{TS: ts(t)}
}

func botProfile(name string) *struct {
	Name string `json:"name"`
} {
	return &struct {
		Name string `json:"name"`
	}{Name: name}
}

func sampleUser(id, name, realName, displayName, imageURL string) slack.User {
	u := slack.User{ID: id, Name: name, RealName: realName}
	u.Profile.DisplayName = displayName
	u.Profile.RealName = realName
	u.Profile.Image48 = imageURL
	u.Profile.Image72 = imageURL
	return u
}
