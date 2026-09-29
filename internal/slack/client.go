// Package slack is a thin Slack Web API client covering only the methods
// slapex needs (doc/design/slack-api-usage.md). It implements the rate limit
// policy from decision log 0025: honour 429 + Retry-After, exponential
// backoff for transient failures, at most 5 retries per request, and
// self-pacing of roughly 1 request/sec per Web API method. Asset downloads
// are not paced: they go through a client of their own, which the parallel
// fetch of the assets bounds by origin instead (internal/lane, decision log
// 0067).
package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kiyohara/slapex/internal/lane"
)

const apiBase = "https://slack.com/api/"

const (
	maxRetries = 5
	maxBackoff = 60 * time.Second
	methodPace = 1 * time.Second
	pageLimit  = 200
)

// A download attempt fails when its response headers have not come
// downloadHeaderWait after the request went out, and its body when it has
// gone downloadStallWait without a byte (Issue #275, decision log 0067). They
// take the place of an overall timeout, which a large file could outlast
// while it shares its connection with others. The wait for a connection is
// the transport's (its dial and TLS handshake timeouts). An attempt at an
// asset that is not a Slack file (downloadNeedsAuth), such as a URL preview's
// image on a third-party host, also fails once it has taken
// downloadPublicTimeout: a host that sends a byte now and then would pass the
// stall wait for good.
const (
	downloadHeaderWait    = 30 * time.Second
	downloadStallWait     = 30 * time.Second
	downloadPublicTimeout = 5 * time.Minute
)

// APIError is a Slack-level failure (HTTP 200 with ok: false).
type APIError struct {
	Method string
	Code   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("slack api %s failed: %s", e.Method, e.Code)
}

// ErrTooLarge is returned by Download when the body exceeds the given limit.
var ErrTooLarge = errors.New("download exceeds size limit")

// Client is a Slack Web API client. Its Web API calls run one at a time;
// Download may run on several goroutines at once (the parallel asset fetch).
type Client struct {
	token   string
	baseURL string
	// httpClient sends the Web API calls, and dlClient the downloads, over a
	// transport of their own (NewDownloadTransport) and with no overall
	// timeout: headerWait and stallWait bound each attempt instead, and
	// publicTimeout an attempt at an asset that is not a Slack file.
	httpClient            *http.Client
	dlClient              *http.Client
	headerWait, stallWait time.Duration
	publicTimeout         time.Duration
	lastCall              map[string]time.Time
	// sleep performs pacing and retry waits. Tests replace it with a fake
	// that records the requested durations without sleeping.
	sleep func(context.Context, time.Duration) error
	// trace writes the HTTP trace (WithTrace, trace.go); nil when off.
	trace *tracer
	// Logf reports progress such as rate limit waits. Defaults to a no-op.
	Logf func(format string, args ...any)
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL points the client at an alternate Slack Web API base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		c.baseURL = baseURL
	}
}

// WithSleeper replaces the Web API pacing and the retry sleeps.
func WithSleeper(sleep func(context.Context, time.Duration) error) Option {
	return func(c *Client) {
		c.sleep = sleep
	}
}

// WithTransport sends every request, the Web API calls and the downloads
// alike, through rt instead of the client's own transports, keeping the
// client's timeouts. Tests pass their fake server's own transport: every
// httptest.Server.Close closes the idle connections of http.DefaultTransport,
// which can break a response a client of a parallel test is about to receive
// (Issue #254).
func WithTransport(rt http.RoundTripper) Option {
	return func(c *Client) {
		c.httpClient.Transport = rt
		c.dlClient.Transport = rt
	}
}

// NewDownloadTransport returns the transport New gives the downloads (Issue
// #275): a copy of http.DefaultTransport, which the Web API calls use, that
// keeps as many idle connections per host as a lane runs downloads
// (internal/lane). The downloads of an HTTP/2 origin share the connection its
// lane's first download opened, as long as the server allows as many streams
// at a time as the lane runs downloads (Slack's hosts allow 128); past that,
// net/http opens another connection. It does not make the requests wait for a
// free stream instead (http.HTTP2Config.StrictMaxConcurrentRequests): with
// that, net/http stalls for good once more requests wait for the connection
// than the server allows streams (golang/go#70809, decision log 0067).
// tools/assetbench downloads through a copy of it.
func NewDownloadTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = lane.Defaults.PerOrigin()
	return t
}

// New creates a Slack Web API client for token.
func New(token string, opts ...Option) *Client {
	c := &Client{
		token:         token,
		baseURL:       apiBase,
		httpClient:    &http.Client{Timeout: 120 * time.Second},
		dlClient:      &http.Client{Transport: NewDownloadTransport()},
		headerWait:    downloadHeaderWait,
		stallWait:     downloadStallWait,
		publicTimeout: downloadPublicTimeout,
		lastCall:      map[string]time.Time{},
		sleep:         sleepCtx,
		Logf:          func(string, ...any) {},
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.trace != nil {
		c.trace.install(c)
	}
	return c
}

type apiEnvelope struct {
	OK               bool   `json:"ok"`
	Error            string `json:"error"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func (c *Client) pace(ctx context.Context, key string) error {
	if last, ok := c.lastCall[key]; ok {
		if wait := methodPace - time.Since(last); wait > 0 {
			if err := c.sleep(ctx, wait); err != nil {
				return err
			}
		}
	}
	c.lastCall[key] = time.Now()
	return nil
}

// call POSTs a form-encoded Web API request and decodes the body into out.
func (c *Client) call(ctx context.Context, method string, params url.Values, out any) (string, error) {
	ctx, endTrace := c.traceRequest(ctx, TraceAPI, method)
	defer endTrace()
	if err := c.pace(ctx, method); err != nil {
		return "", fmt.Errorf("slack api %s: %w", method, err)
	}
	var body []byte
	err := c.withRetry(ctx, "api "+method, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+method,
			strings.NewReader(params.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+c.token)
		return c.httpClient.Do(req)
	}, func(resp *http.Response) error {
		// The body is read whole before it is decoded, so a read that fails
		// part way is retried like a network error.
		var err error
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		return err
	})
	if err != nil {
		return "", fmt.Errorf("slack api %s: %w", method, err)
	}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("slack api %s: decode response: %w", method, err)
	}
	if !env.OK {
		return "", &APIError{Method: method, Code: env.Error}
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return "", fmt.Errorf("slack api %s: decode payload: %w", method, err)
		}
	}
	return env.ResponseMetadata.NextCursor, nil
}

// noticesKey is the context key of WithNotices.
type noticesKey struct{}

// WithNotices sends the progress notices of the requests made with ctx — the
// retries and the rate limit waits — to logf instead of Client.Logf. The
// parallel asset fetch holds each download's notices with it, to pass them on
// in plan order (output.Assets.Fetch, Issue #275).
func WithNotices(ctx context.Context, logf func(format string, args ...any)) context.Context {
	return context.WithValue(ctx, noticesKey{}, logf)
}

// noticef returns where the notices of a request made with ctx go.
func (c *Client) noticef(ctx context.Context) func(format string, args ...any) {
	if logf, ok := ctx.Value(noticesKey{}).(func(string, ...any)); ok {
		return logf
	}
	return c.Logf
}

// withRetry sends a request until it gets a 200 response that accept takes,
// honouring 429 + Retry-After and retrying transient failures (5xx, network
// errors) with exponential backoff and jitter, at most maxRetries times. The
// Web API calls and Download share it; what names the request in the
// progress lines, which go where noticef says.
//
// send builds and sends one request, and its error is retried. withRetry
// closes every response it does not pass to accept. accept owns the body of
// the 200 response it gets: it closes the body or hands it on, and its error
// is retried like a network error.
func (c *Client) withRetry(ctx context.Context, what string, send func() (*http.Response, error), accept func(*http.Response) error) error {
	var lastErr error
	logf := c.noticef(ctx)
	// skipBackoff is set when a 429 already waited out Retry-After; the wait
	// happens at detection so it is honoured even when retries are exhausted.
	skipBackoff := false
	for attempt := 0; ; attempt++ {
		if attempt > maxRetries {
			return fmt.Errorf("giving up after %d retries: %w", maxRetries, lastErr)
		}
		if attempt > 0 && !skipBackoff {
			wait := backoffWait(attempt)
			logf("retrying %s in %s (%s)", what, wait.Round(time.Second), lastErr)
			if err := c.sleep(ctx, wait); err != nil {
				return err
			}
		}
		skipBackoff = false
		resp, err := send()
		if err != nil {
			lastErr = err
			continue
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited (429)")
			if wait, ok := retryAfter(resp); ok {
				logf("rate limited on %s, waiting %s as instructed by Slack", what, wait.Round(time.Second))
				if err := c.sleep(ctx, wait); err != nil {
					return err
				}
				skipBackoff = true
			}
			continue
		case resp.StatusCode >= 500:
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: HTTP %d", resp.StatusCode)
			continue
		case resp.StatusCode != http.StatusOK:
			resp.Body.Close()
			return fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
		}
		if err := accept(resp); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
}

// backoffWait is the exponential backoff before the given retry attempt
// (1-based): 1s, 2s, 4s, ... capped at maxBackoff, plus up to 1s of jitter.
func backoffWait(attempt int) time.Duration {
	wait := min(time.Duration(1<<(attempt-1))*time.Second, maxBackoff)
	return wait + time.Duration(rand.Int63n(int64(time.Second)))
}

// retryAfter parses the Retry-After header, adding up to 1s of jitter. It
// reports false when the header is missing or unusable; the caller then
// falls back to exponential backoff.
func retryAfter(resp *http.Response) (time.Duration, bool) {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return time.Duration(secs)*time.Second + time.Duration(rand.Int63n(int64(time.Second))), true
		}
	}
	return 0, false
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Download fetches an asset URL. Slack private file URLs receive the Slack
// OAuth token; public third-party assets such as unfurl images and service
// icons do not.
// When limit > 0 and the body exceeds it, ErrTooLarge is returned.
// Downloads are not paced (decision log 0067): the parallel asset fetch
// bounds them by origin (internal/lane).
func (c *Client) Download(ctx context.Context, srcURL string, limit int64, w io.Writer) (written int64, contentType string, err error) {
	ctx, endTrace := c.traceRequest(ctx, TraceDownload, assetKind(ctx))
	defer endTrace()
	body, ct, err := c.downloadRetry(ctx, srcURL)
	if err != nil {
		return 0, "", err
	}
	defer body.Close()
	reader := io.Reader(body)
	if limit > 0 {
		reader = io.LimitReader(body, limit+1)
	}
	written, err = io.Copy(w, reader)
	if err != nil {
		return written, ct, err
	}
	if limit > 0 && written > limit {
		return written, ct, ErrTooLarge
	}
	return written, ct, nil
}

// downloadRetry sends the GET for srcURL through withRetry and returns the 200
// response's body unread, for the caller to stream and close. A failure while
// streaming is therefore not retried: part of the body may already be written.
func (c *Client) downloadRetry(ctx context.Context, srcURL string) (io.ReadCloser, string, error) {
	// Every attempt sends this one request, under a watchdog of its own
	// (sendDownload): a GET has no body to use up, and withRetry closes each
	// response it rejects before it sends again.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return nil, "", withoutURL(err)
	}
	// A Slack file may be large and share its connection with others for
	// long: its attempts have no overall timeout.
	timeout := c.publicTimeout
	if downloadNeedsAuth(srcURL) {
		req.Header.Set("Authorization", "Bearer "+c.token)
		timeout = 0
	}
	var resp *http.Response
	err = c.withRetry(ctx, "download", func() (*http.Response, error) {
		return c.sendDownload(req, timeout)
	}, func(ok *http.Response) error {
		resp = ok
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

// sendDownload sends one attempt of the download req under a watchdog: the
// attempt fails when its response headers have not come headerWait after the
// request went out, its body when it goes stallWait without a byte, and the
// whole of it, when timeout is not 0, once it has taken timeout. The
// response's body ends the watch when it is closed.
func (c *Client) sendDownload(req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithCancelCause(req.Context())
	wd := &watchdog{cancel: cancel}
	if timeout > 0 {
		wd.limit(timeout, &downloadTimeout{fmt.Sprintf("download took longer than %s", timeout)})
	}
	noHeaders := &downloadTimeout{fmt.Sprintf("no response headers within %s", c.headerWait)}
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		// The wait for response headers lasts until the final response's
		// headers are in, past any interim (1xx) response and headers that
		// come slowly. A redirect's next request pauses it while that gets a
		// connection, and has a wait of its own once written. The HTTP/2
		// transport may report a request written only after its response
		// came: once the body's wait is on, arm leaves it alone.
		GetConn:      func(string) { wd.disarm() },
		WroteRequest: func(httptrace.WroteRequestInfo) { wd.arm(c.headerWait, noHeaders) },
	})
	resp, err := c.dlClient.Do(req.WithContext(ctx))
	if err == nil && wd.timedOut() != nil {
		// The response came after the watchdog canceled the attempt, which
		// has timed out all the same: net/http passes on a response that
		// raced the cancellation, and a server may answer the cancellation
		// itself.
		resp.Body.Close()
		err = &url.Error{Op: "Get", URL: req.URL.String(), Err: context.Canceled}
	}
	if err != nil {
		timeout := wd.stop()
		cancel(nil)
		if timeout != nil {
			// The transport may name the cancellation instead of its cause.
			var urlErr *url.Error
			if errors.As(err, &urlErr) {
				return nil, fmt.Errorf("%s: %w", urlErr.Op, timeout)
			}
			return nil, timeout
		}
		return nil, withoutURL(err)
	}
	wd.armBody(c.stallWait, &downloadTimeout{fmt.Sprintf("download stalled: no data for %s", c.stallWait)})
	resp.Body = &watchedBody{ReadCloser: resp.Body, wd: wd, cancel: cancel}
	return resp, nil
}

// downloadTimeout is the error of a download attempt its watchdog stopped. It
// is a timeout (net.Error), which the HTTP trace records as such.
type downloadTimeout struct{ msg string }

func (e *downloadTimeout) Error() string   { return e.msg }
func (e *downloadTimeout) Timeout() bool   { return true }
func (e *downloadTimeout) Temporary() bool { return true }

// watchdog cancels a download attempt once the time it is armed for passes
// without progress, or once the attempt has taken its limit. An armed
// watchdog that expires cancels the attempt's context with the cause it was
// armed with, and the limit with its own; stop ends the watch and tells
// whether it had.
type watchdog struct {
	cancel context.CancelCauseFunc

	mu       sync.Mutex
	timer    *time.Timer
	armed    bool
	wait     time.Duration
	deadline time.Time
	cause    error
	fired    error // the cause it canceled with
	stopped  bool
	body     bool        // the body's wait is armed
	limiter  *time.Timer // the attempt's limit (limit)
}

// limit cancels the attempt with cause once it has taken wait, whatever its
// progress.
func (w *watchdog) limit(wait time.Duration, cause error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.limiter = time.AfterFunc(wait, func() {
		w.mu.Lock()
		if w.stopped || w.fired != nil {
			w.mu.Unlock()
			return
		}
		w.fired = cause
		w.mu.Unlock()
		w.cancel(cause)
	})
}

// arm starts the wait for progress: the attempt is canceled with cause when
// wait passes without progress. Once armBody has armed the body's wait, arm
// does nothing.
func (w *watchdog) arm(wait time.Duration, cause error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.body {
		w.armLocked(wait, cause)
	}
}

// armBody arms the wait for the body's data, the attempt's last wait: from
// then on, arm and disarm do nothing.
func (w *watchdog) armBody(wait time.Duration, cause error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.body = true
	w.armLocked(wait, cause)
}

func (w *watchdog) armLocked(wait time.Duration, cause error) {
	if w.stopped || w.fired != nil {
		return
	}
	w.armed, w.wait, w.cause = true, wait, cause
	w.deadline = time.Now().Add(wait)
	if w.timer == nil {
		w.timer = time.AfterFunc(wait, w.expire)
	} else {
		w.timer.Reset(wait)
	}
}

// progress starts the wait of an armed watchdog again.
func (w *watchdog) progress() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.armed {
		// The timer is not moved: expire finds the later deadline and waits
		// for the rest, which keeps a read from touching the timer.
		w.deadline = time.Now().Add(w.wait)
	}
}

// disarm pauses the wait for progress until the next arm, unless the body's
// wait is armed. It leaves the limit alone.
func (w *watchdog) disarm() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.body {
		w.armed = false
	}
}

// stop ends the watch and returns the cause the watchdog canceled with, or
// nil when it had not.
func (w *watchdog) stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopped = true
	if w.timer != nil {
		w.timer.Stop()
	}
	if w.limiter != nil {
		w.limiter.Stop()
	}
	return w.fired
}

// timedOut returns the cause the watchdog canceled with, or nil.
func (w *watchdog) timedOut() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.fired
}

func (w *watchdog) expire() {
	w.mu.Lock()
	if w.stopped || !w.armed || w.fired != nil {
		w.mu.Unlock()
		return
	}
	if rest := time.Until(w.deadline); rest > 0 {
		w.timer.Reset(rest)
		w.mu.Unlock()
		return
	}
	cause := w.cause
	w.fired = cause
	w.mu.Unlock()
	w.cancel(cause)
}

// watchedBody is the body of a watched download attempt: each read that
// brings data restarts the watchdog's wait, a read that fails once the
// watchdog fired fails with its timeout, and Close ends the watch.
type watchedBody struct {
	io.ReadCloser
	wd     *watchdog
	cancel context.CancelCauseFunc
}

func (b *watchedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.wd.progress()
	}
	if err != nil && err != io.EOF {
		if timeout := b.wd.timedOut(); timeout != nil {
			err = timeout
		}
	}
	return n, err
}

func (b *watchedBody) Close() error {
	err := b.ReadCloser.Close()
	b.wd.stop()
	b.cancel(nil)
	return err
}

// withoutURL drops the URL that net/http puts in the text of a failed
// request's error (*url.Error, `Get "<url>": <cause>`), keeping the operation
// and the cause (`Get: <cause>`). A download's error reaches stderr in the
// retry notices and the asset warning, and the URL of an upload is a Slack
// private file URL, which must not be shown (cli-interface.md 出力制御).
func withoutURL(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	return fmt.Errorf("%s: %w", urlErr.Op, urlErr.Err)
}

func downloadNeedsAuth(srcURL string) bool {
	u, err := url.Parse(srcURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Hostname(), "files.slack.com")
}
