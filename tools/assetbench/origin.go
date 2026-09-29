package main

// Fake origins: one httptest server over TLS per workload origin. Each serves
// /<id>/<bytes> with bytes of content, after the origin's handshake and
// first-byte delays, at the origin's bandwidth, with its HTTP/2 stream limit,
// and redirects /redirect?to=<URL> to the URL after its first-byte delay.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// chunkSize is how much of a body the origin sends at a time.
const chunkSize = 32 * kib

type fakeOrigin struct {
	cfg   Origin
	srv   *httptest.Server
	link  *link
	conns atomic.Int64 // connections accepted
}

func startOrigin(cfg Origin) *fakeOrigin {
	o := &fakeOrigin{cfg: cfg, link: &link{bytesPerSec: cfg.BytesPerSec}}
	srv := httptest.NewUnstartedServer(o)
	srv.EnableHTTP2 = cfg.HTTP2
	handshake := time.Duration(cfg.HandshakeMS) * time.Millisecond
	srv.TLS = &tls.Config{
		// Runs in the handshake, after the ClientHello; nil keeps the
		// server's own configuration.
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			time.Sleep(handshake)
			return nil, nil
		},
	}
	if cfg.MaxStreams > 0 {
		srv.Config.HTTP2 = &http.HTTP2Config{MaxConcurrentStreams: cfg.MaxStreams}
	}
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			o.conns.Add(1)
		}
	}
	// A client may drop a connection it dialed but did not need.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	o.srv = srv
	return o
}

// url is where the origin serves asset id of n bytes.
func (o *fakeOrigin) url(id int, n int64) string {
	return fmt.Sprintf("%s/%d/%d", o.srv.URL, id, n)
}

// redirect is where the origin redirects to target.
func (o *fakeOrigin) redirect(target string) string {
	return o.srv.URL + "/redirect?to=" + url.QueryEscape(target)
}

func (o *fakeOrigin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	firstByte := time.Duration(o.cfg.FirstByteMS) * time.Millisecond
	if r.URL.Path == "/redirect" {
		if sleep(r.Context(), firstByte) {
			http.Redirect(w, r, r.URL.Query().Get("to"), http.StatusFound)
		}
		return
	}
	var id int
	var n int64
	if _, err := fmt.Sscanf(r.URL.Path, "/%d/%d", &id, &n); err != nil || n < 0 {
		http.NotFound(w, r)
		return
	}
	if !sleep(r.Context(), firstByte) {
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(n, 10))
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	rc.Flush()
	// The content differs per asset, as the saved files are named by it.
	chunk := make([]byte, chunkSize)
	for i := range chunk {
		chunk[i] = byte(id + i)
	}
	for sent := int64(0); sent < n; {
		size := min(int64(len(chunk)), n-sent)
		if !o.link.transmit(r.Context(), size) {
			return
		}
		if _, err := w.Write(chunk[:size]); err != nil {
			return
		}
		rc.Flush()
		sent += size
	}
}

// link is an origin's bandwidth. The chunks of all its responses take turns,
// each for the time its size takes at the link's rate.
type link struct {
	bytesPerSec int64

	mu   sync.Mutex
	free time.Time // when the link has sent everything it was given
}

// transmit waits until the link has sent n more bytes. It reports false when
// ctx ends first.
func (l *link) transmit(ctx context.Context, n int64) bool {
	if l.bytesPerSec <= 0 {
		return ctx.Err() == nil
	}
	l.mu.Lock()
	start := time.Now()
	if l.free.After(start) {
		start = l.free
	}
	l.free = start.Add(time.Duration(n * int64(time.Second) / l.bytesPerSec))
	done := l.free
	l.mu.Unlock()
	return sleep(ctx, time.Until(done))
}

// sleep waits for d, and reports false when ctx ends first.
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// certPool trusts the origins' certificate.
func certPool(origins []*fakeOrigin) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, o := range origins {
		pool.AddCert(o.srv.Certificate())
	}
	return pool
}

// newTransport is a fresh copy of the transport slapex downloads with
// (slack.NewDownloadTransport), which trusts the fake origins and never goes
// through a proxy.
func newTransport(pool *x509.CertPool) *http.Transport {
	tr := slack.NewDownloadTransport()
	tr.Proxy = nil
	tr.TLSClientConfig = &tls.Config{RootCAs: pool}
	return tr
}
