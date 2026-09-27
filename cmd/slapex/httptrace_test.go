package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// TestOpenHTTPTraceOff: without SLAPEX_HTTP_TRACE (unset, empty or blank)
// there is no trace, and nothing of it shows.
func TestOpenHTTPTraceOff(t *testing.T) {
	for _, value := range []string{"", "  \t"} {
		trace, err := openHTTPTrace(func(key string) string {
			if key == httpTraceEnv {
				return value
			}
			return ""
		})
		if trace != nil || err != nil {
			t.Fatalf("openHTTPTrace(%q) = %v, %v; want no trace", value, trace, err)
		}
		if opts := trace.clientOptions(); opts != nil {
			t.Errorf("clientOptions() = %d options, want none", len(opts))
		}
		var buf bytes.Buffer
		trace.finish(ui.NewPrinter(&buf, false))
		if buf.Len() != 0 {
			t.Errorf("finish printed %q without a trace", buf.String())
		}
	}
}

// TestOpenHTTPTraceCreatesFile: the file is created for the user only, or
// truncated when it exists, and holds what the client writes.
func TestOpenHTTPTraceCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	if err := os.WriteFile(path, []byte("an older trace\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	trace, err := openHTTPTrace(func(key string) string {
		if key == httpTraceEnv {
			return path
		}
		return ""
	})
	if err != nil || trace == nil {
		t.Fatalf("openHTTPTrace = %v, %v; want the trace file", trace, err)
	}
	if _, err := trace.Write([]byte("{}\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var buf bytes.Buffer
	trace.finish(ui.NewPrinter(&buf, false))
	if buf.Len() != 0 {
		t.Errorf("finish printed %q for a complete trace", buf.String())
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "{}\n" {
		t.Fatalf("trace file = %q, %v; want only the new line", data, err)
	}

	// A new file is readable by the user only.
	fresh := filepath.Join(t.TempDir(), "fresh.jsonl")
	trace, err = openHTTPTrace(func(string) string { return fresh })
	if err != nil {
		t.Fatalf("openHTTPTrace: %v", err)
	}
	trace.finish(ui.NewPrinter(&buf, false))
	info, err := os.Stat(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("trace file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestOpenHTTPTraceBadPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "trace.jsonl")
	trace, err := openHTTPTrace(func(string) string { return path })
	if trace != nil || err == nil {
		t.Fatalf("openHTTPTrace(%q) = %v, %v; want an error", path, trace, err)
	}
	if !strings.HasPrefix(err.Error(), httpTraceEnv+": ") || classify(err) != exitRuntime {
		t.Fatalf("error = %q (exit %d), want it to name %s and exit %d", err, classify(err), httpTraceEnv, exitRuntime)
	}
}

// TestHTTPTraceFinishWarnsWhenIncomplete: a failed write does not fail the
// export; finish warns that the trace is incomplete.
func TestHTTPTraceFinishWarnsWhenIncomplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	trace, err := openHTTPTrace(func(string) string { return path })
	if err != nil {
		t.Fatalf("openHTTPTrace: %v", err)
	}
	trace.f.Close() // every later write fails
	if _, err := trace.Write([]byte("{}\n")); err == nil {
		t.Fatal("Write to a closed file succeeded")
	}
	var buf bytes.Buffer
	trace.finish(ui.NewPrinter(&buf, false))
	if got := buf.String(); !strings.HasPrefix(got, "WARN: "+httpTraceEnv+": the trace is incomplete: ") {
		t.Fatalf("finish printed %q, want the incomplete-trace warning", got)
	}
}

// TestRunWritesHTTPTrace runs slapex with SLAPEX_HTTP_TRACE against a fake
// Slack API (SLAPEX_API_BASE_URL) that rejects the token: the one request is
// traced, without the token, and the exit code stays the auth error's.
func TestRunWritesHTTPTrace(t *testing.T) {
	const token = "xoxb-cli-trace-test"
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":false,"error":"invalid_auth"}`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	t.Setenv(slackTokenEnv, token)
	t.Setenv(apiBaseURLEnv, srv.URL+"/api/")
	t.Setenv(httpTraceEnv, path)
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"slapex", "--no-interactive", "--no-color", "--output", filepath.Join(dir, "out"), "project-alpha"}

	var code int
	stdout, stderr := captureStdio(t, func() { code = run() })
	if code != exitAuth || stdout != "" {
		t.Fatalf("run() = %d, stdout %q; want %d and no stdout\nstderr:\n%s", code, stdout, exitAuth, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	var rec slack.TraceRecord
	if err := json.Unmarshal(data, &rec); err != nil || strings.Count(string(data), "\n") != 1 {
		t.Fatalf("trace = %q (%v), want one JSON line", data, err)
	}
	if rec.Type != slack.TraceAPI || rec.Method != "auth.test" || rec.Status != http.StatusOK || requests != 1 {
		t.Errorf("trace line = %+v for %d requests, want the one auth.test call", rec, requests)
	}
	if strings.Contains(string(data), token) || strings.Contains(stderr, httpTraceEnv) {
		t.Errorf("trace %q or stderr %q shows what it should not", data, stderr)
	}

	// A trace file that cannot be created stops the run before any request.
	t.Setenv(httpTraceEnv, filepath.Join(dir, "missing-dir", "trace.jsonl"))
	stdout, stderr = captureStdio(t, func() { code = run() })
	if code != exitRuntime || stdout != "" || !strings.Contains(stderr, httpTraceEnv+": ") || requests != 1 {
		t.Errorf("run() with a bad trace path = %d, stdout %q, %d requests; want %d before any request\nstderr:\n%s",
			code, stdout, requests, exitRuntime, stderr)
	}
}
