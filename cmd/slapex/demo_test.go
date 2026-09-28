package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgsDemo(t *testing.T) {
	got, err := parseCLIArgs([]string{"--demo"}, io.Discard)
	if err != nil {
		t.Fatalf("parseCLIArgs(--demo) returned error: %v", err)
	}
	if !got.demo {
		t.Fatal("demo = false, want true")
	}
	got, err = parseCLIArgs(nil, io.Discard)
	if err != nil {
		t.Fatalf("parseCLIArgs() returned error: %v", err)
	}
	if got.demo {
		t.Fatal("demo = true by default, want false")
	}
}

// The --help output must advertise the token-free demo run so evaluators can
// discover it without reading the docs (Issue #113).
func TestParseArgsHelpMentionsDemo(t *testing.T) {
	var buf bytes.Buffer
	_, err := parseCLIArgs([]string{"--help"}, &buf)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseCLIArgs(--help) error = %v, want %v", err, flag.ErrHelp)
	}
	if !strings.Contains(buf.String(), "--demo") {
		t.Fatalf("usage %q missing the --demo option", buf.String())
	}
}

func TestDemoPrefersJapanese(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "unset falls back to English", env: map[string]string{}, want: false},
		{name: "LANG ja", env: map[string]string{"LANG": "ja_JP.UTF-8"}, want: true},
		{name: "LANG en", env: map[string]string{"LANG": "en_US.UTF-8"}, want: false},
		{name: "LANG uppercase JA", env: map[string]string{"LANG": "JA"}, want: true},
		{
			name: "LC_ALL takes precedence over LANG",
			env:  map[string]string{"LC_ALL": "ja_JP.UTF-8", "LANG": "en_US.UTF-8"},
			want: true,
		},
		{
			name: "LC_MESSAGES takes precedence over LANG",
			env:  map[string]string{"LC_MESSAGES": "ja_JP.UTF-8", "LANG": "en_US.UTF-8"},
			want: true,
		},
		{
			name: "empty higher-precedence var falls through to LANG",
			env:  map[string]string{"LC_ALL": "", "LANG": "ja_JP.UTF-8"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := demoPrefersJapanese(func(key string) string { return tt.env[key] })
			if got != tt.want {
				t.Fatalf("demoPrefersJapanese(%v) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestDemoScenarioLocaleSelection(t *testing.T) {
	ja := demoScenario(func(key string) string {
		if key == "LANG" {
			return "ja_JP.UTF-8"
		}
		return ""
	})
	if ja.Lang != "ja" {
		t.Fatalf("Japanese locale selected scenario %q, want ja", ja.Lang)
	}
	en := demoScenario(func(key string) string {
		if key == "LANG" {
			return "en_US.UTF-8"
		}
		return ""
	})
	if en.Lang != "en" {
		t.Fatalf("English locale selected scenario %q, want en", en.Lang)
	}
}

// TestRunDemoStdoutIsOutputDirOnly runs the full --demo path through run() and
// guards the two contracts that matter for the new mode: it needs no Slack
// token, and stdout carries only the output directory (the token-free notice
// goes to stderr), preserving the machine-readable stdout contract
// (doc/design/cli-interface.md).
func TestRunDemoStdoutIsOutputDirOnly(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	tmp := t.TempDir()
	os.Args = []string{"slapex", "--demo", "--no-color", "--output", tmp}

	var code int
	stdout, stderr := captureStdio(t, func() { code = run() })
	if code != exitOK {
		t.Fatalf("run(--demo) = %d, want %d; stderr=%s", code, exitOK, stderr)
	}

	dir := strings.TrimSpace(stdout)
	if dir == "" || strings.Count(strings.TrimRight(stdout, "\n"), "\n") != 0 {
		t.Fatalf("stdout = %q, want exactly the output dir on one line", stdout)
	}
	if !strings.HasPrefix(dir, tmp) {
		t.Fatalf("output dir %q is not under --output %q", dir, tmp)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		t.Fatalf("index.html was not created: %v", err)
	}
	if strings.Contains(stdout, "token") {
		t.Fatalf("stdout %q leaked the demo notice; it must stay on stderr", stdout)
	}
	if !strings.Contains(stderr, "no Slack token used") {
		t.Fatalf("stderr %q missing the token-free demo notice", stderr)
	}
}

// TestRunDemoNeedsNoSlackToken: --demo exports without SLACK_TOKEN, and with
// one set it neither sends a request to the Slack API base URL
// (SLAPEX_API_BASE_URL) nor shows the token, and writes no HTTP trace
// (SLAPEX_HTTP_TRACE): the demo only talks to its in-process fixture server
// (doc/design/cli-interface.md). The channel argument is ignored.
func TestRunDemoNeedsNoSlackToken(t *testing.T) {
	const token = "xoxp-demo-must-not-send"
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unexpected request", http.StatusTeapot)
	}))
	defer srv.Close()
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	dir := t.TempDir()
	trace := filepath.Join(dir, "trace.jsonl")
	t.Setenv(apiBaseURLEnv, srv.URL+"/api/")
	t.Setenv(httpTraceEnv, trace)
	for i, env := range []string{"", token} {
		t.Setenv(slackTokenEnv, env)
		out := filepath.Join(dir, fmt.Sprintf("out%d", i))
		os.Args = []string{"slapex", "--demo", "--no-color", "--output", out, "no-such-channel"}
		var code int
		stdout, stderr := captureStdio(t, func() { code = run() })
		if code != exitOK || !strings.HasPrefix(stdout, out) {
			t.Fatalf("run(--demo) with SLACK_TOKEN %q = %d, stdout %q; want %d and the output dir\nstderr:\n%s", env, code, stdout, exitOK, stderr)
		}
		if strings.Contains(stdout+stderr, token) {
			t.Errorf("run(--demo) showed the token: stdout %q, stderr %q", stdout, stderr)
		}
	}
	if requests != 0 {
		t.Errorf("the Slack API base URL got %d requests, want none", requests)
	}
	if _, err := os.Stat(trace); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("run(--demo) left an HTTP trace (stat: %v), want none", err)
	}
}
