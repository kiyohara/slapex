package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"testing"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
)

func parseArgs(args []string) (*cliOptions, error) {
	return parseArgsWithOutput(args, io.Discard)
}

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
			_, err := parseArgs(tt.args)
			if tt.wantErr && err == nil {
				t.Fatalf("parseArgs(%v) succeeded, want error", tt.args)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("parseArgs(%v) returned error: %v", tt.args, err)
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
			got, err := parseArgs(tt.args)
			if err != nil {
				t.Fatalf("parseArgs(%v) returned error: %v", tt.args, err)
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
			_, err := parseArgs(tt.args)
			if err == nil {
				t.Fatalf("parseArgs(%v) succeeded, want error", tt.args)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("parseArgs(%v) error = %v, want %v", tt.args, err, tt.wantErr)
			}
		})
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
