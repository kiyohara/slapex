package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// tinyWorkload has an HTTP/2 origin with delays and a bandwidth, and an
// HTTP/1.1 origin without.
func tinyWorkload() Workload {
	return Workload{
		Name: "tiny",
		Origins: []Origin{
			{Class: "Slack CDN", HTTP2: true, HandshakeMS: 30, FirstByteMS: 20, BytesPerSec: 1 * mib},
			{Class: "other", HTTP2: false},
		},
		Assets: []Asset{
			{Origin: 0, Kind: output.KindAvatar, Bytes: 256 * kib},
			{Origin: 1, Kind: output.KindOGImage, Bytes: 100 * kib},
			{Origin: 0, Kind: output.KindEmoji, Bytes: 5 * kib},
		},
	}
}

func records(t *testing.T, trace []byte) []slack.TraceRecord {
	t.Helper()
	var recs []slack.TraceRecord
	for line := range bytes.Lines(trace) {
		var rec slack.TraceRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("trace line %q: %v", line, err)
		}
		recs = append(recs, rec)
	}
	return recs
}

func us(p *int64) time.Duration {
	if p == nil {
		return -1
	}
	return time.Duration(*p) * time.Microsecond
}

// TestUnpacedRun: the origins serve what the workload says, with its HTTP
// versions, delays and bandwidth, and the unpaced strategy does not wait.
func TestUnpacedRun(t *testing.T) {
	t.Parallel()

	b := startBench(tinyWorkload())
	defer b.close()
	res, trace, err := b.run(context.Background(), strategies[1], 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.saved != 3 || res.notSaved != 0 || res.requests != 3 || res.times.PacingWait != 0 {
		t.Fatalf("result = %+v, want 3 assets saved with 3 requests and no pacing", res)
	}
	recs := records(t, trace)
	if len(recs) != 3 {
		t.Fatalf("trace has %d lines, want 3", len(recs))
	}
	for i, want := range []struct {
		proto  string
		bytes  int64
		reused bool
	}{{"HTTP/2.0", 256 * kib, false}, {"HTTP/1.1", 100 * kib, false}, {"HTTP/2.0", 5 * kib, true}} {
		if got := recs[i]; got.Proto != want.proto || got.Bytes != want.bytes || got.ConnReused != want.reused || got.Status != http.StatusOK {
			t.Errorf("line %d = %s %d, %d bytes, reused %v; want %s 200, %d bytes, reused %v",
				i, got.Proto, got.Status, got.Bytes, got.ConnReused, want.proto, want.bytes, want.reused)
		}
	}
	first := recs[0]
	if tls := us(first.TLSDoneUS) - us(first.TLSStartUS); tls < 30*time.Millisecond {
		t.Errorf("TLS handshake took %v, want at least the origin's 30ms", tls)
	}
	if wait := us(first.FirstByteUS) - us(first.GotConnUS); wait < 20*time.Millisecond {
		t.Errorf("first byte came %v after the connection, want at least the origin's 20ms", wait)
	}
	// 256 KiB at 1 MiB/s takes 250ms, of which the first chunk (31ms) may
	// come before the first byte is seen.
	if transfer := first.Times().Transfer; transfer < 200*time.Millisecond {
		t.Errorf("transfer took %v, want about 250ms at the origin's bandwidth", transfer)
	}

	var out bytes.Buffer
	writeReport(&out, b.w, []result{res})
	for _, want := range []string{"Workload \"tiny\": 3 assets (0.4 MB) from 2 origins (Slack CDN 2 on 1, other 1 on 1); HTTP/2 origins 1, HTTP/1.1 origins 1.",
		"| unpaced (serial, no pacing) | 1 |", "| 3 | 2 | 0 | 1.0× |"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report misses %q:\n%s", want, out.String())
		}
	}
}

// TestCurrentRunPaces: the current strategy starts the downloads 1 s apart.
func TestCurrentRunPaces(t *testing.T) {
	t.Parallel()

	b := startBench(tinyWorkload())
	defer b.close()
	res, _, err := b.run(context.Background(), strategies[0], 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.saved != 3 || res.wall < 2*time.Second || res.times.PacingWait < time.Second {
		t.Fatalf("result = %+v, want 3 assets over at least 2s, most of it pacing", res)
	}
}

// TestMaxStreams: an HTTP/2 origin that allows one stream makes the client
// open a second connection for a second request at the same time; with Go's
// default limit the two share one.
func TestMaxStreams(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		maxStreams int
		wantConns  int64
	}{{maxStreams: 1, wantConns: 2}, {maxStreams: 0, wantConns: 1}} {
		o := startOrigin(Origin{Class: "other", HTTP2: true, FirstByteMS: 200, MaxStreams: tc.maxStreams})
		defer o.srv.Close()
		tr := newTransport(certPool([]*fakeOrigin{o}))
		defer tr.CloseIdleConnections()
		client := &http.Client{Transport: tr}
		get := func() {
			resp, err := client.Get(o.url(0, 10))
			if err != nil {
				t.Errorf("GET: %v", err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		// The first request opens the connection and learns its limit.
		get()
		var wg sync.WaitGroup
		for range 2 {
			wg.Go(get)
		}
		wg.Wait()
		if got := o.conns.Load(); got != tc.wantConns {
			t.Errorf("max streams %d: %d connections, want %d", tc.maxStreams, got, tc.wantConns)
		}
	}
}

// TestRecentWorkload pins the preset to the export measured for #272.
func TestRecentWorkload(t *testing.T) {
	w := recentWorkload()
	if err := w.validate(); err != nil {
		t.Fatal(err)
	}
	perClass := map[string]int{}
	perOrigin := map[int]int{}
	for _, a := range w.Assets {
		perClass[w.Origins[a.Origin].Class]++
		perOrigin[a.Origin]++
	}
	want := map[string]int{"files.slack.com": 4, "Slack CDN": 14, "gravatar": 5, "other": 33}
	if len(w.Assets) != 56 || len(w.Origins) != 25 || len(perOrigin) != 25 || !maps.Equal(perClass, want) {
		t.Errorf("recent = %d assets from %d origins (%d used), per class %v; want 56 from 25, %v",
			len(w.Assets), len(w.Origins), len(perOrigin), perClass, want)
	}
	if most := slices.Max(slices.Collect(maps.Values(perOrigin))); most != 9 {
		t.Errorf("most assets on one origin = %d, want 9", most)
	}
	if err := smallWorkload().validate(); err != nil {
		t.Errorf("small: %v", err)
	}
}

// TestWorkloadFile: a printed workload loads back; a broken one does not.
func TestWorkloadFile(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := run(&out, "small", true, "", 1, ""); err != nil {
		t.Fatalf("print: %v", err)
	}
	path := filepath.Join(dir, "small.json")
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := loadWorkload(path)
	if err != nil || len(w.Assets) != 8 || w.Name != "small" {
		t.Fatalf("loadWorkload = %d assets named %q, %v; want the small preset", len(w.Assets), w.Name, err)
	}

	for name, content := range map[string]string{
		"no-assets.json": `{"origins":[{"class":"other"}],"assets":[]}`,
		"no-origin.json": `{"origins":[],"assets":[{"origin":0,"kind":"avatar","bytes":1}]}`,
		"bad-kind.json":  `{"origins":[{"class":"other"}],"assets":[{"origin":0,"kind":"video","bytes":1}]}`,
		"not-json.json":  `origins: []`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadWorkload(path); err == nil {
			t.Errorf("loadWorkload(%s) succeeded", name)
		}
	}
	if _, err := loadWorkload("no-such-preset"); err == nil {
		t.Error("loadWorkload(no-such-preset) succeeded")
	}
	if _, err := pickStrategies("current,parallel"); err == nil {
		t.Error("pickStrategies accepted an unknown strategy")
	}
}
