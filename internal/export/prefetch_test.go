package export

// Unit cases of the prefetcher (prefetch.go, Issue #278): a request goes out
// once, whichever of the prefetch and the driver asks for it first; the
// notices of a prefetched request wait for the driver to take it and pass
// straight on from then; stop cancels what is under way, waits for it and
// drops it; and the switch the comparisons use turns the prefetch off. The
// comparisons of whole exports with the prefetch and without it are in
// integration_prefetch_test.go.

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// usersInfoServer answers users.info for any user: with a 429 that asks to
// wait one second for its first rateLimited requests, then with the user.
// hold, when set, runs on each request before the server answers it, with the
// request's number from 1.
type usersInfoServer struct {
	rateLimited int
	hold        func(r *http.Request, n int)

	mu    sync.Mutex
	users []string // the user of each request, in request order
}

func (s *usersInfoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user := r.PostForm.Get("user")
	s.mu.Lock()
	s.users = append(s.users, user)
	n := len(s.users)
	s.mu.Unlock()
	if s.hold != nil {
		s.hold(r, n)
	}
	if n <= s.rateLimited {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	fmt.Fprintf(w, `{"ok":true,"user":{"id":%q}}`, user)
}

func (s *usersInfoServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.users)
}

// newUsersInfoClient serves s and returns a client of it whose waits go to
// sleep and whose notices go to lines.
func newUsersInfoClient(t *testing.T, s *usersInfoServer, sleep func(context.Context, time.Duration) error, lines chan<- string) *slack.Client {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	client := slack.New(integrationTestToken,
		slack.WithBaseURL(srv.URL+"/api/"),
		slack.WithSleeper(sleep),
		slack.WithTransport(srv.Client().Transport),
	)
	client.Logf = func(format string, args ...any) {
		lines <- fmt.Sprintf(format, args...)
	}
	return client
}

func noWait(context.Context, time.Duration) error { return nil }

// nextLine is the next notice passed on to lines.
func nextLine(t *testing.T, lines <-chan string) string {
	t.Helper()
	select {
	case line := <-lines:
		return line
	case <-time.After(10 * time.Second):
		t.Fatalf("no notice passed on")
		return ""
	}
}

var rateLimitNotice = regexp.MustCompile(`^rate limited on api users\.info, waiting [12]s as instructed by Slack$`)

func TestPrefetcherHoldsNoticesUntilTaken(t *testing.T) {
	t.Parallel()

	// Each wait for a 429's Retry-After stops until the test resumes it.
	waits := make(chan struct{})
	resume := make(chan struct{})
	sleep := func(ctx context.Context, _ time.Duration) error {
		select {
		case waits <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case <-resume:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	lines := make(chan string, 10)
	server := &usersInfoServer{rateLimited: 3}
	f := newPrefetcher(context.Background(), newUsersInfoClient(t, server, sleep, lines), nil)
	defer f.stop()

	f.prefetchUsers([]string{"U01"})
	for i := range 2 {
		<-waits // the notice of the 429 came before its wait
		if n := len(lines); n != 0 {
			t.Fatalf("%d notices passed on before the driver took the request, want them held", n)
		}
		if i == 0 {
			resume <- struct{}{}
		}
	}

	type result struct {
		user *slack.User
		err  error
	}
	took := make(chan result, 1)
	go func() {
		user, err := f.userInfo(context.Background(), "U01")
		took <- result{user, err}
	}()
	var got []string
	for range 2 {
		got = append(got, nextLine(t, lines)) // the held notices, once the driver takes the request
	}
	resume <- struct{}{}
	got = append(got, nextLine(t, lines)) // the third 429's, straight on, while the request is under way
	select {
	case <-took:
		t.Fatalf("the driver took the result before the request ended")
	default:
	}
	<-waits
	resume <- struct{}{}

	res := <-took
	if res.err != nil || res.user == nil || res.user.ID != "U01" {
		t.Fatalf("userInfo() = %+v, %v, want user U01", res.user, res.err)
	}
	for _, line := range got {
		if !rateLimitNotice.MatchString(line) {
			t.Errorf("notice = %q, want %s", line, rateLimitNotice)
		}
	}
	if want := []string{"U01", "U01", "U01", "U01"}; !slices.Equal(server.requests(), want) {
		t.Errorf("requests = %q, want %q", server.requests(), want)
	}
}

func TestPrefetcherStopDropsWhatNoStageTook(t *testing.T) {
	t.Parallel()

	// The first request gets a 429, whose notice is held; the second is
	// under way until the client gives it up.
	underWay := make(chan struct{})
	server := &usersInfoServer{rateLimited: 1, hold: func(r *http.Request, n int) {
		if n == 2 {
			close(underWay)
			<-r.Context().Done()
		}
	}}
	lines := make(chan string, 10)
	sleep := func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	f := newPrefetcher(context.Background(), newUsersInfoClient(t, server, sleep, lines), nil)

	f.prefetchUsers([]string{"U01"})
	<-underWay
	f.stop()

	r := f.requests[requestKey{method: "users.info", arg: "U01"}]
	select {
	case <-r.done:
	default:
		t.Fatalf("stop returned while the request was under way")
	}
	if r.err == nil {
		t.Errorf("the stopped request succeeded, want it canceled")
	}
	f.prefetchUsers([]string{"U02"})
	if f.requests[requestKey{method: "users.info", arg: "U02"}] != nil {
		t.Errorf("a request went ahead after stop")
	}
	if n := len(lines); n != 0 {
		t.Errorf("%d notices passed on, want those of the request no stage took dropped", n)
	}
	if want := []string{"U01", "U01"}; !slices.Equal(server.requests(), want) {
		t.Errorf("requests = %q, want %q", server.requests(), want)
	}
}

func TestPrefetcherSendsEachRequestOnce(t *testing.T) {
	t.Parallel()

	type take struct {
		key   requestKey
		ahead bool
	}
	var taken []take
	ctx := context.WithValue(context.Background(), requestTakenKey{}, func(key requestKey, ahead bool) {
		taken = append(taken, take{key, ahead})
	})
	server := &usersInfoServer{}
	reuse := &reusableCache{users: map[string]cachedUser{"U09": {DisplayName: "cached"}}}
	f := newPrefetcher(ctx, newUsersInfoClient(t, server, noWait, make(chan string, 10)), reuse)

	f.prefetchUsers([]string{"U01", "U02", "U09"})
	f.prefetchUsers([]string{"U01"})
	for _, id := range []string{"U02", "U01", "U03"} {
		if user, err := f.userInfo(ctx, id); err != nil || user.ID != id {
			t.Fatalf("userInfo(%s) = %+v, %v", id, user, err)
		}
	}
	f.prefetchUsers([]string{"U03"})
	f.stop()

	// The reuse cache holds U09, and U03 went out when the driver asked.
	if got, want := slices.Sorted(slices.Values(server.requests())), []string{"U01", "U02", "U03"}; !slices.Equal(got, want) {
		t.Errorf("requests = %q, want %q", got, want)
	}
	want := []take{{requestKey{"users.info", "U02"}, true}, {requestKey{"users.info", "U01"}, true}, {requestKey{"users.info", "U03"}, false}}
	if !slices.Equal(taken, want) {
		t.Errorf("taken = %v, want %v", taken, want)
	}
}

func TestPrefetcherOff(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), prefetchOffKey{}, true)
	server := &usersInfoServer{}
	f := newPrefetcher(ctx, newUsersInfoClient(t, server, noWait, make(chan string, 10)), nil)
	defer f.stop()

	f.prefetchUsers([]string{"U01", "U02"})
	f.prefetchEmojiList()
	f.prefetchThread("C123", "1700000001.000000")
	if len(f.requests) != 0 {
		t.Fatalf("requests went ahead with the prefetch off: %v", slices.Collect(maps.Keys(f.requests)))
	}
	if user, err := f.userInfo(ctx, "U02"); err != nil || user.ID != "U02" {
		t.Fatalf("userInfo(U02) = %+v, %v", user, err)
	}
	if want := []string{"U02"}; !slices.Equal(server.requests(), want) {
		t.Errorf("requests = %q, want %q", server.requests(), want)
	}
}
