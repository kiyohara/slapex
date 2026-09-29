// Command assetbench compares ways of downloading the assets of an export
// (Issue #273, PF-01 of #272). It serves a workload's assets from in-process
// fake origins (httptest over TLS, HTTP/2 or HTTP/1.1, each with its own
// handshake and first-byte delays, bandwidth and HTTP/2 stream limit) and
// downloads them through slapex's own client, download transport and asset
// store (internal/slack, internal/output) with each strategy:
//
//   - paced: one at a time, in the order the page asks for them, 1 s apart,
//     as slapex downloaded before #275. The client no longer paces the
//     downloads, so the benchmark waits in its place.
//   - unpaced: one at a time, in the same order, without the pacing.
//   - parallel: as slapex downloads from #275 (PF-03): the page's assets are
//     planned, then fetched in parallel, in one lane per origin within the
//     lane limits (internal/lane), which the -h2, -h1, -total, -large and
//     -large-size flags set.
//
// Every run traces its requests (slack.WithTrace), and the report splits each
// run's wall time the way tools/tracereport does. The origins are models, not
// measurements: the report compares strategies, and its absolute times are
// only as good as the workload.
//
// Usage:
//
//	docker compose run --rm dev go run ./tools/assetbench [-workload traced] [-strategies paced,unpaced,parallel] [-runs 1]
//
// -workload takes a preset (traced, heavy, recent, small) or a JSON file, and
// -print-workload prints the workload as JSON to start a file from.
// -trace-dir keeps the trace of every run, for tools/tracereport.
package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/lane"
	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// maxAttachment is slapex's default --max-attachment-size.
const maxAttachment = 10 * mib

// A strategy is a way of downloading the workload.
type strategy struct {
	name  string
	about string
	// pace is the wait between the starts of two downloads, and 0 none.
	pace time.Duration
	// parallel plans the assets and fetches the plan in lanes
	// (output.Assets.Fetch); otherwise each asset is downloaded when the page
	// asks for it, one at a time.
	parallel bool
}

var strategies = []strategy{
	{name: "paced", about: "serial, 1 s pacing, before #275", pace: time.Second},
	{name: "unpaced", about: "serial, no pacing"},
	{name: "parallel", about: "origin lanes, from #275", parallel: true},
}

func main() {
	workload := flag.String("workload", "traced", "workload: a preset (traced, heavy, recent, small) or a JSON file")
	printWorkload := flag.Bool("print-workload", false, "print the workload as JSON and exit")
	names := flag.String("strategies", "paced,unpaced,parallel", "comma-separated strategies to run: paced, unpaced, parallel")
	runs := flag.Int("runs", 1, "runs of each strategy")
	traceDir := flag.String("trace-dir", "", "keep the HTTP trace of every run in this directory, for tools/tracereport")
	limits := lane.Defaults
	flag.IntVar(&limits.HTTP2, "h2", limits.HTTP2, "parallel: downloads at a time from an HTTP/2 origin")
	flag.IntVar(&limits.HTTP1, "h1", limits.HTTP1, "parallel: downloads at a time from an HTTP/1.1 origin")
	flag.IntVar(&limits.Total, "total", limits.Total, "parallel: downloads at a time in all")
	flag.IntVar(&limits.Large, "large", limits.Large, "parallel: large downloads at a time from one origin")
	flag.Int64Var(&limits.LargeSize, "large-size", limits.LargeSize, "parallel: bytes from which a download of known size is large; 0 for none")
	flag.Parse()
	if err := run(os.Stdout, *workload, *printWorkload, *names, *runs, *traceDir, limits); err != nil {
		fmt.Fprintln(os.Stderr, "assetbench:", err)
		os.Exit(1)
	}
}

func run(stdout io.Writer, workloadArg string, printWorkload bool, names string, runs int, traceDir string, limits lane.Limits) error {
	w, err := loadWorkload(workloadArg)
	if err != nil {
		return err
	}
	if printWorkload {
		out, err := json.MarshalIndent(w, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s\n", out)
		return err
	}
	chosen, err := pickStrategies(names)
	if err != nil {
		return err
	}
	if runs < 1 {
		return errors.New("-runs must be at least 1")
	}
	if traceDir != "" {
		if err := os.MkdirAll(traceDir, 0o755); err != nil {
			return err
		}
	}

	b := startBench(w)
	defer b.close()
	var results []result
	for _, s := range chosen {
		for i := range runs {
			fmt.Fprintf(os.Stderr, "assetbench: %s, run %d of %d ...\n", s.name, i+1, runs)
			res, trace, err := b.run(context.Background(), s, i+1, limits)
			if err != nil {
				return err
			}
			results = append(results, res)
			if traceDir != "" {
				path := filepath.Join(traceDir, fmt.Sprintf("%s-%d.jsonl", s.name, i+1))
				if err := os.WriteFile(path, trace, 0o600); err != nil {
					return err
				}
			}
		}
	}
	writeReport(stdout, w, limits, results)
	return nil
}

func pickStrategies(names string) ([]strategy, error) {
	var chosen []strategy
	for name := range strings.SplitSeq(names, ",") {
		name = strings.TrimSpace(name)
		i := slices.IndexFunc(strategies, func(s strategy) bool { return s.name == name })
		if i < 0 {
			return nil, fmt.Errorf("unknown strategy %q", name)
		}
		chosen = append(chosen, strategies[i])
	}
	return chosen, nil
}

// bench is a workload with its origins up.
type bench struct {
	w       Workload
	origins []*fakeOrigin
}

func startBench(w Workload) *bench {
	b := &bench{w: w}
	for _, o := range w.Origins {
		b.origins = append(b.origins, startOrigin(o))
	}
	return b
}

func (b *bench) close() {
	for _, o := range b.origins {
		o.srv.Close()
	}
}

// result is one run of a strategy.
type result struct {
	strategy strategy
	run      int
	wall     time.Duration
	times    slack.TraceTimes
	requests int
	newConns int
	peak     int           // the most requests at a time
	longest  time.Duration // the longest request
	saved    int
	notSaved int // failed or over the size limit
}

// run downloads the workload with s over new connections, as run n of s, the
// parallel strategy within limits, and returns the result and the trace.
func (b *bench) run(ctx context.Context, s strategy, n int, limits lane.Limits) (result, []byte, error) {
	dir, err := os.MkdirTemp("", "assetbench-")
	if err != nil {
		return result{}, nil, err
	}
	defer os.RemoveAll(dir)
	tr := newTransport(certPool(b.origins))
	defer tr.CloseIdleConnections()
	var trace bytes.Buffer
	// No token: the client sends one only to files.slack.com.
	client := slack.New("", slack.WithTransport(tr), slack.WithTrace(&trace))
	var dl output.Downloader = client
	paced := &pacedDownloader{Downloader: client, pace: s.pace}
	if s.pace > 0 {
		dl = paced
	}
	assets := output.NewAssets(ctx, dl, dir, maxAttachment)
	assets.Lanes = limits
	// save asks a for the workload's assets, as a render of the page does.
	save := func(a *output.Assets) {
		for i, asset := range b.w.Assets {
			a.Save(asset.Kind, b.url(i, asset), asset.meta())
		}
	}

	start := time.Now()
	if s.parallel {
		planner := assets.Planner()
		save(planner)
		assets.Fetch(planner.Plan())
	}
	save(assets)
	res := result{strategy: s, run: n, wall: time.Since(start)}
	saved, skipped, failed := assets.Counts()
	res.saved, res.notSaved = saved, skipped+failed
	var spans [][2]time.Time
	for line := range bytes.Lines(trace.Bytes()) {
		var rec slack.TraceRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return result{}, nil, fmt.Errorf("trace: %w", err)
		}
		res.requests++
		if rec.GotConnUS != nil && !rec.ConnReused {
			res.newConns++
		}
		res.times = res.times.Add(rec.Times())
		if rec.DoneUS != nil {
			took := time.Duration(*rec.DoneUS) * time.Microsecond
			spans = append(spans, [2]time.Time{rec.Start, rec.Start.Add(took)})
			res.longest = max(res.longest, took)
		}
	}
	res.times.PacingWait += paced.waited
	res.peak = peak(spans)
	return res, trace.Bytes(), nil
}

// url is where the page links asset i: at its origin, or at the origin that
// redirects to it.
func (b *bench) url(i int, a Asset) string {
	u := b.origins[a.Origin].url(i, a.Bytes)
	if a.Via != nil {
		u = b.origins[*a.Via].redirect(u)
	}
	return u
}

// meta is what the page knows of the asset before it is downloaded: Slack
// gives the size of an uploaded file, and of nothing else.
func (a Asset) meta() output.AssetMeta {
	if a.Kind == output.KindUploadOriginal || a.Kind == output.KindAttachment {
		return output.AssetMeta{SizeBytes: a.Bytes}
	}
	return output.AssetMeta{}
}

// pacedDownloader starts each download pace after the start of the one before
// it, as the Slack client paced the downloads before #275, and adds up the
// waits. It serves one download at a time.
type pacedDownloader struct {
	output.Downloader
	pace time.Duration

	last   time.Time
	waited time.Duration
}

func (d *pacedDownloader) Download(ctx context.Context, srcURL string, limit int64, w io.Writer) (int64, string, error) {
	if !d.last.IsZero() {
		if wait := d.pace - time.Since(d.last); wait > 0 {
			if !sleep(ctx, wait) {
				return 0, "", ctx.Err()
			}
			d.waited += wait
		}
	}
	d.last = time.Now()
	return d.Downloader.Download(ctx, srcURL, limit, w)
}

// peak is the most spans that overlap at one time.
func peak(spans [][2]time.Time) int {
	type edge struct {
		at    time.Time
		delta int
	}
	var edges []edge
	for _, s := range spans {
		edges = append(edges, edge{s[0], 1}, edge{s[1], -1})
	}
	// An end before a start at the same time: spans that only touch do not
	// overlap.
	slices.SortFunc(edges, func(a, b edge) int { return cmp.Or(a.at.Compare(b.at), cmp.Compare(a.delta, b.delta)) })
	most, n := 0, 0
	for _, e := range edges {
		n += e.delta
		most = max(most, n)
	}
	return most
}

func writeReport(w io.Writer, wl Workload, limits lane.Limits, results []result) {
	fmt.Fprintf(w, "## Asset download benchmark\n\n")
	fmt.Fprintf(w, "%s on %s/%s with %d CPUs. Workload %q: %s.\n", runtime.Version(), runtime.GOOS, runtime.GOARCH,
		runtime.NumCPU(), wl.Name, describe(wl))
	if slices.ContainsFunc(results, func(r result) bool { return r.strategy.parallel }) {
		fmt.Fprintf(w, "Lanes: %d downloads at a time from an HTTP/2 origin, %d from an HTTP/1.1 origin, %d in all; "+
			"%d of %s or more from one origin.\n", limits.HTTP2, limits.HTTP1, limits.Total, limits.Large, size(limits.LargeSize))
	}
	fmt.Fprintln(w, "The origins are in-process models (tools/assetbench), not measurements.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Strategy | Run | Wall | Pacing wait | Retry wait | Connect | First byte | Transfer | Other | Requests | New conns | Peak | Longest | Not saved | Speedup |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	base := results[0].wall
	if i := slices.IndexFunc(results, func(r result) bool { return r.strategy.name == "paced" }); i >= 0 {
		base = results[i].wall
	}
	for _, r := range results {
		t := r.times
		fmt.Fprintf(w, "| %s (%s) | %d | %s | %s | %s | %s | %s | %s | %s | %d | %d | %d | %s | %d | %.1f× |\n",
			r.strategy.name, r.strategy.about, r.run, seconds(r.wall), seconds(t.PacingWait), seconds(t.RetryWait),
			seconds(t.Connect), seconds(t.FirstByte), seconds(t.Transfer), seconds(max(r.wall-t.Total(), 0)),
			r.requests, r.newConns, r.peak, seconds(r.longest), r.notSaved, base.Seconds()/r.wall.Seconds())
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "\"Other\" is the wall time outside the requests: writing and hashing the files, and the client's own work. "+
		"The requests of the parallel strategy overlap, so its parts add up to more than its wall time. "+
		"\"Peak\" is the most requests at a time, and \"Longest\" the longest request.")
}

// size is n bytes in KiB or MiB.
func size(n int64) string {
	if n >= mib && n%mib == 0 {
		return fmt.Sprintf("%d MiB", n/mib)
	}
	return fmt.Sprintf("%d KiB", n/kib)
}

// describe sums up the workload: its assets, its requests and origins per
// class, and the HTTP versions of the origins. A redirected asset takes a
// request at each of its two origins.
func describe(wl Workload) string {
	type group struct {
		requests int
		origins  map[int]bool
	}
	groups := map[string]*group{}
	requests := 0
	request := func(origin int) {
		class := wl.Origins[origin].Class
		if groups[class] == nil {
			groups[class] = &group{origins: map[int]bool{}}
		}
		groups[class].requests++
		groups[class].origins[origin] = true
		requests++
	}
	var total int64
	for _, a := range wl.Assets {
		if a.Via != nil {
			request(*a.Via)
		}
		request(a.Origin)
		total += a.Bytes
	}
	used := map[int]bool{}
	var parts []string
	for _, class := range slices.SortedFunc(maps.Keys(groups), func(a, b string) int {
		return cmp.Or(cmp.Compare(classRank(a), classRank(b)), strings.Compare(a, b))
	}) {
		g := groups[class]
		parts = append(parts, fmt.Sprintf("%s %d on %d", class, g.requests, len(g.origins)))
		maps.Copy(used, g.origins)
	}
	h2 := 0
	for i := range used {
		if wl.Origins[i].HTTP2 {
			h2++
		}
	}
	return fmt.Sprintf("%d assets (%.1f MB) in %d requests to %d origins (%s); HTTP/2 origins %d, HTTP/1.1 origins %d",
		len(wl.Assets), float64(total)/mib, requests, len(used), strings.Join(parts, ", "), h2, len(used)-h2)
}

// classRank orders the classes as tools/tracereport does.
func classRank(class string) int {
	i := slices.Index([]string{"files.slack.com", "Slack CDN", "gravatar", "other"}, class)
	if i < 0 {
		return 4
	}
	return i
}

func seconds(d time.Duration) string {
	return fmt.Sprintf("%.3f s", d.Seconds())
}
