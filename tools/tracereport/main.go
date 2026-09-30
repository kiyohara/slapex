// Command tracereport summarizes the HTTP trace of a slapex run
// (SLAPEX_HTTP_TRACE, Issue #273) as Markdown, to paste into an Issue: per
// class of origin (the Web API, files.slack.com, the Slack CDN, gravatar and
// the rest), how many requests and bytes, where their time went (pacing
// wait, retry wait, connect, first byte, transfer) against the whole run,
// and when in the run they went on (Issue #278).
//
// It prints no host of a third party, no URL hash and no time of day, only
// how many origins there are and how the requests spread over them, so that
// the summary tells nothing about what the channel links to.
//
// Usage:
//
//	docker compose run --rm dev go run ./tools/tracereport trace.jsonl
//
// With no file it reads the trace from stdin. The run is the export from its
// start to its end, as the run line at the end of the trace gives it;
// "Outside requests" is the part of the run that no request took, such as
// choosing the channel, rendering and writing files (a channel chosen at the
// prompt adds the time the choice took). A trace without the run line, of a
// run that was killed for one, is summarized over the span of its requests
// instead: from the first request (with its pacing wait) to the end of the
// last one (with its retry wait).
//
// Requests overlap: the downloads run in parallel lanes, and the Web API
// calls of different methods side by side. Their Totals then add up to more
// than the time they took, up to more than 100% of the run, and "Outside
// requests" is less than the time no request took. The spans say when the
// requests of each class went on instead: from the start of the run, when
// the first of them started and when the last one ended. Compare those, and
// the run's time, between two runs (decision log 0069).
package main

import (
	"bufio"
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: tracereport [trace.jsonl]\n\n"+
			"Summarizes the HTTP trace of one slapex run (SLAPEX_HTTP_TRACE) as Markdown. With no file, reads stdin.\n")
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	in := io.Reader(os.Stdin)
	if flag.NArg() == 1 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, "tracereport:", err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
	}
	recs, skipped, err := readTrace(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tracereport:", err)
		os.Exit(1)
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "tracereport: skipped %d lines that are not trace records\n", skipped)
	}
	if len(recs) == 0 {
		fmt.Fprintln(os.Stderr, "tracereport: no trace records")
		os.Exit(1)
	}
	writeReport(os.Stdout, summarize(recs))
}

// readTrace reads the records of a trace, and counts the lines that are not
// records (a line cut short when the run was killed, for one).
func readTrace(r io.Reader) ([]slack.TraceRecord, int, error) {
	var recs []slack.TraceRecord
	skipped := 0
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec slack.TraceRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil || rec.Start.IsZero() {
			skipped++
			continue
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, 0, err
	}
	return recs, skipped, nil
}

// The classes of origin, in report order.
const (
	classAPI      = "Web API"
	classFiles    = "files.slack.com"
	classCDN      = "Slack CDN"
	classGravatar = "gravatar"
	classOther    = "Other"
)

var classOrder = []string{classAPI, classFiles, classCDN, classGravatar, classOther}

// classOf sorts a request into a class: Web API calls by their type, the
// downloads by the host they went to (each redirect hop by its own).
func classOf(rec slack.TraceRecord) string {
	if rec.Type == slack.TraceAPI {
		return classAPI
	}
	host := strings.ToLower((&url.URL{Host: rec.Host}).Hostname())
	under := func(domain string) bool { return host == domain || strings.HasSuffix(host, "."+domain) }
	switch {
	case host == "files.slack.com":
		return classFiles
	case under("slack-edge.com"), under("slack-imgs.com"):
		return classCDN
	case under("gravatar.com"):
		return classGravatar
	default:
		return classOther
	}
}

// stats adds up a group of requests.
type stats struct {
	requests  int
	calls     int // first attempts, which start a call or a download
	retries   int // later attempts
	redirects int // redirect hops
	newConns  int
	bytes     int64
	times     slack.TraceTimes
	// origins counts the requests per origin, and protos the origins per
	// HTTP version.
	origins map[string]int
	protos  map[string]map[string]bool
}

func (s *stats) add(rec slack.TraceRecord) {
	s.requests++
	switch {
	case rec.Redirect > 0:
		s.redirects++
	case rec.Attempt > 0:
		s.retries++
	default:
		s.calls++
	}
	if rec.GotConnUS != nil && !rec.ConnReused {
		s.newConns++
	}
	s.bytes += rec.Bytes
	s.times = s.times.Add(rec.Times())
	origin := rec.Scheme + "://" + rec.Host
	if s.origins == nil {
		s.origins, s.protos = map[string]int{}, map[string]map[string]bool{}
	}
	s.origins[origin]++
	if rec.Proto != "" {
		if s.protos[rec.Proto] == nil {
			s.protos[rec.Proto] = map[string]bool{}
		}
		s.protos[rec.Proto][origin] = true
	}
}

// interval is when a group of requests went on: from the start of the first
// to the end of the last.
type interval struct {
	start, end time.Time
}

func (iv *interval) add(start, end time.Time) {
	if iv.start.IsZero() || start.Before(iv.start) {
		iv.start = start
	}
	if end.After(iv.end) {
		iv.end = end
	}
}

type summary struct {
	// span is the run: from the run line, or, in a trace without one (run is
	// false), from the first request to the end of the last one. It starts
	// at begin.
	span     time.Duration
	begin    time.Time
	run      bool
	all      stats
	classes  map[string]*stats
	methods  map[string]*stats // Web API requests by method
	kinds    map[string]*stats // downloads by asset kind
	statuses map[int]int
	errors   map[string]int

	// intervals are when the requests of each class went on, and downloads
	// and requests when those of all the downloads and all the requests did.
	intervals map[string]*interval
	downloads interval
	requests  interval
}

func summarize(recs []slack.TraceRecord) summary {
	s := summary{
		classes:   map[string]*stats{},
		intervals: map[string]*interval{},
		methods:   map[string]*stats{},
		kinds:     map[string]*stats{},
		statuses:  map[int]int{},
		errors:    map[string]int{},
	}
	var first, last time.Time
	seen := false
	extend := func(begin, end time.Time) {
		if !seen || begin.Before(first) {
			first = begin
		}
		if !seen || end.After(last) {
			last = end
		}
		seen = true
	}
	for _, rec := range recs {
		if rec.Type == slack.TraceRun {
			s.run = true
			extend(rec.Start, rec.Start.Add(lastOffset(rec)))
			continue
		}
		t := rec.Times()
		extend(rec.Start.Add(-t.PacingWait), rec.Start.Add(lastOffset(rec)+t.RetryWait))
		s.all.add(rec)
		class := classOf(rec)
		addTo(s.classes, class, rec)
		end := rec.Start.Add(lastOffset(rec))
		if s.intervals[class] == nil {
			s.intervals[class] = &interval{}
		}
		s.intervals[class].add(rec.Start, end)
		s.requests.add(rec.Start, end)
		if rec.Type == slack.TraceAPI {
			addTo(s.methods, rec.Method, rec)
		} else {
			addTo(s.kinds, cmp.Or(rec.Kind, "(none)"), rec)
			s.downloads.add(rec.Start, end)
		}
		if rec.Status != 0 {
			s.statuses[rec.Status]++
		}
		if rec.Error != "" {
			s.errors[rec.Error]++
		}
	}
	s.span = last.Sub(first)
	s.begin = first
	return s
}

func addTo(groups map[string]*stats, key string, rec slack.TraceRecord) {
	if groups[key] == nil {
		groups[key] = &stats{}
	}
	groups[key].add(rec)
}

// lastOffset is the latest phase the record has: its end, or how far it got.
func lastOffset(rec slack.TraceRecord) time.Duration {
	var last int64
	for _, us := range []*int64{rec.DNSStartUS, rec.DNSDoneUS, rec.ConnectStartUS, rec.ConnectDoneUS,
		rec.TLSStartUS, rec.TLSDoneUS, rec.GotConnUS, rec.FirstByteUS, rec.DoneUS} {
		if us != nil {
			last = max(last, *us)
		}
	}
	return time.Duration(last) * time.Microsecond
}

func writeReport(w io.Writer, s summary) {
	api := s.classes[classAPI]
	if api == nil {
		api = &stats{}
	}
	downloadOrigins := 0
	for name, st := range s.classes {
		if name != classAPI {
			downloadOrigins += len(st.origins)
		}
	}
	fmt.Fprintf(w, "## HTTP trace\n\n")
	over := "in a run of " + seconds(s.span)
	if !s.run {
		over = "over " + seconds(s.span)
	}
	fmt.Fprintf(w, "%s %s: %s, and %s from %s.\n\n", plural(s.all.requests, "request"), over,
		plural(api.calls, "Web API call"), plural(s.all.calls-api.calls, "download"), plural(downloadOrigins, "origin"))
	if !s.run {
		fmt.Fprint(w, "The trace has no run line, so the shares are of the time from the first request to the end of the last one, without the work before and after them.\n\n")
	}

	fmt.Fprintln(w, "| Class | Requests | Retries | Redirects | New conns | Bytes | Pacing wait | Retry wait | Connect | First byte | Transfer | Total | Share |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	row := func(name string, st *stats) {
		t := st.times
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			name, st.requests, st.retries, st.redirects, st.newConns, size(st.bytes),
			seconds(t.PacingWait), seconds(t.RetryWait), seconds(t.Connect), seconds(t.FirstByte), seconds(t.Transfer),
			seconds(t.Total()), share(t.Total(), s.span))
	}
	for _, name := range classOrder {
		if st := s.classes[name]; st != nil {
			row(name, st)
		}
	}
	row("All", &s.all)
	outside := max(s.span-s.all.times.Total(), 0)
	fmt.Fprintf(w, "| Outside requests | | | | | | | | | | | %s | %s |\n\n", seconds(outside), share(outside, s.span))

	from := "the start of the run"
	if !s.run {
		from = "the start of the first request (with its pacing wait)"
	}
	fmt.Fprintf(w, "When the requests of each class went on, counted from %s. Requests that overlap, "+
		"as the downloads and the Web API calls of different methods do, take less of the run than their Totals add up to.\n\n", from)
	fmt.Fprintln(w, "| Class | First request starts | Last request ends | Span |")
	fmt.Fprintln(w, "|---|---:|---:|---:|")
	spanRow := func(name string, iv interval) {
		fmt.Fprintf(w, "| %s | %s | %s | %s |\n", name,
			seconds(iv.start.Sub(s.begin)), seconds(iv.end.Sub(s.begin)), seconds(iv.end.Sub(iv.start)))
	}
	for _, name := range classOrder {
		if iv := s.intervals[name]; iv != nil {
			spanRow(name, *iv)
		}
	}
	if !s.downloads.start.IsZero() {
		spanRow("All downloads", s.downloads)
	}
	spanRow("All", s.requests)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "In \"Requests per origin\", `n ×k` is k origins with n requests each.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Class | Origins | Requests per origin | HTTP versions (origins) |")
	fmt.Fprintln(w, "|---|---:|---|---|")
	for _, name := range classOrder {
		if st := s.classes[name]; st != nil {
			fmt.Fprintf(w, "| %s | %d | %s | %s |\n", name, len(st.origins), spread(st.origins), protos(st.protos))
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Statuses: %s. Failed requests: %s.\n", counts(s.statuses), counts(s.errors))

	writeGroups(w, "Web API by method", "Method", "Calls", s.methods, s.span)
	writeGroups(w, "Downloads by asset kind", "Kind", "Downloads", s.kinds, s.span)
}

// writeGroups writes the time of each group, the longest first.
func writeGroups(w io.Writer, title, label, calls string, groups map[string]*stats, span time.Duration) {
	if len(groups) == 0 {
		return
	}
	fmt.Fprintf(w, "\n### %s\n\n", title)
	fmt.Fprintf(w, "| %s | %s | Requests | Bytes | Pacing wait | Retry wait | Connect | First byte | Transfer | Total | Share |\n", label, calls)
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	keys := slices.SortedFunc(maps.Keys(groups), func(a, b string) int {
		return cmp.Or(cmp.Compare(groups[b].times.Total(), groups[a].times.Total()), strings.Compare(a, b))
	})
	for _, key := range keys {
		st := groups[key]
		t := st.times
		fmt.Fprintf(w, "| %s | %d | %d | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			key, st.calls, st.requests, size(st.bytes), seconds(t.PacingWait), seconds(t.RetryWait), seconds(t.Connect),
			seconds(t.FirstByte), seconds(t.Transfer), seconds(t.Total()), share(t.Total(), span))
	}
}

// spread is how the requests spread over the origins, most requests first.
func spread(origins map[string]int) string {
	perCount := map[int]int{}
	for _, n := range origins {
		perCount[n]++
	}
	var parts []string
	for _, n := range slices.Backward(slices.Sorted(maps.Keys(perCount))) {
		parts = append(parts, fmt.Sprintf("%d ×%d", n, perCount[n]))
	}
	return strings.Join(parts, ", ")
}

func protos(p map[string]map[string]bool) string {
	var parts []string
	for _, proto := range slices.Sorted(maps.Keys(p)) {
		parts = append(parts, fmt.Sprintf("%s ×%d", proto, len(p[proto])))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func counts[K int | string](m map[K]int) string {
	if len(m) == 0 {
		return "none"
	}
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(m)) {
		parts = append(parts, fmt.Sprintf("%v ×%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// plural counts things: "1 origin", "2 origins".
func plural(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

func seconds(d time.Duration) string {
	return fmt.Sprintf("%.3f s", d.Seconds())
}

func share(d, span time.Duration) string {
	if span <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", 100*d.Seconds()/span.Seconds())
}

// size prints a byte count in 1024-based units, as slapex prints sizes.
func size(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
