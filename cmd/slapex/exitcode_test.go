package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

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
