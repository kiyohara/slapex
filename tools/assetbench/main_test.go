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

	"github.com/kiyohara/slapex/internal/lane"
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

// strategyNamed is the strategy of that name.
func strategyNamed(t *testing.T, name string) strategy {
	t.Helper()
	chosen, err := pickStrategies(name)
	if err != nil {
		t.Fatal(err)
	}
	return chosen[0]
}

// TestUnpacedRun: the origins serve what the workload says, with its HTTP
// versions, delays and bandwidth, and the unpaced strategy does not wait.
func TestUnpacedRun(t *testing.T) {
	t.Parallel()

	b := startBench(tinyWorkload())
	defer b.close()
	res, trace, err := b.run(context.Background(), strategyNamed(t, "unpaced"), 1, lane.Defaults)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// The trace times the requests with the clock, so a pacing wait of no
	// time can come out as a few microseconds.
	if res.saved != 3 || res.notSaved != 0 || res.requests != 3 || res.peak != 1 || res.times.PacingWait >= time.Millisecond {
		t.Fatalf("result = %+v, want 3 assets saved with 3 requests, one at a time, and no pacing", res)
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
	writeReport(&out, b.w, lane.Defaults, []result{res})
	for _, want := range []string{"Workload \"tiny\": 3 assets (0.4 MB) in 3 requests to 2 origins (Slack CDN 2 on 1, other 1 on 1); HTTP/2 origins 1, HTTP/1.1 origins 1.",
		"| unpaced (serial, no pacing) | 1 |", "| 3 | 2 | 1 | ", " | 0 | 1.0× |"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report misses %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "Lanes:") {
		t.Errorf("report of a serial run tells the lane limits:\n%s", out.String())
	}
}

// TestPacedRun: the paced strategy starts the downloads 1 s apart.
func TestPacedRun(t *testing.T) {
	t.Parallel()

	b := startBench(tinyWorkload())
	defer b.close()
	res, _, err := b.run(context.Background(), strategyNamed(t, "paced"), 1, lane.Defaults)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.saved != 3 || res.peak != 1 || res.wall < 2*time.Second || res.times.PacingWait < time.Second {
		t.Fatalf("result = %+v, want 3 assets one at a time over at least 2s, most of it pacing", res)
	}
}

// TestParallelRun: the parallel strategy downloads from the origins at the
// same time, without pacing; the downloads of the HTTP/2 origin share one
// connection, and a redirected asset is saved from where it was redirected
// to.
func TestParallelRun(t *testing.T) {
	t.Parallel()

	w := tinyWorkload()
	gravatar := len(w.Origins)
	w.Origins = append(w.Origins, Origin{Class: "gravatar", HTTP2: true})
	w.Assets = append(w.Assets, Asset{Origin: 1, Kind: output.KindAvatar, Bytes: 9 * kib, Via: &gravatar})
	b := startBench(w)
	defer b.close()
	res, trace, err := b.run(context.Background(), strategyNamed(t, "parallel"), 1, lane.Defaults)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.saved != 4 || res.notSaved != 0 || res.requests != 5 || res.peak < 2 || res.times.PacingWait >= time.Millisecond {
		t.Fatalf("result = %+v, want 4 assets saved with 5 requests, some at the same time, and no pacing", res)
	}
	if got := b.origins[0].conns.Load(); got != 1 {
		t.Errorf("the HTTP/2 origin got %d connections, want 1", got)
	}
	var statuses []int
	for _, rec := range records(t, trace) {
		if rec.Kind == output.KindAvatar && rec.Bytes < 100*kib {
			statuses = append(statuses, rec.Status)
		}
	}
	if !slices.Equal(statuses, []int{http.StatusFound, http.StatusOK}) {
		t.Errorf("the redirected avatar's requests got %v, want a redirect and 200", statuses)
	}

	var out bytes.Buffer
	writeReport(&out, b.w, lane.Defaults, []result{res})
	for _, want := range []string{"in 5 requests to 3 origins (Slack CDN 2 on 1, gravatar 1 on 1, other 2 on 1)",
		"Lanes: 16 downloads at a time from an HTTP/2 origin, 6 from an HTTP/1.1 origin, 64 in all; 4 of 4 MiB or more from one origin.",
		"| parallel (origin lanes, from #275) | 1 |"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report misses %q:\n%s", want, out.String())
		}
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

// TestTracedWorkload pins the preset to the trace of #272: its requests per
// class and origin, HTTP versions and asset kinds.
func TestTracedWorkload(t *testing.T) {
	w := tracedWorkload()
	if err := w.validate(); err != nil {
		t.Fatal(err)
	}
	type class struct{ requests, origins int }
	perClass := map[string]class{}
	perOrigin := map[int]int{}
	perKind := map[string]int{}
	request := func(origin int) {
		c := perClass[w.Origins[origin].Class]
		if perOrigin[origin] == 0 {
			c.origins++
		}
		c.requests++
		perClass[w.Origins[origin].Class] = c
		perOrigin[origin]++
	}
	for _, a := range w.Assets {
		if a.Via != nil {
			request(*a.Via)
		}
		request(a.Origin)
		perKind[a.Kind]++
	}
	wantClass := map[string]class{"files.slack.com": {4, 1}, "Slack CDN": {15, 2}, "gravatar": {4, 1}, "other": {36, 23}}
	if len(w.Assets) != 56 || len(w.Origins) != 27 || !maps.Equal(perClass, wantClass) {
		t.Errorf("traced = %d assets from %d origins, per class %v; want 56 from 27, %v",
			len(w.Assets), len(w.Origins), perClass, wantClass)
	}
	others := map[int]int{} // origins by their requests
	http1 := 0
	for i, o := range w.Origins {
		if o.Class == "other" {
			others[perOrigin[i]]++
		}
		if !o.HTTP2 {
			http1++
		}
	}
	if want := map[int]int{4: 1, 2: 10, 1: 12}; !maps.Equal(others, want) || http1 != 3 {
		t.Errorf("third-party origins by requests = %v, HTTP/1.1 origins %d; want %v and 3", others, http1, want)
	}
	wantKind := map[string]int{output.KindOGImage: 19, output.KindAvatar: 13, output.KindServiceIcon: 14,
		output.KindUploadThumb: 2, output.KindEmoji: 5, output.KindUploadOriginal: 2, output.KindWorkspaceIcon: 1}
	if !maps.Equal(perKind, wantKind) {
		t.Errorf("traced assets per kind = %v, want %v", perKind, wantKind)
	}
	if err := heavyWorkload().validate(); err != nil {
		t.Errorf("heavy: %v", err)
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
	if err := run(&out, "small", true, "", 1, "", lane.Defaults); err != nil {
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
		"self-via.json":  `{"origins":[{"class":"other"}],"assets":[{"origin":0,"kind":"avatar","bytes":1,"via":0}]}`,
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
	if _, err := pickStrategies("paced,current"); err == nil {
		t.Error("pickStrategies accepted an unknown strategy")
	}
}
