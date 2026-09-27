package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// captureStdio runs fn with os.Stdout / os.Stderr redirected to pipes and
// returns what fn wrote to each. It guards the stream contract: stdout is
// reserved for the machine-readable result (doc/design/cli-interface.md).
func captureStdio(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	fn()

	outW.Close()
	errW.Close()
	outB, _ := io.ReadAll(outR)
	errB, _ := io.ReadAll(errR)
	return string(outB), string(errB)
}

func TestRunStdoutCarriesOnlyTheResult(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// --version: stdout carries exactly the version line, nothing else.
	os.Args = []string{"slapex", "--version"}
	var code int
	stdout, _ := captureStdio(t, func() { code = run() })
	if code != exitOK {
		t.Fatalf("run(--version) = %d, want %d", code, exitOK)
	}
	if stdout != "slapex "+version+"\n" {
		t.Fatalf("stdout = %q, want the version line only", stdout)
	}

	// Usage error: all diagnostics go to stderr, stdout stays empty.
	os.Args = []string{"slapex", "--days", "0"}
	stdout, stderr := captureStdio(t, func() { code = run() })
	if code != exitUsage {
		t.Fatalf("run(--days 0) = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty on usage error", stdout)
	}
	if !strings.Contains(stderr, "--days must be between") {
		t.Fatalf("stderr = %q, missing usage diagnostics", stderr)
	}
}

func TestRunDateUsageErrors(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	for _, args := range [][]string{
		{"slapex", "--date", "2026-02-30"},
		{"slapex", "--date", "2026-07-03T25:00:00"},
		{"slapex", "--date", "2026-07-03T09:00:00JST"},
		{"slapex", "--date", "yesterday"},
		{"slapex", "--date", "2026-07-03", "--days", "7"},
		{"slapex", "--from", "2026-07-03"},
		{"slapex", "--to", "2026-07-04"},
		{"slapex", "--from", "2026-07-03", "--to", "2026-07-03"},
		{"slapex", "--from", "2026-07-03", "--to", "2026-07-04", "--days", "7"},
		{"slapex", "--from", "2026-07-03", "--to", "2026-07-04", "--date", "2026-07-03"},
	} {
		os.Args = args
		var code int
		stdout, _ := captureStdio(t, func() { code = run() })
		if code != exitUsage {
			t.Fatalf("run(%v) = %d, want %d", args[1:], code, exitUsage)
		}
		if stdout != "" {
			t.Fatalf("run(%v) stdout = %q, want empty", args[1:], stdout)
		}
	}
}

// TestAPIBaseURLFromEnv is the negative half of the credential-scope tests
// for the internal SLAPEX_API_BASE_URL override (decision log 0046): when the
// variable is unset, empty or whitespace-only, no override is applied and the
// client keeps its default https://slack.com/api/ target (asserted by
// internal/slack TestNewDefaults), so the token is never redirected.
func TestAPIBaseURLFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "unset", env: map[string]string{}, want: ""},
		{name: "empty", env: map[string]string{apiBaseURLEnv: ""}, want: ""},
		{name: "whitespace only", env: map[string]string{apiBaseURLEnv: "  \t"}, want: ""},
		{
			name: "set",
			env:  map[string]string{apiBaseURLEnv: "http://127.0.0.1:8765/api/"},
			want: "http://127.0.0.1:8765/api/",
		},
		{
			name: "other variables are ignored",
			env:  map[string]string{"SLACK_TOKEN": "xoxp-user-token"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiBaseURLFromEnv(func(key string) string { return tt.env[key] })
			if got != tt.want {
				t.Fatalf("apiBaseURLFromEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNewSlackClientBaseURLOverride is the positive half of the
// credential-scope tests for SLAPEX_API_BASE_URL: with the override set, API
// requests (including the Authorization header) go to the override host.
func TestNewSlackClientBaseURLOverride(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"url":"https://demo.example.slack.com/","team":"Demo","team_id":"T1","user":"demo","user_id":"U1"}`)
	}))
	defer srv.Close()

	client := newSlackClient("xoxp-test-fake", func(key string) string {
		if key == apiBaseURLEnv {
			return srv.URL + "/api/"
		}
		return ""
	})
	if _, err := client.AuthTest(context.Background()); err != nil {
		t.Fatalf("AuthTest via override: %v", err)
	}
	if gotPath != "/api/auth.test" {
		t.Fatalf("override server got path %q, want %q", gotPath, "/api/auth.test")
	}
	if gotAuth != "Bearer xoxp-test-fake" {
		t.Fatalf("override server got Authorization %q, want the Bearer token", gotAuth)
	}
}
