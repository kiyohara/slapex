package slack

// HTTP trace (Issue #273, PF-01 of #272). With WithTrace the client writes one
// JSON line (TraceRecord) per HTTP request it sends: every attempt of a Web API
// call or a download, and every redirect hop, with the request's time split
// into its phases (net/http/httptrace) and the pacing and retry waits around
// it. slapex turns it on with the internal SLAPEX_HTTP_TRACE variable
// (doc/design/cli-interface.md); tools/tracereport summarizes a trace and
// tools/assetbench reads its own.
//
// The trace keeps no URL, header, token or body. A request is known by its
// scheme and host and by a hash of its URL keyed per client, which tells the
// URLs of one trace apart but cannot be matched against a known URL.
//
// The tracing wraps the client's transport and sleeper and leaves withRetry
// alone. call and Download put a requestTrace for the call or download in the
// context (traceRequest); the transport starts a record for each request it is
// handed, and the sleeper adds each wait to the call or download it belongs
// to. A wait before the first request is its pacing wait; a later one is the
// retry wait that the latest request's failure caused (a backoff or a
// Retry-After). A record is therefore written when the next request starts or
// when the call or download ends.

import (
	"context"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"
)

// TraceRecord.Type values.
const (
	TraceAPI      = "api"
	TraceDownload = "download"
)

// TraceRecord is one line of the HTTP trace: one HTTP request, which is one
// attempt of a Web API call or a download, or one redirect hop of an attempt.
// The *US offsets are microseconds from Start; a phase that did not happen has
// none.
type TraceRecord struct {
	// Start is when the request was handed to the transport, in UTC.
	Start time.Time `json:"start"`
	// Type is TraceAPI or TraceDownload.
	Type string `json:"type"`
	// Method is the Web API method of a call. Kind is the asset kind of a
	// download (output.Kind*), when its caller gave one (WithAssetKind).
	Method string `json:"method,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Scheme string `json:"scheme"`
	// Host is the URL's host, lowercased, with the port when the URL has one.
	Host string `json:"host"`
	// URLHash tells the URLs of one trace apart (see the comment above).
	URLHash string `json:"url_hash"`
	// Attempt counts the retries before this request: 0 for the first try.
	// Redirect counts the redirect hops before it within its attempt.
	Attempt  int `json:"attempt"`
	Redirect int `json:"redirect,omitempty"`
	// PacingWaitUS is the pacing wait before the first attempt.
	PacingWaitUS int64 `json:"pacing_wait_us,omitempty"`
	// Proto and Status come from the response. Error classes a failure of the
	// request or of reading its body (errorClass, or timeout once the
	// request's deadline has passed); it holds no error text.
	Proto          string `json:"proto,omitempty"`
	Status         int    `json:"status,omitempty"`
	Error          string `json:"error,omitempty"`
	DNSStartUS     *int64 `json:"dns_start_us,omitempty"`
	DNSDoneUS      *int64 `json:"dns_done_us,omitempty"`
	ConnectStartUS *int64 `json:"connect_start_us,omitempty"`
	ConnectDoneUS  *int64 `json:"connect_done_us,omitempty"`
	TLSStartUS     *int64 `json:"tls_start_us,omitempty"`
	TLSDoneUS      *int64 `json:"tls_done_us,omitempty"`
	// GotConnUS is when the request had a connection; ConnReused is set when
	// an earlier request had opened it.
	GotConnUS   *int64 `json:"got_conn_us,omitempty"`
	ConnReused  bool   `json:"conn_reused,omitempty"`
	FirstByteUS *int64 `json:"first_byte_us,omitempty"`
	// DoneUS is when the body was read to its end or closed, or when the
	// request or a body read failed.
	DoneUS *int64 `json:"done_us,omitempty"`
	// Bytes is how much of the response body was read, after the transport's
	// transparent gzip decoding.
	Bytes int64 `json:"bytes"`
	// RetryWaitUS is the wait after this request failed, before the next
	// attempt or before giving up: a backoff or a Retry-After.
	RetryWaitUS int64 `json:"retry_wait_us,omitempty"`
}

// TraceTimes splits the time of traced requests into the parts that a trace
// summary adds up.
type TraceTimes struct {
	PacingWait time.Duration // before the first attempt of a call or download
	RetryWait  time.Duration // after a failed request, before the next attempt or giving up
	Connect    time.Duration // until the request had a connection: DNS, TCP and TLS, or taking an idle one
	FirstByte  time.Duration // from having the connection to the first byte of the response
	Transfer   time.Duration // from the first byte of the response to the end of the body
}

// Total is the time of all the parts.
func (t TraceTimes) Total() time.Duration {
	return t.PacingWait + t.RetryWait + t.Connect + t.FirstByte + t.Transfer
}

// Add returns t and u added part by part.
func (t TraceTimes) Add(u TraceTimes) TraceTimes {
	return TraceTimes{
		PacingWait: t.PacingWait + u.PacingWait,
		RetryWait:  t.RetryWait + u.RetryWait,
		Connect:    t.Connect + u.Connect,
		FirstByte:  t.FirstByte + u.FirstByte,
		Transfer:   t.Transfer + u.Transfer,
	}
}

// Times splits the record's time. The phases of a request follow one another:
// Connect until the connection, FirstByte until the first byte of the
// response, Transfer until the end. A request that failed earlier spent the
// rest of its time in the phase it was in.
func (r TraceRecord) Times() TraceTimes {
	t := TraceTimes{
		PacingWait: time.Duration(r.PacingWaitUS) * time.Microsecond,
		RetryWait:  time.Duration(r.RetryWaitUS) * time.Microsecond,
	}
	done := offset(r.DoneUS)
	switch {
	case r.GotConnUS == nil:
		t.Connect = done
	case r.FirstByteUS == nil:
		t.Connect = offset(r.GotConnUS)
		t.FirstByte = max(done-t.Connect, 0)
	default:
		t.Connect = offset(r.GotConnUS)
		t.FirstByte = max(offset(r.FirstByteUS)-t.Connect, 0)
		t.Transfer = max(done-offset(r.FirstByteUS), 0)
	}
	return t
}

func offset(us *int64) time.Duration {
	if us == nil {
		return 0
	}
	return time.Duration(*us) * time.Microsecond
}

// WithTrace writes the HTTP trace to w, one JSON line (TraceRecord) per HTTP
// request the client sends, each line in one Write. The client ignores write
// errors: a writer that must report them keeps them itself.
func WithTrace(w io.Writer) Option {
	return func(c *Client) {
		c.trace = newTracer(w)
	}
}

type assetKindKey struct{}

// WithAssetKind labels the downloads made with ctx as assets of kind in the
// HTTP trace. It changes nothing else.
func WithAssetKind(ctx context.Context, kind string) context.Context {
	return context.WithValue(ctx, assetKindKey{}, kind)
}

func assetKind(ctx context.Context) string {
	kind, _ := ctx.Value(assetKindKey{}).(string)
	return kind
}

// tracer writes the trace of one client.
type tracer struct {
	key []byte // keys the URL hash; random per client and never written

	mu sync.Mutex
	w  io.Writer
}

func newTracer(w io.Writer) *tracer {
	key := make([]byte, 32)
	crand.Read(key) // never returns an error
	return &tracer{key: key, w: w}
}

// install wraps c's transport and sleeper, once the options have set them.
func (t *tracer) install(c *Client) {
	next := c.httpClient.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	c.httpClient.Transport = &traceTransport{t: t, next: next}
	sleep := c.sleep
	c.sleep = func(ctx context.Context, d time.Duration) error {
		start := time.Now()
		err := sleep(ctx, d)
		if r, ok := ctx.Value(requestTraceKey{}).(*requestTrace); ok {
			r.waited(time.Since(start))
		}
		return err
	}
}

func (t *tracer) urlHash(u string) string {
	mac := hmac.New(sha256.New, t.key)
	mac.Write([]byte(u))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}

func (t *tracer) write(rec TraceRecord) {
	line, err := json.Marshal(rec)
	if err != nil {
		return // a TraceRecord always marshals
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.w.Write(append(line, '\n'))
}

type requestTraceKey struct{}

// traceRequest labels one Web API call (TraceAPI with its method) or one
// download (TraceDownload with its asset kind) in ctx, and returns the
// function that ends it by writing its last record. Without a trace it returns
// ctx as it is.
func (c *Client) traceRequest(ctx context.Context, typ, label string) (context.Context, func()) {
	if c.trace == nil {
		return ctx, func() {}
	}
	r := &requestTrace{t: c.trace, typ: typ, label: label}
	return context.WithValue(ctx, requestTraceKey{}, r), r.end
}

// requestTrace follows one Web API call or download across its requests.
type requestTrace struct {
	t     *tracer
	typ   string
	label string

	mu       sync.Mutex
	pacing   time.Duration // waited before the first request
	attempt  int
	redirect int
	last     *traceLine // written when the next request starts or at end
}

// start begins the line of req, the next request of r. It first writes the
// previous line, which is complete by then: net/http follows a redirect, and
// withRetry retries, only after closing the earlier response's body.
func (r *requestTrace) start(req *http.Request) *traceLine {
	rec := TraceRecord{
		Type:    r.typ,
		Scheme:  req.URL.Scheme,
		Host:    strings.ToLower(req.URL.Host),
		URLHash: r.t.urlHash(req.URL.String()),
	}
	if r.typ == TraceAPI {
		rec.Method = r.label
	} else {
		rec.Kind = r.label
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last == nil {
		rec.PacingWaitUS = r.pacing.Microseconds()
	} else {
		if req.Response != nil {
			r.redirect++
		} else {
			r.attempt++
			r.redirect = 0
		}
		r.t.write(r.last.finish())
	}
	rec.Attempt, rec.Redirect = r.attempt, r.redirect
	r.last = &traceLine{rec: rec}
	r.last.begin = time.Now()
	r.last.rec.Start = r.last.begin.UTC()
	r.last.deadline, _ = req.Context().Deadline()
	return r.last
}

// waited adds a wait of the call or download: the pacing wait before its first
// request, or the retry wait after its latest one.
func (r *requestTrace) waited(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last == nil {
		r.pacing += d
		return
	}
	r.last.addRetryWait(d)
}

// end writes the last line.
func (r *requestTrace) end() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last != nil {
		r.t.write(r.last.finish())
		r.last = nil
	}
}

// traceLine is the record of one request while it is under way. The httptrace
// hooks may run on other goroutines, some even after the request is over (a
// dial that lost a race), so every change takes mu and none lands once the
// line is finished.
type traceLine struct {
	mu        sync.Mutex
	begin     time.Time
	deadline  time.Time // the request's, from the client's timeout or the caller
	rec       TraceRecord
	retryWait time.Duration
	finished  bool
}

// mark sets *field to the offset of now, unless it is set already.
func (l *traceLine) mark(field **int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.markLocked(field)
}

func (l *traceLine) markLocked(field **int64) {
	if l.finished || *field != nil {
		return
	}
	us := time.Since(l.begin).Microseconds()
	*field = &us
}

func (l *traceLine) hooks() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart:     func(httptrace.DNSStartInfo) { l.mark(&l.rec.DNSStartUS) },
		DNSDone:      func(httptrace.DNSDoneInfo) { l.mark(&l.rec.DNSDoneUS) },
		ConnectStart: func(string, string) { l.mark(&l.rec.ConnectStartUS) },
		ConnectDone: func(_, _ string, err error) {
			if err == nil {
				l.mark(&l.rec.ConnectDoneUS)
			}
		},
		TLSHandshakeStart: func() { l.mark(&l.rec.TLSStartUS) },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err == nil {
				l.mark(&l.rec.TLSDoneUS)
			}
		},
		GotConn: func(info httptrace.GotConnInfo) {
			l.mu.Lock()
			defer l.mu.Unlock()
			if l.finished || l.rec.GotConnUS != nil {
				return
			}
			l.markLocked(&l.rec.GotConnUS)
			l.rec.ConnReused = info.Reused
		},
		GotFirstResponseByte: func() { l.mark(&l.rec.FirstByteUS) },
	}
}

func (l *traceLine) responded(resp *http.Response) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rec.Proto, l.rec.Status = resp.Proto, resp.StatusCode
}

// failed records a failure of the request or of a body read, which ends it.
// A failure once the request's deadline has passed is a timeout, whatever the
// error says: http.Client stops a request it sends through a transport it does
// not know, as traceTransport is, both by its context and by Request.Cancel,
// and the transport reports whichever it notices first ("net/http: request
// canceled" for the latter).
func (l *traceLine) failed(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.finished || l.rec.Error != "" {
		return
	}
	l.rec.Error = errorClass(err)
	if !l.deadline.IsZero() && !time.Now().Before(l.deadline) {
		l.rec.Error = "timeout"
	}
	l.markLocked(&l.rec.DoneUS)
}

// read counts the n bytes of a body read. The end of the body ends the request.
func (l *traceLine) read(n int, err error) {
	l.mu.Lock()
	l.rec.Bytes += int64(n)
	if err == io.EOF {
		l.markLocked(&l.rec.DoneUS)
	}
	l.mu.Unlock()
	if err != nil && err != io.EOF {
		l.failed(err)
	}
}

func (l *traceLine) addRetryWait(d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.retryWait += d
}

// finish ends the line and returns its record.
func (l *traceLine) finish() TraceRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.finished = true
	l.rec.RetryWaitUS = l.retryWait.Microseconds()
	return l.rec
}

// errorClass names the kind of a failed request or body read. The error text
// is left out: it can hold a host or an address.
func errorClass(err error) string {
	var dnsErr *net.DNSError
	var netErr net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.As(err, &dnsErr):
		return "dns"
	case errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	default:
		return "network"
	}
}

// traceTransport records each request it sends in the requestTrace of the
// request's context. http.Client does not know this transport, so it stops a
// request at its Timeout by Request.Cancel as well as by the context: the
// request stops at the same time, but the error text can differ from a run
// without the trace (cli-interface.md).
type traceTransport struct {
	t    *tracer
	next http.RoundTripper
}

func (tt *traceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r, labeled := req.Context().Value(requestTraceKey{}).(*requestTrace)
	if !labeled {
		// A request that no call or download labeled is still recorded, on
		// its own, and written when it ends.
		r = &requestTrace{t: tt.t}
	}
	line := r.start(req)
	resp, err := tt.next.RoundTrip(req.WithContext(httptrace.WithClientTrace(req.Context(), line.hooks())))
	if err != nil {
		line.failed(err)
		if !labeled {
			r.end()
		}
		return nil, err
	}
	line.responded(resp)
	body := &traceBody{ReadCloser: resp.Body, line: line}
	if !labeled {
		body.end = r.end
	}
	resp.Body = body
	return resp, nil
}

// traceBody counts the bytes read from a response body and marks its end.
type traceBody struct {
	io.ReadCloser
	line *traceLine
	end  func() // ends a request that no call or download labeled; nil otherwise
}

func (b *traceBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.line.read(n, err)
	return n, err
}

func (b *traceBody) Close() error {
	err := b.ReadCloser.Close()
	b.line.mark(&b.line.rec.DoneUS)
	if b.end != nil {
		b.end()
	}
	return err
}
