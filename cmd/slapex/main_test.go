package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "bytes", input: "10485760", want: 10 * 1024 * 1024},
		{name: "kb", input: "512KB", want: 512 * 1024},
		{name: "mb", input: "10MB", want: 10 * 1024 * 1024},
		{name: "gb", input: "2GB", want: 2 * 1024 * 1024 * 1024},
		{name: "lowercase unit", input: "1mb", want: 1024 * 1024},
		{name: "space padded", input: " 1 KB ", want: 1024},
		{name: "empty", input: "", wantErr: true},
		{name: "non integer", input: "1.5MB", wantErr: true},
		{name: "unknown unit", input: "1TB", wantErr: true},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1MB", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseSize(%q) succeeded, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSize(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseArgsValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "max posts lower bound", args: []string{"--max-posts", "1"}},
		{name: "max posts upper bound", args: []string{"--max-posts", "10000"}},
		{name: "max posts below range", args: []string{"--max-posts", "0"}, wantErr: true},
		{name: "max posts above range", args: []string{"--max-posts", "10001"}, wantErr: true},
		{name: "days lower bound", args: []string{"--days", "1"}},
		{name: "days upper bound", args: []string{"--days", "90"}},
		{name: "days below range", args: []string{"--days", "0"}, wantErr: true},
		{name: "days above range", args: []string{"--days", "91"}, wantErr: true},
		{name: "date", args: []string{"--date", "2026-07-03"}},
		{name: "slash date", args: []string{"--date", "2026/07/03"}},
		{name: "date with hour", args: []string{"--date", "2026-07-03T09"}},
		{name: "date with minute", args: []string{"--date", "2026-07-03T09:30"}},
		{name: "date with offset", args: []string{"--date", "2026-07-03T09:30:15+09:00"}},
		{name: "invalid calendar date", args: []string{"--date", "2026-02-30"}, wantErr: true},
		{name: "invalid hour", args: []string{"--date", "2026-07-03T25:00:00"}, wantErr: true},
		{name: "timezone abbreviation", args: []string{"--date", "2026-07-03T09:00:00JST"}, wantErr: true},
		{name: "natural language", args: []string{"--date", "yesterday"}, wantErr: true},
		{name: "japanese date", args: []string{"--date", "2026年07月03日"}, wantErr: true},
		{name: "date with explicit days", args: []string{"--date", "2026-07-03", "--days", "7"}, wantErr: true},
		{name: "date with max posts", args: []string{"--date", "2026-07-03", "--max-posts", "10"}},
		{name: "datetime range slash dates", args: []string{"--from", "2026/07/03", "--to", "2026/07/04"}},
		{name: "datetime range hours", args: []string{"--from", "2026-07-03T09", "--to", "2026-07-03T10"}},
		{name: "datetime range minutes", args: []string{"--from", "2026-07-03T09:30", "--to", "2026-07-03T10:45"}},
		{name: "datetime range offsets", args: []string{"--from", "2026-07-03T09:30:15+09:00", "--to", "2026-07-03T10:00:00+09:00"}},
		{name: "from only", args: []string{"--from", "2026-07-03"}, wantErr: true},
		{name: "to only", args: []string{"--to", "2026-07-04"}, wantErr: true},
		{name: "range with explicit days", args: []string{"--from", "2026-07-03", "--to", "2026-07-04", "--days", "7"}, wantErr: true},
		{name: "range with date", args: []string{"--from", "2026-07-03", "--to", "2026-07-04", "--date", "2026-07-03"}, wantErr: true},
		{name: "empty range", args: []string{"--from", "2026-07-03", "--to", "2026-07-03"}, wantErr: true},
		{name: "reversed range", args: []string{"--from", "2026-07-04", "--to", "2026-07-03"}, wantErr: true},
		{name: "invalid range date", args: []string{"--from", "2026-02-30", "--to", "2026-03-01"}, wantErr: true},
		{name: "max attachment lower bound unit", args: []string{"--max-attachment-size", "1KB"}},
		{name: "max attachment lower bound bytes", args: []string{"--max-attachment-size", "1024"}},
		{name: "max attachment below range", args: []string{"--max-attachment-size", "1023"}, wantErr: true},
		{name: "max attachment invalid format", args: []string{"--max-attachment-size", "large"}, wantErr: true},
		{name: "exclude body emoji", args: []string{"--exclude-body-emoji", "shushing_face,:SPEAK_NO_EVIL:"}},
		{name: "exclude body emoji skin tone", args: []string{"--exclude-body-emoji", ":+1::skin-tone-3:"}},
		{name: "exclude body emoji empty", args: []string{"--exclude-body-emoji", ""}, wantErr: true},
		{name: "exclude body emoji empty item", args: []string{"--exclude-body-emoji", "shushing_face,,speak_no_evil"}, wantErr: true},
		{name: "exclude body emoji trailing comma", args: []string{"--exclude-body-emoji", "shushing_face,"}, wantErr: true},
		{name: "exclude reaction emoji", args: []string{"--exclude-reaction-emoji", "shushing_face,:SPEAK_NO_EVIL:"}},
		{name: "exclude reaction emoji skin tone", args: []string{"--exclude-reaction-emoji", ":+1::skin-tone-3:"}},
		{name: "exclude reaction emoji empty", args: []string{"--exclude-reaction-emoji", ""}, wantErr: true},
		{name: "exclude reaction emoji empty item", args: []string{"--exclude-reaction-emoji", "shushing_face,,speak_no_evil"}, wantErr: true},
		{name: "exclude reaction emoji trailing comma", args: []string{"--exclude-reaction-emoji", "shushing_face,"}, wantErr: true},
		{name: "both emoji filters", args: []string{"--exclude-body-emoji", "shushing_face", "--exclude-reaction-emoji", "speak_no_evil"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCLIArgs(tt.args, io.Discard)
			if tt.wantErr && err == nil {
				t.Fatalf("parseCLIArgs(%v) succeeded, want error", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("parseCLIArgs(%v) returned error: %v", tt.args, err)
			}
		})
	}
}

func TestParseArgsNormalizesReactionEmoji(t *testing.T) {
	got, err := parseCLIArgs([]string{"--exclude-reaction-emoji", " shushing_face, :SPEAK_NO_EVIL:, :+1::skin-tone-3: "}, io.Discard)
	if err != nil {
		t.Fatalf("parseCLIArgs returned error: %v", err)
	}
	if want := "shushing_face,speak_no_evil,+1"; strings.Join(got.excludeReactionEmoji, ",") != want {
		t.Fatalf("excludeReactionEmoji = %v, want %s", got.excludeReactionEmoji, want)
	}
}

func TestParseArgsDateDisablesDefaultDays(t *testing.T) {
	got, err := parseCLIArgs([]string{"--date", "2026-07-03"}, io.Discard)
	if err != nil {
		t.Fatalf("parseCLIArgs(--date) returned error: %v", err)
	}
	if got.date != "2026-07-03" || got.days != 0 {
		t.Fatalf("date/days = %q/%d, want 2026-07-03/0", got.date, got.days)
	}
}

func TestParseArgsDateTimeRangeDisablesDefaultDays(t *testing.T) {
	got, err := parseCLIArgs([]string{"--from", "2026-07-03T09", "--to", "2026-07-03T10:45"}, io.Discard)
	if err != nil {
		t.Fatalf("parseCLIArgs(--from/--to) returned error: %v", err)
	}
	if got.from != "2026-07-03T09" || got.to != "2026-07-03T10:45" || got.days != 0 {
		t.Fatalf("from/to/days = %q/%q/%d", got.from, got.to, got.days)
	}
}

func TestParseArgsTwoPass(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "option after positional", args: []string{"general", "--days", "7"}},
		{name: "option before positional", args: []string{"--days", "7", "general"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCLIArgs(tt.args, io.Discard)
			if err != nil {
				t.Fatalf("parseCLIArgs(%v) returned error: %v", tt.args, err)
			}
			if got.channel != "general" {
				t.Fatalf("channel = %q, want %q", got.channel, "general")
			}
			if got.days != 7 {
				t.Fatalf("days = %d, want 7", got.days)
			}
		})
	}
}

func TestParseArgsUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{name: "unknown option", args: []string{"--unknown"}, wantErr: errUsage},
		{name: "invalid flag value", args: []string{"--days", "x"}, wantErr: errUsage},
		{name: "too many arguments", args: []string{"general", "extra"}, wantErr: errUsage},
		{name: "help", args: []string{"--help"}, wantErr: flag.ErrHelp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCLIArgs(tt.args, io.Discard)
			if err == nil {
				t.Fatalf("parseCLIArgs(%v) succeeded, want error", tt.args)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("parseCLIArgs(%v) error = %v, want %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestParseArgsNoColor(t *testing.T) {
	got, err := parseCLIArgs([]string{"--no-color", "general"}, io.Discard)
	if err != nil {
		t.Fatalf("parseCLIArgs(--no-color) returned error: %v", err)
	}
	if !got.noColor {
		t.Fatal("noColor = false, want true")
	}
	got, err = parseCLIArgs([]string{"general"}, io.Discard)
	if err != nil {
		t.Fatalf("parseCLIArgs(general) returned error: %v", err)
	}
	if got.noColor {
		t.Fatal("noColor = true by default, want false")
	}
}

// The --help output must list --no-color (Issue #100 acceptance criteria,
// doc/design/cli-interface.md option table).
func TestParseArgsHelpMentionsNoColor(t *testing.T) {
	var buf bytes.Buffer
	_, err := parseCLIArgs([]string{"--help"}, &buf)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseCLIArgs(--help) error = %v, want %v", err, flag.ErrHelp)
	}
	if !strings.Contains(buf.String(), "no-color") {
		t.Fatalf("usage %q missing no-color option", buf.String())
	}
}

func TestParseArgsHelpMentionsExcludeReactionEmoji(t *testing.T) {
	var buf bytes.Buffer
	_, err := parseCLIArgs([]string{"--help"}, &buf)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseCLIArgs(--help) error = %v, want %v", err, flag.ErrHelp)
	}
	if !strings.Contains(buf.String(), "exclude-reaction-emoji") {
		t.Fatalf("usage %q missing exclude-reaction-emoji option", buf.String())
	}
}

func TestParseArgsHelpMentionsDateTimeRange(t *testing.T) {
	var buf bytes.Buffer
	_, err := parseCLIArgs([]string{"--help"}, &buf)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseCLIArgs(--help) error = %v, want %v", err, flag.ErrHelp)
	}
	out := buf.String()
	for _, option := range []string{"-from", "-to"} {
		if !strings.Contains(out, option) {
			t.Fatalf("usage %q missing %s option", out, option)
		}
	}
}

func TestParseArgsHelpMentionsSlackToken(t *testing.T) {
	var buf bytes.Buffer
	_, err := parseCLIArgs([]string{"--help"}, &buf)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseCLIArgs(--help) error = %v, want %v", err, flag.ErrHelp)
	}
	out := buf.String()
	if !strings.Contains(out, "SLACK_TOKEN") {
		t.Fatalf("usage %q missing SLACK_TOKEN", out)
	}
	if strings.Contains(out, "SLACK_BOT_TOKEN") {
		t.Fatalf("usage %q still mentions SLACK_BOT_TOKEN", out)
	}
}

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

func TestSlackTokenFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "SLACK_TOKEN",
			env:  map[string]string{"SLACK_TOKEN": "xoxp-user-token"},
			want: "xoxp-user-token",
		},
		{
			name: "SLACK_BOT_TOKEN is not a fallback",
			env:  map[string]string{"SLACK_BOT_TOKEN": "xoxb-bot-token"},
			want: "",
		},
		{
			name: "SLACK_TOKEN wins by being the only supported variable",
			env: map[string]string{
				"SLACK_TOKEN":     "xoxb-bot-token",
				"SLACK_BOT_TOKEN": "xoxp-old-variable",
			},
			want: "xoxb-bot-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slackTokenFromEnv(func(key string) string {
				return tt.env[key]
			})
			if got != tt.want {
				t.Fatalf("slackTokenFromEnv() = %q, want %q", got, tt.want)
			}
		})
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

func TestResolveToken(t *testing.T) {
	// A regular (non-terminal) file stands in for an available controlling
	// terminal: resolveToken only checks tty for nil and hands it to prompt,
	// so the prompt stub never touches the real terminal.
	ttyFile, err := os.CreateTemp(t.TempDir(), "tty")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer ttyFile.Close()

	tests := []struct {
		name          string
		envToken      string
		tty           *os.File
		noInteractive bool
		promptReturns string
		want          string
		wantPrompted  bool
	}{
		{
			name:     "env token wins without prompting",
			envToken: "xoxp-env",
			tty:      ttyFile,
			want:     "xoxp-env",
		},
		{
			name: "no controlling terminal yields empty without prompting",
			tty:  nil,
			want: "",
		},
		{
			name:          "no-interactive suppresses the prompt",
			tty:           ttyFile,
			noInteractive: true,
			want:          "",
		},
		{
			name:          "prompts when unset and interactive",
			tty:           ttyFile,
			promptReturns: "xoxp-typed",
			want:          "xoxp-typed",
			wantPrompted:  true,
		},
		{
			name:          "empty prompt input yields empty token",
			tty:           ttyFile,
			promptReturns: "",
			want:          "",
			wantPrompted:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompted := false
			prompt := func(f *os.File) string {
				prompted = true
				if f != tt.tty {
					t.Fatalf("prompt received tty %v, want %v", f, tt.tty)
				}
				return tt.promptReturns
			}
			got := resolveToken(tt.envToken, tt.tty, tt.noInteractive, prompt)
			if got != tt.want {
				t.Fatalf("resolveToken() = %q, want %q", got, tt.want)
			}
			if prompted != tt.wantPrompted {
				t.Fatalf("prompt called = %v, want %v", prompted, tt.wantPrompted)
			}
		})
	}
}

func TestWriteTokenPrompt(t *testing.T) {
	var buf bytes.Buffer
	writeTokenPrompt(&buf)
	out := buf.String()
	// The prompt must name the variable, promise the value is not stored, and
	// point to secret managers / CI secrets for repeated use (Issue #97).
	for _, want := range []string{
		slackTokenEnv,
		"this run only",
		"not echoed",
		"not written to files, cache, logs or HTML",
		"1Password",
		"CI secrets",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("writeTokenPrompt output %q missing %q", out, want)
		}
	}
}

func TestOpenTerminalReturnsNilForNonTerminal(t *testing.T) {
	// A path that opens successfully but is not a terminal must yield nil, so
	// interactive selection is not attempted on a pipe/regular file.
	if f := openTerminal(os.DevNull); f != nil {
		f.Close()
		t.Fatalf("openTerminal(%q) = non-nil, want nil for non-terminal", os.DevNull)
	}
	// A path that cannot be opened (no controlling terminal, e.g. CI) must
	// yield nil rather than error out.
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if f := openTerminal(missing); f != nil {
		f.Close()
		t.Fatal("openTerminal(nonexistent) = non-nil, want nil for open failure")
	}
}

func TestReportMissingToken(t *testing.T) {
	var buf bytes.Buffer
	got := reportMissingToken(ui.NewPrinter(&buf, false))
	if got != exitAuth {
		t.Fatalf("reportMissingToken code = %d, want %d", got, exitAuth)
	}
	out := buf.String()
	for _, want := range []string{"SLACK_TOKEN is not set.", "Set SLACK_TOKEN", helpURL} {
		if !strings.Contains(out, want) {
			t.Fatalf("output %q missing %q", out, want)
		}
	}
	if strings.Contains(out, "SLACK_BOT_TOKEN") {
		t.Fatalf("output %q still mentions SLACK_BOT_TOKEN", out)
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "usage error",
			err:  &export.UsageError{},
			want: exitUsage,
		},
		{
			name: "wrapped usage error",
			err:  fmt.Errorf("select channel: %w", &export.UsageError{}),
			want: exitUsage,
		},
		{
			name: "target not found",
			err:  &slack.APIError{Method: "conversations.info", Code: "channel_not_found"},
			want: exitUsage,
		},
		{
			name: "auth failure",
			err:  &slack.APIError{Method: "auth.test", Code: "invalid_auth"},
			want: exitAuth,
		},
		{
			name: "permission failure",
			err:  &slack.APIError{Method: "conversations.history", Code: "not_in_channel"},
			want: exitAuth,
		},
		{
			name: "runtime slack api failure",
			err:  &slack.APIError{Method: "conversations.history", Code: "internal_error"},
			want: exitRuntime,
		},
		{
			name: "plain runtime failure",
			err:  errors.New("unexpected internal failure"),
			want: exitRuntime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.err); got != tt.want {
				t.Fatalf("classify(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// TestReportRunError fixes the cmd-layer contract the v1-09 integration error
// scenarios feed into: each export.Run error type maps to the documented exit
// code (cli-interface.md), and auth / permission failures (exit 3) also print
// the setup help URL (usage-flow.md「情報が足りない場合の案内」). The integration
// tests in internal/export assert that export.Run produces these error types
// for the matching Slack conditions (invalid_auth / missing_scope /
// not_in_channel / retry-exhaustion / no channel match).
func TestReportRunError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    int
		wantHelpURL bool
	}{
		{
			name:        "invalid_auth -> auth + help URL",
			err:         &slack.APIError{Method: "auth.test", Code: "invalid_auth"},
			wantCode:    exitAuth,
			wantHelpURL: true,
		},
		{
			name:        "missing_scope -> auth + help URL",
			err:         &slack.APIError{Method: "conversations.history", Code: "missing_scope"},
			wantCode:    exitAuth,
			wantHelpURL: true,
		},
		{
			name:        "not_in_channel -> auth + help URL",
			err:         &slack.APIError{Method: "conversations.history", Code: "not_in_channel"},
			wantCode:    exitAuth,
			wantHelpURL: true,
		},
		{
			name:        "channel_not_found -> usage, no help URL",
			err:         &slack.APIError{Method: "conversations.history", Code: "channel_not_found"},
			wantCode:    exitUsage,
			wantHelpURL: false,
		},
		{
			name:        "no channel match -> usage, no help URL",
			err:         &export.UsageError{},
			wantCode:    exitUsage,
			wantHelpURL: false,
		},
		{
			name:        "retry limit reached -> runtime, no help URL",
			err:         fmt.Errorf("slack api conversations.history: %w", errors.New("giving up after 5 retries: rate limited (429)")),
			wantCode:    exitRuntime,
			wantHelpURL: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := reportRunError(ui.NewPrinter(&buf, false), tt.err)
			if got != tt.wantCode {
				t.Fatalf("reportRunError code = %d, want %d", got, tt.wantCode)
			}
			out := buf.String()
			if !strings.Contains(out, "slapex: ") {
				t.Fatalf("output %q missing slapex: error prefix", out)
			}
			if gotURL := strings.Contains(out, helpURL); gotURL != tt.wantHelpURL {
				t.Fatalf("output %q help URL = %v, want %v", out, gotURL, tt.wantHelpURL)
			}
		})
	}
}
