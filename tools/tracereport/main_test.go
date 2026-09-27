package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// traceLines is a small trace: two Web API calls (one retried after a 429),
// a Slack file, an avatar on the Slack CDN that redirects to gravatar, and
// three downloads from two third-party origins.
func traceLines(t *testing.T) string {
	t.Helper()
	base := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	us := func(v int64) *int64 { return &v }
	at := func(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }
	// A request that took connect 10ms, first byte 20ms more, transfer 70ms more.
	phases := func(rec slack.TraceRecord, reused bool) slack.TraceRecord {
		rec.GotConnUS, rec.FirstByteUS, rec.DoneUS = us(10_000), us(30_000), us(100_000)
		rec.ConnReused = reused
		if rec.Proto == "" {
			rec.Proto = "HTTP/2.0"
		}
		if rec.Status == 0 {
			rec.Status = 200
		}
		return rec
	}
	recs := []slack.TraceRecord{
		phases(slack.TraceRecord{Start: at(0), Type: slack.TraceAPI, Method: "auth.test", Scheme: "https", Host: "slack.com", URLHash: "00000000000000a1", Status: 429, RetryWaitUS: 1_000_000}, false),
		phases(slack.TraceRecord{Start: at(1100), Type: slack.TraceAPI, Method: "auth.test", Scheme: "https", Host: "slack.com", URLHash: "00000000000000a1", Attempt: 1}, true),
		phases(slack.TraceRecord{Start: at(2200), Type: slack.TraceAPI, Method: "conversations.history", Scheme: "https", Host: "slack.com", URLHash: "00000000000000a2", Bytes: 2048}, true),
		phases(slack.TraceRecord{Start: at(3300), Type: slack.TraceDownload, Kind: "attachment", Scheme: "https", Host: "files.slack.com", URLHash: "00000000000000b1", Bytes: 3 << 20}, false),
		phases(slack.TraceRecord{Start: at(5000), Type: slack.TraceDownload, Kind: "avatar", Scheme: "https", Host: "ca.slack-edge.com", URLHash: "00000000000000c1", PacingWaitUS: 600_000, Status: 302}, false),
		phases(slack.TraceRecord{Start: at(5100), Type: slack.TraceDownload, Kind: "avatar", Scheme: "https", Host: "secure.gravatar.com", URLHash: "00000000000000c2", Redirect: 1, Bytes: 9 << 10}, false),
		phases(slack.TraceRecord{Start: at(6200), Type: slack.TraceDownload, Kind: "og_image", Scheme: "https", Host: "news.example.org", URLHash: "00000000000000d1", PacingWaitUS: 900_000, Bytes: 100 << 10}, false),
		phases(slack.TraceRecord{Start: at(7300), Type: slack.TraceDownload, Kind: "og_image", Scheme: "https", Host: "news.example.org", URLHash: "00000000000000d2", PacingWaitUS: 900_000, Bytes: 100 << 10}, true),
		phases(slack.TraceRecord{Start: at(8400), Type: slack.TraceDownload, Kind: "service_icon", Scheme: "https", Host: "blog.example.net:8443", URLHash: "00000000000000e1", PacingWaitUS: 900_000, Proto: "HTTP/1.1", Bytes: 4 << 10}, false),
	}
	var b strings.Builder
	for _, rec := range recs {
		line, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestReport(t *testing.T) {
	recs, skipped, err := readTrace(strings.NewReader(traceLines(t) + "{\"start\":\"2026-09-27T12:0\n{}\n\n"))
	if err != nil || skipped != 2 || len(recs) != 9 {
		t.Fatalf("readTrace = %d records, %d skipped, %v; want 9 and 2 skipped", len(recs), skipped, err)
	}
	var out bytes.Buffer
	writeReport(&out, summarize(recs))
	report := out.String()

	for _, want := range []string{
		// Without the run line, 8.4s + the last request's 0.1s, from the
		// first request: 8.5s.
		"9 requests over 8.500 s: 2 Web API calls, and 5 downloads from 5 origins.",
		"The trace has no run line, so the shares are of the time from the first request to the end of the last one",
		"| Web API | 3 | 1 | 0 | 1 | 2.0 KB | 0.000 s | 1.000 s | 0.030 s | 0.060 s | 0.210 s | 1.300 s | 15.3% |",
		"| files.slack.com | 1 | 0 | 0 | 1 | 3.0 MB | 0.000 s | 0.000 s | 0.010 s | 0.020 s | 0.070 s | 0.100 s | 1.2% |",
		"| Slack CDN | 1 | 0 | 0 | 1 | 0 B | 0.600 s |",
		"| gravatar | 1 | 0 | 1 | 1 | 9.0 KB |",
		"| Other | 3 | 0 | 0 | 2 | 204.0 KB | 2.700 s |",
		"| All | 9 | 1 | 1 | 6 | 3.2 MB | 3.300 s | 1.000 s | 0.090 s | 0.180 s | 0.630 s | 5.200 s | 61.2% |",
		"| Outside requests | | | | | | | | | | | 3.300 s | 38.8% |",
		"| Other | 2 | 2 ×1, 1 ×1 | HTTP/1.1 ×1, HTTP/2.0 ×1 |",
		"Statuses: 200 ×7, 302 ×1, 429 ×1. Failed requests: none.",
		"| auth.test | 1 | 2 | 0 B | 0.000 s | 1.000 s |",
		"| og_image | 2 | 2 | 200.0 KB | 1.800 s |",
		"| avatar | 1 | 2 | 9.0 KB | 0.600 s |",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report misses %q:\n%s", want, report)
		}
	}
	// The report names no third-party host, URL hash or time of day.
	for _, secret := range []string{"example.org", "example.net", "slack-edge.com", "secure.gravatar", "0000000000", "12:00", "2026"} {
		if strings.Contains(report, secret) {
			t.Errorf("report contains %q:\n%s", secret, report)
		}
	}
}

// TestReportRun: with the run line, the shares are of the whole run, which
// includes the work before the first request and after the last one: 1 s of
// requests in a run of 10 s is 10%, and the other 9 s are outside requests.
func TestReportRun(t *testing.T) {
	start := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	us := func(v int64) *int64 { return &v }
	req, err := json.Marshal(slack.TraceRecord{Start: start.Add(200 * time.Millisecond), Type: slack.TraceAPI,
		Method: "auth.test", Scheme: "https", Host: "slack.com", URLHash: "00000000000000a1", Status: 200,
		GotConnUS: us(100_000), FirstByteUS: us(900_000), DoneUS: us(1_000_000)})
	if err != nil {
		t.Fatal(err)
	}
	// The run line as slapex writes it, last.
	run := `{"start":"2026-09-27T12:00:00Z","type":"run","done_us":10000000}`
	recs, skipped, err := readTrace(strings.NewReader(string(req) + "\n" + run + "\n"))
	if err != nil || skipped != 0 || len(recs) != 2 {
		t.Fatalf("readTrace = %d records, %d skipped, %v; want 2", len(recs), skipped, err)
	}
	var out bytes.Buffer
	writeReport(&out, summarize(recs))
	report := out.String()
	for _, want := range []string{
		"1 request in a run of 10.000 s: 1 Web API call, and 0 downloads from 0 origins.",
		"| All | 1 | 0 | 0 | 1 | 0 B | 0.000 s | 0.000 s | 0.100 s | 0.800 s | 0.100 s | 1.000 s | 10.0% |",
		"| Outside requests | | | | | | | | | | | 9.000 s | 90.0% |",
		"Statuses: 200 ×1. Failed requests: none.",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report misses %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "no run line") {
		t.Errorf("report says the trace has no run line:\n%s", report)
	}
}

func TestClassOf(t *testing.T) {
	for host, want := range map[string]string{
		"files.slack.com":        classFiles,
		"FILES.SLACK.COM":        classFiles,
		"ca.slack-edge.com":      classCDN,
		"emoji.slack-edge.com":   classCDN,
		"slack-imgs.com":         classCDN,
		"secure.gravatar.com":    classGravatar,
		"gravatar.com:443":       classGravatar,
		"notslack-edge.com":      classOther,
		"files.slack.com.evil":   classOther,
		"[2001:db8::1]:8443":     classOther,
		"news.example.org":       classOther,
		"slack.com":              classOther, // a download from the API host
		"gravatar.com.example.o": classOther,
	} {
		if got := classOf(slack.TraceRecord{Type: slack.TraceDownload, Host: host}); got != want {
			t.Errorf("classOf(%q) = %s, want %s", host, got, want)
		}
	}
	if got := classOf(slack.TraceRecord{Type: slack.TraceAPI, Host: "slack.com"}); got != classAPI {
		t.Errorf("classOf(api) = %s, want %s", got, classAPI)
	}
}
