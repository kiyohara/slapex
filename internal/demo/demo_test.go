package demo

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// TestScenariosRenderEndToEnd runs both bundled fixtures through the shared
// Export driver, the same path slapex --demo and gensample take. It guards
// that the fixtures stay renderable and that no {{base}} placeholder leaks
// into the output.
func TestScenariosRenderEndToEnd(t *testing.T) {
	now := time.Now()
	for _, sc := range []*Scenario{ScenarioJA(now), ScenarioEN(now)} {
		t.Run(sc.Lang, func(t *testing.T) {
			dir, err := Export(context.Background(), sc, Options{
				OutputDir:      t.TempDir(),
				MaxPosts:       1000,
				Days:           30,
				MaxAttachBytes: 10 << 20,
				ToolVersion:    "test",
			}, ui.NewPrinter(io.Discard, false))
			if err != nil {
				t.Fatalf("demo.Export: %v", err)
			}

			html, err := os.ReadFile(filepath.Join(dir, "index.html"))
			if err != nil {
				t.Fatalf("read index.html: %v", err)
			}
			if len(html) == 0 {
				t.Fatal("index.html is empty")
			}
			if bytes.Contains(html, []byte("{{base}}")) {
				t.Fatal("index.html still contains the {{base}} placeholder")
			}
			assets, err := os.ReadDir(filepath.Join(dir, "assets"))
			if err != nil {
				t.Fatalf("read assets dir: %v", err)
			}
			if len(assets) == 0 {
				t.Fatal("no assets were written")
			}
		})
	}
}

func TestExportDateRangeEndToEnd(t *testing.T) {
	now := time.Date(2026, 7, 3, 16, 0, 0, 0, time.Local)
	sc := ScenarioJA(now)
	targetDate := now.AddDate(0, 0, -1).Format("2006-01-02")
	dir, err := Export(context.Background(), sc, Options{
		OutputDir:      t.TempDir(),
		MaxPosts:       1000,
		Date:           targetDate,
		MaxAttachBytes: 10 << 20,
		ToolVersion:    "test",
		Now:            now,
	}, ui.NewPrinter(io.Discard, false))
	if err != nil {
		t.Fatalf("demo.Export(--date): %v", err)
	}
	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(html, []byte("イベントサイト、staging")) {
		t.Fatal("date export is missing a target-day message")
	}
	if bytes.Contains(html, []byte("開催決定です")) {
		t.Fatal("date export contains a previous-day timeline message")
	}
}

func TestExportDateTimeRangeEndToEnd(t *testing.T) {
	now := time.Date(2026, 7, 3, 16, 0, 0, 0, time.Local)
	sc := ScenarioJA(now)
	day := now.AddDate(0, 0, -1)
	start := time.Date(day.Year(), day.Month(), day.Day(), 11, 30, 0, 0, time.Local)
	end := time.Date(day.Year(), day.Month(), day.Day(), 11, 31, 0, 0, time.Local)
	dir, err := Export(context.Background(), sc, Options{
		OutputDir:      t.TempDir(),
		MaxPosts:       1000,
		From:           start.Format(time.RFC3339),
		To:             end.Format(time.RFC3339),
		MaxAttachBytes: 10 << 20,
		ToolVersion:    "test",
		Now:            now,
	}, ui.NewPrinter(io.Discard, false))
	if err != nil {
		t.Fatalf("demo.Export(--from/--to): %v", err)
	}
	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(html, []byte("イベントサイト、staging")) {
		t.Fatal("datetime range export is missing the message at the start boundary")
	}
	if bytes.Contains(html, []byte("v1.4.0 を staging")) {
		t.Fatal("datetime range export contains the message at the end boundary")
	}
}

// TestFilterRange guards that the fake conversations.history honours both
// boundaries and uses the same half-open interval as a real run.
func TestFilterRange(t *testing.T) {
	msgs := []slack.Message{
		{TS: "100.000000", Text: "old"},
		{TS: "200.000000", Text: "mid"},
		{TS: "300.000000", Text: "new"},
	}

	// The start is included and the end is excluded.
	got := filterRange(msgs, "200.000000", "300.000000")
	gotTexts := make([]string, len(got))
	for i, m := range got {
		gotTexts[i] = m.Text
	}
	if strings.Join(gotTexts, ",") != "mid" {
		t.Fatalf("filterRange([200,300)) kept %v, want [mid]", gotTexts)
	}

	// Empty or unparseable boundaries do not filter that side.
	if n := len(filterRange(msgs, "", "")); n != len(msgs) {
		t.Fatalf("filterRange(empty) kept %d, want %d", n, len(msgs))
	}
	if n := len(filterRange(msgs, "not-a-ts", "not-a-ts")); n != len(msgs) {
		t.Fatalf("filterRange(invalid) kept %d, want %d", n, len(msgs))
	}
}

// TestAuthorized covers the fake server's token check, including the
// AllowAnyToken relaxation used only by demo recordings.
func TestAuthorized(t *testing.T) {
	strict := &fakeServer{}
	if !strict.authorized("Bearer " + FakeToken) {
		t.Fatal("strict server should accept the exact FakeToken")
	}
	if strict.authorized("Bearer other-token") {
		t.Fatal("strict server should reject a different token")
	}
	if strict.authorized("") {
		t.Fatal("strict server should reject a missing Authorization header")
	}

	any := &fakeServer{anyBearer: true}
	if !any.authorized("Bearer anything-goes") {
		t.Fatal("anyBearer server should accept any non-empty Bearer token")
	}
	if any.authorized("Bearer ") {
		t.Fatal("anyBearer server should reject an empty Bearer value")
	}
	if any.authorized("") {
		t.Fatal("anyBearer server should reject a missing Authorization header")
	}
}

// TestReplaceBaseURL asserts every {{base}} placeholder in a fixture is
// rewritten to the serving base URL, so no asset reference is left dangling.
func TestReplaceBaseURL(t *testing.T) {
	const base = "http://127.0.0.1:9999"
	sc := ScenarioJA(time.Now())
	sc.ReplaceBaseURL(base)

	if got := sc.TeamInfo.Icon.Image68; strings.Contains(got, "{{base}}") || !strings.HasPrefix(got, base) {
		t.Fatalf("team icon URL = %q, want it rewritten to %q", got, base)
	}
	for id, u := range sc.Users {
		if strings.Contains(u.Profile.Image72, "{{base}}") {
			t.Fatalf("user %s avatar still has the placeholder: %q", id, u.Profile.Image72)
		}
	}
	for name, raw := range sc.Emoji {
		if strings.Contains(raw, "{{base}}") {
			t.Fatalf("emoji %s URL still has the placeholder: %q", name, raw)
		}
	}
	for _, m := range sc.Messages {
		for _, f := range m.Files {
			if strings.Contains(f.URLPrivateDownload, "{{base}}") {
				t.Fatalf("file %q download URL still has the placeholder", f.Name)
			}
		}
	}
}
