package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
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
		{name: "max attachment lower bound unit", args: []string{"--max-attachment-size", "1KB"}},
		{name: "max attachment lower bound bytes", args: []string{"--max-attachment-size", "1024"}},
		{name: "max attachment below range", args: []string{"--max-attachment-size", "1023"}, wantErr: true},
		{name: "max attachment invalid format", args: []string{"--max-attachment-size", "large"}, wantErr: true},
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
	got := reportMissingToken(&buf)
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
			got := reportRunError(&buf, tt.err)
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
