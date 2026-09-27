package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kiyohara/slapex/internal/ui"
)

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
