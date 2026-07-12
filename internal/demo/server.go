package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// NoPacing is a no-op Slack client sleeper (slack.WithSleeper). The demo serves
// its fixture in-process with no real rate limits, so applying the normal
// per-method API pacing would only add pointless latency.
func NoPacing(context.Context, time.Duration) error { return nil }

// FakeToken authenticates against the in-process fake server only. It is not a
// real credential and never leaves the process: sample generation and the
// slapex --demo run both send it to a loopback fake server, so no real Slack
// host ever receives it.
const FakeToken = "xoxp-slapex-demo-fake-token"

// fakeServer serves the subset of the Slack Web API and file downloads that the
// export pipeline uses, backed by a scenario fixture.
type fakeServer struct {
	sc *Scenario
	// assetDelay is an artificial per-asset response delay used by long-lived
	// serving so the download progress indicator stays visible in demo
	// recordings.
	assetDelay time.Duration
	// anyBearer accepts any non-empty Bearer token instead of the exact
	// FakeToken. Demo GIF recording sets it because the recording types an
	// arbitrary fake value at the token prompt.
	anyBearer bool
}

// HandlerOption configures Handler.
type HandlerOption func(*fakeServer)

// WithAssetDelay adds an artificial per-asset response delay so the download
// progress indicator stays visible in demo recordings.
func WithAssetDelay(d time.Duration) HandlerOption {
	return func(f *fakeServer) { f.assetDelay = d }
}

// AllowAnyToken makes the fake server accept any non-empty Bearer token
// instead of the exact FakeToken. Only the demo GIF recording needs this,
// because it types an arbitrary fake value at slapex's token prompt.
func AllowAnyToken() HandlerOption {
	return func(f *fakeServer) { f.anyBearer = true }
}

// Handler returns an http.Handler serving sc's fake Slack API. The caller owns
// the listener and must call sc.ReplaceBaseURL with the externally reachable
// base URL before serving, so the fixture's asset URLs resolve. Use NewServer
// instead when an in-process loopback server is enough.
func Handler(sc *Scenario, opts ...HandlerOption) http.Handler {
	f := &fakeServer{sc: sc}
	for _, opt := range opts {
		opt(f)
	}
	return f.mux()
}

// Server is an in-process httptest server backed by a scenario fixture, with
// the fixture's asset URLs already rewritten to point at it. It authenticates
// the exact FakeToken.
type Server struct {
	srv *httptest.Server
}

// NewServer starts a loopback httptest server for sc and rewrites the
// fixture's asset URLs to point at it. Close it when done.
func NewServer(sc *Scenario) *Server {
	f := &fakeServer{sc: sc}
	srv := httptest.NewServer(f.mux())
	sc.ReplaceBaseURL(srv.URL)
	return &Server{srv: srv}
}

// URL is the server's root URL.
func (s *Server) URL() string { return s.srv.URL }

// APIBaseURL is the base URL to pass to slack.WithBaseURL.
func (s *Server) APIBaseURL() string { return s.srv.URL + "/api/" }

// Close shuts the server down.
func (s *Server) Close() { s.srv.Close() }

func (f *fakeServer) mux() *http.ServeMux {
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

func (f *fakeServer) handleAPI(w http.ResponseWriter, r *http.Request) {
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
		writeSlackOK(w, map[string]any{"messages": filterRange(f.sc.Messages, r.PostForm.Get("oldest"), r.PostForm.Get("latest"))})
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
// generation and slapex --demo require the exact in-process FakeToken;
// AllowAnyToken (demo GIF recording) accepts any non-empty Bearer value.
func (f *fakeServer) authorized(header string) bool {
	if f.anyBearer {
		token, ok := strings.CutPrefix(header, "Bearer ")
		return ok && strings.TrimSpace(token) != ""
	}
	return header == "Bearer "+FakeToken
}

func (f *fakeServer) handleAsset(w http.ResponseWriter, r *http.Request) {
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
