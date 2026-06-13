package slack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "xoxb-test-token"

// sleepRecorder fakes Client.sleep, recording waits without real sleeping.
type sleepRecorder struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (r *sleepRecorder) sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waits = append(r.waits, d)
	return nil
}

func (r *sleepRecorder) recorded() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.waits...)
}

// newTestClient points a Client at srv and disables real sleeping.
func newTestClient(srv *httptest.Server) (*Client, *sleepRecorder) {
	rec := &sleepRecorder{}
	c := New(testToken, WithBaseURL(srv.URL+"/api/"), WithSleeper(rec.sleep))
	return c, rec
}

// assertWaitIn checks that wait is base plus at most 1s of jitter.
func assertWaitIn(t *testing.T, wait, base time.Duration) {
	t.Helper()
	if wait < base || wait >= base+time.Second {
		t.Errorf("wait = %v, want in [%v, %v)", wait, base, base+time.Second)
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()

	c := New(testToken)
	if c.baseURL != apiBase {
		t.Errorf("baseURL = %q, want %q", c.baseURL, apiBase)
	}
	if c.sleep == nil {
		t.Error("sleep is nil, want real sleep by default")
	}
}

func TestCallSendsFormEncodedRequestWithAuth(t *testing.T) {
	t.Parallel()

	type request struct {
		method      string
		path        string
		auth        string
		contentType string
		form        url.Values
	}
	var (
		mu  sync.Mutex
		got request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("parse request body: %v", err)
		}
		mu.Lock()
		got = request{
			method:      r.Method,
			path:        r.URL.Path,
			auth:        r.Header.Get("Authorization"),
			contentType: r.Header.Get("Content-Type"),
			form:        form,
		}
		mu.Unlock()
		fmt.Fprint(w, `{"ok":true,"user":{"id":"U123","name":"alice"}}`)
	}))
	defer srv.Close()

	c, _ := newTestClient(srv)
	user, err := c.UserInfo(context.Background(), "U123")
	if err != nil {
		t.Fatalf("UserInfo: %v", err)
	}
	if user.ID != "U123" || user.Name != "alice" {
		t.Errorf("user = %+v, want ID U123 / Name alice", user)
	}

	mu.Lock()
	defer mu.Unlock()
	if got.method != http.MethodPost {
		t.Errorf("method = %q, want POST", got.method)
	}
	if got.path != "/api/users.info" {
		t.Errorf("path = %q, want /api/users.info", got.path)
	}
	if want := "Bearer " + testToken; got.auth != want {
		t.Errorf("Authorization = %q, want %q", got.auth, want)
	}
	if got.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", got.contentType)
	}
	if got.form.Get("user") != "U123" {
		t.Errorf("form user = %q, want U123", got.form.Get("user"))
	}
}

func TestCallAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":false,"error":"invalid_auth"}`)
	}))
	defer srv.Close()

	c, rec := newTestClient(srv)
	_, err := c.AuthTest(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Method != "auth.test" || apiErr.Code != "invalid_auth" {
		t.Errorf("APIError = %+v, want Method auth.test / Code invalid_auth", apiErr)
	}
	if waits := rec.recorded(); len(waits) != 0 {
		t.Errorf("waits = %v, want none (ok:false must not be retried)", waits)
	}
}

// pagedHandler serves canned JSON pages keyed by the cursor form value and
// records the cursors and form values it receives.
type pagedHandler struct {
	t     *testing.T
	path  string
	pages map[string]string

	mu      sync.Mutex
	cursors []string
	forms   []url.Values
}

func (h *pagedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != h.path {
		h.t.Errorf("path = %q, want %q", r.URL.Path, h.path)
	}
	if err := r.ParseForm(); err != nil {
		h.t.Errorf("parse form: %v", err)
	}
	cursor := r.PostForm.Get("cursor")
	h.mu.Lock()
	h.cursors = append(h.cursors, cursor)
	h.forms = append(h.forms, r.PostForm)
	h.mu.Unlock()
	page, ok := h.pages[cursor]
	if !ok {
		h.t.Errorf("unexpected cursor %q", cursor)
		fmt.Fprint(w, `{"ok":false,"error":"invalid_cursor"}`)
		return
	}
	fmt.Fprint(w, page)
}

func (h *pagedHandler) gotCursors() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.cursors...)
}

func (h *pagedHandler) gotForms() []url.Values {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]url.Values(nil), h.forms...)
}

func TestListChannelsPagination(t *testing.T) {
	t.Parallel()

	h := &pagedHandler{
		t:    t,
		path: "/api/conversations.list",
		pages: map[string]string{
			"":     `{"ok":true,"channels":[{"id":"C1","name":"one"},{"id":"C2","name":"two"}],"response_metadata":{"next_cursor":"cur1"}}`,
			"cur1": `{"ok":true,"channels":[{"id":"C3","name":"three"},{"id":"C4","name":"four"}],"response_metadata":{"next_cursor":"cur2"}}`,
			"cur2": `{"ok":true,"channels":[{"id":"C5","name":"five"}],"response_metadata":{"next_cursor":""}}`,
		},
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, _ := newTestClient(srv)
	channels, err := c.ListChannels(context.Background())
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	var ids []string
	for _, ch := range channels {
		ids = append(ids, ch.ID)
	}
	if got, want := strings.Join(ids, ","), "C1,C2,C3,C4,C5"; got != want {
		t.Errorf("channel ids = %q, want %q", got, want)
	}
	if got, want := strings.Join(h.gotCursors(), "|"), "|cur1|cur2"; got != want {
		t.Errorf("cursors = %q, want %q", got, want)
	}
}

func TestHistoryPagination(t *testing.T) {
	t.Parallel()

	h := &pagedHandler{
		t:    t,
		path: "/api/conversations.history",
		pages: map[string]string{
			"":     `{"ok":true,"messages":[{"ts":"5.0","text":"m5"},{"ts":"4.0","text":"m4"}],"response_metadata":{"next_cursor":"cur1"}}`,
			"cur1": `{"ok":true,"messages":[{"ts":"3.0","text":"m3"},{"ts":"2.0","text":"m2"}],"response_metadata":{"next_cursor":"cur2"}}`,
			"cur2": `{"ok":true,"messages":[{"ts":"1.0","text":"m1"}],"response_metadata":{"next_cursor":""}}`,
		},
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, _ := newTestClient(srv)
	const oldest = "1700000000.000000"
	messages, truncated, err := c.History(context.Background(), "C123", oldest, 100, nil)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if truncated {
		t.Error("truncated = true, want false")
	}
	var tss []string
	for _, m := range messages {
		tss = append(tss, m.TS)
	}
	if got, want := strings.Join(tss, ","), "5.0,4.0,3.0,2.0,1.0"; got != want {
		t.Errorf("message ts = %q, want %q", got, want)
	}
	if got, want := strings.Join(h.gotCursors(), "|"), "|cur1|cur2"; got != want {
		t.Errorf("cursors = %q, want %q", got, want)
	}
	for i, form := range h.gotForms() {
		if got := form.Get("channel"); got != "C123" {
			t.Errorf("request %d channel = %q, want C123", i, got)
		}
		if got := form.Get("oldest"); got != oldest {
			t.Errorf("request %d oldest = %q, want %q", i, got, oldest)
		}
		if got := form.Get("limit"); got != "200" {
			t.Errorf("request %d limit = %q, want 200", i, got)
		}
	}
}

func TestRepliesPagination(t *testing.T) {
	t.Parallel()

	const threadTS = "1700000001.000100"
	h := &pagedHandler{
		t:    t,
		path: "/api/conversations.replies",
		pages: map[string]string{
			"":     `{"ok":true,"messages":[{"ts":"1700000001.000100","text":"parent"},{"ts":"r1","text":"reply1"},{"ts":"r2","text":"reply2"}],"response_metadata":{"next_cursor":"cur1"}}`,
			"cur1": `{"ok":true,"messages":[{"ts":"r3","text":"reply3"}],"response_metadata":{"next_cursor":""}}`,
		},
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, _ := newTestClient(srv)
	replies, truncated, err := c.Replies(context.Background(), "C123", threadTS, 100)
	if err != nil {
		t.Fatalf("Replies: %v", err)
	}
	if truncated {
		t.Error("truncated = true, want false")
	}
	var tss []string
	for _, m := range replies {
		tss = append(tss, m.TS)
	}
	if got, want := strings.Join(tss, ","), "r1,r2,r3"; got != want {
		t.Errorf("reply ts = %q, want %q (parent must be excluded)", got, want)
	}
	if got, want := strings.Join(h.gotCursors(), "|"), "|cur1"; got != want {
		t.Errorf("cursors = %q, want %q", got, want)
	}
	for i, form := range h.gotForms() {
		if got := form.Get("ts"); got != threadTS {
			t.Errorf("request %d ts = %q, want %q", i, got, threadTS)
		}
	}
}

// statusSequenceServer responds with the given HTTP statuses in order, then
// serves okBody with status 200. It counts the requests it received.
type statusSequenceServer struct {
	mu       sync.Mutex
	requests int
	statuses []int
	headers  []http.Header // optional per-status headers, parallel to statuses
	okBody   string
}

func (s *statusSequenceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	n := s.requests
	s.requests++
	s.mu.Unlock()
	if n < len(s.statuses) {
		if s.headers != nil && s.headers[n] != nil {
			for k, vs := range s.headers[n] {
				w.Header()[k] = vs
			}
		}
		w.WriteHeader(s.statuses[n])
		return
	}
	fmt.Fprint(w, s.okBody)
}

func (s *statusSequenceServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

const authTestOK = `{"ok":true,"url":"https://example.com/","team":"T1"}`

func TestCall429HonoursRetryAfter(t *testing.T) {
	t.Parallel()

	seq := &statusSequenceServer{
		statuses: []int{http.StatusTooManyRequests},
		headers:  []http.Header{{"Retry-After": {"3"}}},
		okBody:   authTestOK,
	}
	srv := httptest.NewServer(seq)
	defer srv.Close()

	c, rec := newTestClient(srv)
	if _, err := c.AuthTest(context.Background()); err != nil {
		t.Fatalf("AuthTest: %v", err)
	}
	if n := seq.requestCount(); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
	waits := rec.recorded()
	if len(waits) != 1 {
		t.Fatalf("waits = %v, want exactly one Retry-After wait", waits)
	}
	assertWaitIn(t, waits[0], 3*time.Second)
}

func TestCall429WithoutRetryAfterBacksOff(t *testing.T) {
	t.Parallel()

	seq := &statusSequenceServer{
		statuses: []int{http.StatusTooManyRequests, http.StatusTooManyRequests},
		okBody:   authTestOK,
	}
	srv := httptest.NewServer(seq)
	defer srv.Close()

	c, rec := newTestClient(srv)
	if _, err := c.AuthTest(context.Background()); err != nil {
		t.Fatalf("AuthTest: %v", err)
	}
	if n := seq.requestCount(); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
	waits := rec.recorded()
	if len(waits) != 2 {
		t.Fatalf("waits = %v, want two backoff waits", waits)
	}
	assertWaitIn(t, waits[0], 1*time.Second)
	assertWaitIn(t, waits[1], 2*time.Second)
}

func TestCall5xxBacksOff(t *testing.T) {
	t.Parallel()

	seq := &statusSequenceServer{
		statuses: []int{http.StatusInternalServerError, http.StatusServiceUnavailable},
		okBody:   authTestOK,
	}
	srv := httptest.NewServer(seq)
	defer srv.Close()

	c, rec := newTestClient(srv)
	if _, err := c.AuthTest(context.Background()); err != nil {
		t.Fatalf("AuthTest: %v", err)
	}
	if n := seq.requestCount(); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
	waits := rec.recorded()
	if len(waits) != 2 {
		t.Fatalf("waits = %v, want two backoff waits", waits)
	}
	assertWaitIn(t, waits[0], 1*time.Second)
	assertWaitIn(t, waits[1], 2*time.Second)
}

func TestCallGivesUpAfterMaxRetries(t *testing.T) {
	t.Parallel()

	seq := &statusSequenceServer{
		statuses: slices.Repeat([]int{http.StatusInternalServerError}, maxRetries+1),
		okBody:   authTestOK,
	}
	srv := httptest.NewServer(seq)
	defer srv.Close()

	c, rec := newTestClient(srv)
	_, err := c.AuthTest(context.Background())
	if err == nil {
		t.Fatal("AuthTest succeeded, want error after retries are exhausted")
	}
	if !strings.Contains(err.Error(), "giving up after 5 retries") {
		t.Errorf("err = %v, want it to mention giving up after 5 retries", err)
	}
	if n := seq.requestCount(); n != 6 {
		t.Errorf("requests = %d, want 6 (initial + 5 retries)", n)
	}
	waits := rec.recorded()
	if len(waits) != 5 {
		t.Fatalf("waits = %v, want five backoff waits", waits)
	}
	for i, base := range []time.Duration{1, 2, 4, 8, 16} {
		assertWaitIn(t, waits[i], base*time.Second)
	}
}

// always429 returns a server config where every request up to the retry
// budget is rejected with 429 + Retry-After: 1.
func always429() *statusSequenceServer {
	return &statusSequenceServer{
		statuses: slices.Repeat([]int{http.StatusTooManyRequests}, maxRetries+1),
		headers:  slices.Repeat([]http.Header{{"Retry-After": {"1"}}}, maxRetries+1),
		okBody:   authTestOK,
	}
}

func TestCall429RetryAfterWaitsBeforeGivingUp(t *testing.T) {
	t.Parallel()

	seq := always429()
	srv := httptest.NewServer(seq)
	defer srv.Close()

	c, rec := newTestClient(srv)
	_, err := c.AuthTest(context.Background())
	if err == nil {
		t.Fatal("AuthTest succeeded, want error after retries are exhausted")
	}
	if !strings.Contains(err.Error(), "giving up after 5 retries") {
		t.Errorf("err = %v, want it to mention giving up after 5 retries", err)
	}
	if n := seq.requestCount(); n != 6 {
		t.Errorf("requests = %d, want 6 (initial + 5 retries)", n)
	}
	// Every 429 must wait out its Retry-After, including the final attempt's:
	// the same Client keeps issuing requests after a failed call.
	waits := rec.recorded()
	if len(waits) != 6 {
		t.Fatalf("waits = %v, want one Retry-After wait per 429 response", waits)
	}
	for _, w := range waits {
		assertWaitIn(t, w, 1*time.Second)
	}
}

func TestDownloadRetryAfterWaitsBeforeGivingUp(t *testing.T) {
	t.Parallel()

	seq := always429()
	srv := httptest.NewServer(seq)
	defer srv.Close()

	c, rec := newTestClient(srv)
	var buf bytes.Buffer
	_, _, err := c.Download(context.Background(), srv.URL+"/files/orig.png", 0, &buf)
	if err == nil {
		t.Fatal("Download succeeded, want error after retries are exhausted")
	}
	if !strings.Contains(err.Error(), "giving up after 5 retries") {
		t.Errorf("err = %v, want it to mention giving up after 5 retries", err)
	}
	if n := seq.requestCount(); n != 6 {
		t.Errorf("requests = %d, want 6 (initial + 5 retries)", n)
	}
	waits := rec.recorded()
	if len(waits) != 6 {
		t.Fatalf("waits = %v, want one Retry-After wait per 429 response", waits)
	}
	for _, w := range waits {
		assertWaitIn(t, w, 1*time.Second)
	}
}

func TestBackoffWait(t *testing.T) {
	t.Parallel()

	cases := []struct {
		attempt int
		base    time.Duration
	}{
		{attempt: 1, base: time.Second},
		{attempt: 2, base: 2 * time.Second},
		{attempt: 5, base: 16 * time.Second},
		{attempt: 7, base: 60 * time.Second}, // capped at maxBackoff
	}
	for _, tt := range cases {
		got := backoffWait(tt.attempt)
		if got < tt.base || got >= tt.base+time.Second {
			t.Errorf("backoffWait(%d) = %v, want in [%v, %v)", tt.attempt, got, tt.base, tt.base+time.Second)
		}
	}
}

func TestDownloadSavesBody(t *testing.T) {
	t.Parallel()

	const content = "fake image bytes"
	var (
		mu   sync.Mutex
		auth string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auth = r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	c, _ := newTestClient(srv)
	var buf bytes.Buffer
	written, contentType, err := c.Download(context.Background(), srv.URL+"/files/orig.png", 0, &buf)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if written != int64(len(content)) || buf.String() != content {
		t.Errorf("written = %d body = %q, want %d / %q", written, buf.String(), len(content), content)
	}
	if contentType != "image/png" {
		t.Errorf("contentType = %q, want image/png", contentType)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := "Bearer " + testToken; auth != want {
		t.Errorf("Authorization = %q, want %q", auth, want)
	}
}

func TestDownloadRetries(t *testing.T) {
	t.Parallel()

	const content = "asset body"
	seq := &statusSequenceServer{
		statuses: []int{http.StatusTooManyRequests, http.StatusInternalServerError},
		headers:  []http.Header{{"Retry-After": {"7"}}, nil},
		okBody:   content,
	}
	srv := httptest.NewServer(seq)
	defer srv.Close()

	c, rec := newTestClient(srv)
	var buf bytes.Buffer
	written, _, err := c.Download(context.Background(), srv.URL+"/files/orig.png", 0, &buf)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if written != int64(len(content)) || buf.String() != content {
		t.Errorf("written = %d body = %q, want %d / %q", written, buf.String(), len(content), content)
	}
	if n := seq.requestCount(); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
	waits := rec.recorded()
	if len(waits) != 2 {
		t.Fatalf("waits = %v, want two waits", waits)
	}
	assertWaitIn(t, waits[0], 7*time.Second) // Retry-After: 7
	assertWaitIn(t, waits[1], 2*time.Second) // backoff before the 2nd retry
}
