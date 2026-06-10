// Command slapex exports Slack channel posts as locally browsable HTML with
// assets. CLI shape, options and exit codes follow doc/design/cli-interface.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
)

const version = "0.0.0-poc"

const (
	exitOK      = 0
	exitOther   = 1
	exitUsage   = 2
	exitAuth    = 3
	exitRuntime = 4
)

// Slack error codes that indicate authentication / permission problems
// (exit code 3, doc/design/cli-interface.md).
var authErrorCodes = map[string]bool{
	"invalid_auth":     true,
	"not_authed":       true,
	"account_inactive": true,
	"token_revoked":    true,
	"token_expired":    true,
	"missing_scope":    true,
	"no_permission":    true,
	"not_in_channel":   true,
	"ekm_access_denied": true,
}

const helpURL = "https://github.com/kiyohara/slapex/blob/main/doc/help/slack-app-setup.md"

func main() {
	os.Exit(run())
}

func run() int {
	fs := flag.NewFlagSet("slapex", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		outputDir     = fs.String("output", "", "output root directory (default: ./slapex-<yyyymmdd>-<hhmm>)")
		maxPosts      = fs.Int("max-posts", 1000, "maximum number of timeline parent messages (1-10000)")
		days          = fs.Int("days", 30, "fetch messages newer than this many days (1-90)")
		maxAttach     = fs.String("max-attachment-size", "10MB", "per-file save limit for attachments and original images (e.g. 10MB, 512KB, 10485760)")
		keepCache     = fs.Bool("keep-cache", false, "keep the .cache/ directory regardless of the result")
		reuseCache    = fs.String("reuse-cache", "", "reuse a previously kept .cache/ directory (not implemented in PoC)")
		noInteractive = fs.Bool("no-interactive", false, "never start interactive channel selection")
		showVersion   = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: slapex [channel] [options]\n\n")
		fmt.Fprintf(os.Stderr, "Exports Slack channel posts as locally browsable HTML with assets.\n")
		fmt.Fprintf(os.Stderr, "The bot token is taken from the SLACK_BOT_TOKEN environment variable.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}
	// The standard flag package stops parsing at the first non-flag
	// argument, but the spec allows options after the positional channel
	// (cli-interface.md: slapex [channel] [options]). Parse in two passes.
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	channel := ""
	if fs.NArg() > 0 {
		channel = fs.Arg(0)
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return exitOK
			}
			return exitUsage
		}
	}
	if *showVersion {
		fmt.Fprintln(os.Stdout, "slapex "+version)
		return exitOK
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "slapex: too many arguments: %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return exitUsage
	}
	if *maxPosts < 1 || *maxPosts > 10000 {
		fmt.Fprintln(os.Stderr, "slapex: --max-posts must be between 1 and 10000")
		return exitUsage
	}
	if *days < 1 || *days > 90 {
		fmt.Fprintln(os.Stderr, "slapex: --days must be between 1 and 90")
		return exitUsage
	}
	maxAttachBytes, err := parseSize(*maxAttach)
	if err != nil || maxAttachBytes < 1024 {
		fmt.Fprintf(os.Stderr, "slapex: invalid --max-attachment-size %q (expected e.g. 10MB, 512KB, or a byte count >= 1KB)\n", *maxAttach)
		return exitUsage
	}

	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "SLACK_BOT_TOKEN is not set.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Set SLACK_BOT_TOKEN from your secret manager or CI secrets, then run slapex again.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Need to create a Slack App or issue a bot token?")
		fmt.Fprintln(os.Stderr, "See: "+helpURL)
		return exitAuth
	}

	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
	client := slack.New(token)
	client.Logf = logf

	opts := export.Options{
		ChannelKeyword: channel,
		OutputDir:      *outputDir,
		MaxPosts:       *maxPosts,
		Days:           *days,
		MaxAttachBytes: maxAttachBytes,
		KeepCache:      *keepCache,
		ReuseCache:     *reuseCache,
		NoInteractive:  *noInteractive,
		Interactive: term.IsTerminal(int(os.Stdin.Fd())) &&
			term.IsTerminal(int(os.Stdout.Fd())),
		ToolVersion: version,
	}

	dir, err := export.Run(context.Background(), client, opts, logf)
	if err != nil {
		code := classify(err)
		fmt.Fprintf(os.Stderr, "slapex: %s\n", err)
		if code == exitAuth {
			fmt.Fprintln(os.Stderr, "See: "+helpURL)
		}
		return code
	}
	fmt.Fprintln(os.Stdout, dir)
	return exitOK
}

func classify(err error) int {
	var usage *export.UsageError
	if errors.As(err, &usage) {
		return exitUsage
	}
	var api *slack.APIError
	if errors.As(err, &api) {
		if authErrorCodes[api.Code] {
			return exitAuth
		}
		if api.Code == "channel_not_found" {
			return exitUsage
		}
		return exitRuntime
	}
	return exitRuntime
}

// parseSize parses --max-attachment-size values: a plain byte count or an
// integer with a binary KB / MB / GB suffix (doc/design/cli-interface.md).
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	upper := strings.ToUpper(s)
	mult := int64(1)
	switch {
	case strings.HasSuffix(upper, "KB"):
		mult, upper = 1<<10, strings.TrimSuffix(upper, "KB")
	case strings.HasSuffix(upper, "MB"):
		mult, upper = 1<<20, strings.TrimSuffix(upper, "MB")
	case strings.HasSuffix(upper, "GB"):
		mult, upper = 1<<30, strings.TrimSuffix(upper, "GB")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(upper), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return n * mult, nil
}
