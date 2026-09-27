// Command assetbench compares ways of downloading the assets of an export
// (Issue #273, PF-01 of #272). It serves a workload's assets from in-process
// fake origins (httptest over TLS, HTTP/2 or HTTP/1.1, each with its own
// handshake and first-byte delays, bandwidth and HTTP/2 stream limit) and
// downloads them through slapex's own client and asset store
// (internal/slack, internal/output) with each strategy:
//
//   - current: as slapex does today, one at a time, 1 s apart (the pacing).
//   - unpaced: one at a time, without the pacing (slack.WithSleeper).
//
// The parallel strategies of PF-03 (#275) are to join them. Every run traces
// its requests (slack.WithTrace), and the report splits each run's wall time
// the way tools/tracereport does. The origins are models, not measurements:
// the report compares strategies, and its absolute times are only as good as
// the workload.
//
// Usage:
//
//	docker compose run --rm dev go run ./tools/assetbench [-workload recent] [-strategies current,unpaced] [-runs 1]
//
// -workload takes a preset (recent, small) or a JSON file, and
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

	"github.com/kiyohara/slapex/internal/output"
	"github.com/kiyohara/slapex/internal/slack"
)

// maxAttachment is slapex's default --max-attachment-size.
const maxAttachment = 10 * mib

// A strategy is a way of downloading the workload.
type strategy struct {
	name  string
	about string
	// options are its Slack client options, besides the transport and the
	// trace.
	options []slack.Option
}

var strategies = []strategy{
	{name: "current", about: "serial, 1 s pacing"},
	{name: "unpaced", about: "serial, no pacing", options: []slack.Option{
		slack.WithSleeper(func(context.Context, time.Duration) error { return nil }),
	}},
}

func main() {
	workload := flag.String("workload", "recent", "workload: a preset (recent, small) or a JSON file")
	printWorkload := flag.Bool("print-workload", false, "print the workload as JSON and exit")
	names := flag.String("strategies", "current,unpaced", "comma-separated strategies to run: current, unpaced")
	runs := flag.Int("runs", 1, "runs of each strategy")
	traceDir := flag.String("trace-dir", "", "keep the HTTP trace of every run in this directory, for tools/tracereport")
	flag.Parse()
	if err := run(os.Stdout, *workload, *printWorkload, *names, *runs, *traceDir); err != nil {
		fmt.Fprintln(os.Stderr, "assetbench:", err)
		os.Exit(1)
	}
}

func run(stdout io.Writer, workloadArg string, printWorkload bool, names string, runs int, traceDir string) error {
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
			res, trace, err := b.run(context.Background(), s, i+1)
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
	writeReport(stdout, w, results)
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
	saved    int
	notSaved int // failed or over the size limit
}

// run downloads the workload with s over new connections, as run n of s, and
// returns the result and the trace.
func (b *bench) run(ctx context.Context, s strategy, n int) (result, []byte, error) {
	dir, err := os.MkdirTemp("", "assetbench-")
	if err != nil {
		return result{}, nil, err
	}
	defer os.RemoveAll(dir)
	tr := newTransport(certPool(b.origins))
	defer tr.CloseIdleConnections()
	var trace bytes.Buffer
	// No token: the client sends one only to files.slack.com.
	client := slack.New("", append([]slack.Option{slack.WithTransport(tr), slack.WithTrace(&trace)}, s.options...)...)
	assets := output.NewAssets(ctx, client, dir, maxAttachment)

	start := time.Now()
	for i, a := range b.w.Assets {
		assets.Save(a.Kind, b.origins[a.Origin].url(i, a.Bytes), output.AssetMeta{})
	}
	res := result{strategy: s, run: n, wall: time.Since(start)}
	saved, skipped, failed := assets.Counts()
	res.saved, res.notSaved = saved, skipped+failed
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
	}
	return res, trace.Bytes(), nil
}

func writeReport(w io.Writer, wl Workload, results []result) {
	fmt.Fprintf(w, "## Asset download benchmark\n\n")
	fmt.Fprintf(w, "%s on %s/%s with %d CPUs. Workload %q: %s.\n", runtime.Version(), runtime.GOOS, runtime.GOARCH,
		runtime.NumCPU(), wl.Name, describe(wl))
	fmt.Fprintln(w, "The origins are in-process models (tools/assetbench), not measurements.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Strategy | Run | Wall | Pacing wait | Retry wait | Connect | First byte | Transfer | Other | Requests | New conns | Not saved | Speedup |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	base := results[0].wall
	if i := slices.IndexFunc(results, func(r result) bool { return r.strategy.name == "current" }); i >= 0 {
		base = results[i].wall
	}
	for _, r := range results {
		t := r.times
		fmt.Fprintf(w, "| %s (%s) | %d | %s | %s | %s | %s | %s | %s | %s | %d | %d | %d | %.1f× |\n",
			r.strategy.name, r.strategy.about, r.run, seconds(r.wall), seconds(t.PacingWait), seconds(t.RetryWait),
			seconds(t.Connect), seconds(t.FirstByte), seconds(t.Transfer), seconds(max(r.wall-t.Total(), 0)),
			r.requests, r.newConns, r.notSaved, base.Seconds()/r.wall.Seconds())
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "\"Other\" is the wall time outside the requests: writing and hashing the files, and the client's own work.")
}

// describe sums up the workload: its assets and origins per class, and the
// HTTP versions of the origins.
func describe(wl Workload) string {
	type group struct {
		assets  int
		origins map[int]bool
	}
	groups := map[string]*group{}
	var total int64
	for _, a := range wl.Assets {
		class := wl.Origins[a.Origin].Class
		if groups[class] == nil {
			groups[class] = &group{origins: map[int]bool{}}
		}
		groups[class].assets++
		groups[class].origins[a.Origin] = true
		total += a.Bytes
	}
	used := map[int]bool{}
	var parts []string
	for _, class := range slices.SortedFunc(maps.Keys(groups), func(a, b string) int {
		return cmp.Or(cmp.Compare(classRank(a), classRank(b)), strings.Compare(a, b))
	}) {
		g := groups[class]
		parts = append(parts, fmt.Sprintf("%s %d on %d", class, g.assets, len(g.origins)))
		maps.Copy(used, g.origins)
	}
	h2 := 0
	for i := range used {
		if wl.Origins[i].HTTP2 {
			h2++
		}
	}
	return fmt.Sprintf("%d assets (%.1f MB) from %d origins (%s); HTTP/2 origins %d, HTTP/1.1 origins %d",
		len(wl.Assets), float64(total)/mib, len(used), strings.Join(parts, ", "), h2, len(used)-h2)
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
