package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"
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

// TestParseArgsDiagnostics fixes what parseCLIArgs writes for an invalid
// command line (doc/design/cli-interface.md): one "slapex: ..." line for the
// first problem in the order the options are checked, and the usage after an
// extra argument or a flag the flag package rejects. An explicit empty
// --date, --from, --max-attachment-size or emoji list is invalid, unlike an
// unset one (TestParseArgsDefaults).
func TestParseArgsDiagnostics(t *testing.T) {
	const usage = "Usage: slapex [channel] [options]\n"
	tests := []struct {
		name      string
		args      []string
		want      string // the diagnostics; with wantUsage, the line before the usage
		wantUsage bool
	}{
		{name: "unknown option", args: []string{"--unknown"}, want: "flag provided but not defined: -unknown\n", wantUsage: true},
		{name: "invalid flag value", args: []string{"--days", "x"}, want: "invalid value \"x\" for flag -days: parse error\n", wantUsage: true},
		{name: "too many arguments", args: []string{"general", "extra", "more"}, want: "slapex: too many arguments: extra more\n", wantUsage: true},
		{name: "max posts", args: []string{"--max-posts", "0"}, want: "slapex: --max-posts must be between 1 and 10000\n"},
		{name: "from only", args: []string{"--from", "2026-07-03"}, want: "slapex: --from and --to must be used together\n"},
		{name: "to only", args: []string{"--to", "2026-07-04"}, want: "slapex: --from and --to must be used together\n"},
		{name: "range with date", args: []string{"--from", "2026-07-03", "--to", "2026-07-04", "--date", "2026-07-03"}, want: "slapex: --from/--to and --date cannot be used together\n"},
		{name: "range with days", args: []string{"--from", "2026-07-03", "--to", "2026-07-04", "--days", "7"}, want: "slapex: --from/--to and --days cannot be used together\n"},
		{name: "invalid from", args: []string{"--from", "2026-02-30", "--to", "2026-03-01"}, want: "slapex: invalid --from \"2026-02-30\" (unsupported date/time format)\n"},
		{name: "invalid to", args: []string{"--from", "2026-07-03", "--to", "yesterday"}, want: "slapex: invalid --to \"yesterday\" (unsupported date/time format)\n"},
		{name: "reversed range", args: []string{"--from", "2026-07-04", "--to", "2026-07-03"}, want: "slapex: --from must be before --to\n"},
		{name: "invalid date", args: []string{"--date", "yesterday"}, want: "slapex: invalid --date \"yesterday\" (unsupported date/time format)\n"},
		{name: "date with days", args: []string{"--date", "2026-07-03", "--days", "7"}, want: "slapex: --date and --days cannot be used together\n"},
		{name: "days", args: []string{"--days", "91"}, want: "slapex: --days must be between 1 and 90\n"},
		{name: "max attachment size", args: []string{"--max-attachment-size", "1023"}, want: "slapex: invalid --max-attachment-size \"1023\" (expected e.g. 10MB, 512KB, or a byte count >= 1KB)\n"},
		{name: "exclude body emoji", args: []string{"--exclude-body-emoji", "shushing_face,"}, want: "slapex: invalid --exclude-body-emoji \"shushing_face,\": invalid emoji name \"\"\n"},
		{name: "exclude reaction emoji", args: []string{"--exclude-reaction-emoji", "a b"}, want: "slapex: invalid --exclude-reaction-emoji \"a b\": invalid emoji name \"a b\"\n"},

		// An explicit empty value.
		{name: "empty date", args: []string{"--date", ""}, want: "slapex: invalid --date \"\" (unsupported date/time format)\n"},
		{name: "empty range", args: []string{"--from", "", "--to", ""}, want: "slapex: invalid --from \"\" (unsupported date/time format)\n"},
		{name: "empty max attachment size", args: []string{"--max-attachment-size", ""}, want: "slapex: invalid --max-attachment-size \"\" (expected e.g. 10MB, 512KB, or a byte count >= 1KB)\n"},
		{name: "empty exclude body emoji", args: []string{"--exclude-body-emoji", ""}, want: "slapex: invalid --exclude-body-emoji \"\": invalid emoji name \"\"\n"},
		{name: "empty exclude reaction emoji", args: []string{"--exclude-reaction-emoji", ""}, want: "slapex: invalid --exclude-reaction-emoji \"\": invalid emoji name \"\"\n"},

		// The first problem in the check order is the one reported.
		{name: "extra argument before values", args: []string{"--max-posts", "0", "general", "extra"}, want: "slapex: too many arguments: extra\n", wantUsage: true},
		{name: "max posts before range", args: []string{"--max-posts", "0", "--days", "0"}, want: "slapex: --max-posts must be between 1 and 10000\n"},
		{name: "range combination before its dates", args: []string{"--from", "bad", "--to", "bad", "--days", "7"}, want: "slapex: --from/--to and --days cannot be used together\n"},
		{name: "date before days", args: []string{"--date", "yesterday", "--days", "7"}, want: "slapex: invalid --date \"yesterday\" (unsupported date/time format)\n"},
		{name: "days before size", args: []string{"--days", "0", "--max-attachment-size", "1"}, want: "slapex: --days must be between 1 and 90\n"},
		{name: "size before emoji", args: []string{"--max-attachment-size", "1", "--exclude-body-emoji", ""}, want: "slapex: invalid --max-attachment-size \"1\" (expected e.g. 10MB, 512KB, or a byte count >= 1KB)\n"},
		{name: "body emoji before reaction emoji", args: []string{"--exclude-reaction-emoji", "", "--exclude-body-emoji", ""}, want: "slapex: invalid --exclude-body-emoji \"\": invalid emoji name \"\"\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			got, err := parseCLIArgs(tt.args, &buf)
			if got != nil || !errors.Is(err, errUsage) {
				t.Fatalf("parseCLIArgs(%q) = %+v, %v; want a usage error", tt.args, got, err)
			}
			out := buf.String()
			if tt.wantUsage {
				if !strings.HasPrefix(out, tt.want+usage) {
					t.Fatalf("diagnostics = %q, want %q and the usage", out, tt.want)
				}
				return
			}
			if out != tt.want {
				t.Fatalf("diagnostics = %q, want %q", out, tt.want)
			}
		})
	}
}

// TestParseArgsDefaults fixes the options of an empty command line. An
// explicit empty --output or --reuse-cache is the same as leaving it unset.
func TestParseArgsDefaults(t *testing.T) {
	want := cliOptions{maxPosts: 1000, days: 30, maxAttachBytes: 10 << 20}
	for _, args := range [][]string{nil, {"--output", ""}, {"--reuse-cache", ""}} {
		got, err := parseCLIArgs(args, io.Discard)
		if err != nil {
			t.Fatalf("parseCLIArgs(%q) returned error: %v", args, err)
		}
		if !reflect.DeepEqual(*got, want) {
			t.Fatalf("parseCLIArgs(%q) = %+v, want %+v", args, *got, want)
		}
	}
}

// TestParseArgsVersionAndHelp fixes how --version and --help rank against the
// other checks, wherever they appear around the channel: --help wins once the
// flag package reads it, --version wins over the checks after the parse, and
// a flag the parse rejects wins over --version.
func TestParseArgsVersionAndHelp(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error // nil: the options only print the version
	}{
		{name: "version over invalid values", args: []string{"--version", "--days", "0"}},
		{name: "version after invalid values", args: []string{"--max-posts", "0", "--version"}},
		{name: "version over extra arguments", args: []string{"--version", "general", "extra"}},
		{name: "version after the channel", args: []string{"general", "--version", "--date", "yesterday"}},
		{name: "help over version", args: []string{"--version", "--help"}, wantErr: flag.ErrHelp},
		{name: "help before version", args: []string{"--help", "--version"}, wantErr: flag.ErrHelp},
		{name: "help after the channel", args: []string{"general", "--version", "--help"}, wantErr: flag.ErrHelp},
		{name: "help over invalid values", args: []string{"--days", "0", "--help"}, wantErr: flag.ErrHelp},
		{name: "rejected flag over version", args: []string{"--version", "--unknown"}, wantErr: errUsage},
		// Only the arguments up to the one after the channel are parsed, so a
		// flag after that is an extra argument.
		{name: "version after an extra argument", args: []string{"general", "extra", "--version"}, wantErr: errUsage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCLIArgs(tt.args, io.Discard)
			if tt.wantErr != nil {
				if got != nil || !errors.Is(err, tt.wantErr) {
					t.Fatalf("parseCLIArgs(%q) = %+v, %v; want %v", tt.args, got, err, tt.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(*got, cliOptions{showVersion: true}) {
				t.Fatalf("parseCLIArgs(%q) = %+v, %v; want only showVersion", tt.args, got, err)
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
