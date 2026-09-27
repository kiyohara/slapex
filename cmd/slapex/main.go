// Command slapex exports Slack channel posts as locally browsable HTML with
// assets. CLI shape, options and exit codes follow doc/design/cli-interface.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

var version = "dev"

// apiBaseURLEnv overrides the Slack Web API base URL. Internal use only
// (local fixture servers for demo recordings, Issue #115; decision log
// 0046): it is not part of the public CLI surface and stays out of --help
// and user-facing docs. Overriding the base URL redirects the Slack token,
// so newSlackClient applies it only when the variable is explicitly
// non-empty (doc/guidelines/credential-scope-guidelines.md).
const apiBaseURLEnv = "SLAPEX_API_BASE_URL"

func main() {
	os.Exit(run())
}

func run() int {
	opts, err := parseCLIArgs(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if opts.showVersion {
		fmt.Fprintln(os.Stdout, "slapex "+version)
		return exitOK
	}

	// Decoration is decided from stderr itself (the stream progress goes to),
	// never from stdout, keeping the stdout path contract independent
	// (doc/design/cli-interface.md「出力制御」).
	printer := ui.NewPrinter(os.Stderr, ui.Styled(os.Stderr, os.Getenv, opts.noColor))

	// --demo exports a bundled fictional fixture and needs neither a Slack
	// token nor a controlling terminal, so it short-circuits before both
	// (doc/design/cli-interface.md, Issue #113).
	if opts.demo {
		return runDemo(opts, printer, os.Getenv)
	}

	// Open the controlling terminal once; it drives both the interactive
	// missing-token prompt below and interactive channel selection in
	// export.Run. It is nil when unavailable (CI, pipe), keeping both paths
	// non-interactive and deterministic.
	promptTTY := openControllingTerminal()
	if promptTTY != nil {
		defer promptTTY.Close()
	}

	token := resolveToken(slackTokenFromEnv(os.Getenv), promptTTY, opts.noInteractive, promptForToken)
	if token == "" {
		return reportMissingToken(printer)
	}

	httpTrace, err := openHTTPTrace(os.Getenv)
	if err != nil {
		return reportRunError(printer, err)
	}
	defer httpTrace.finish(printer)

	client := newSlackClient(token, os.Getenv, httpTrace.clientOptions()...)
	client.Logf = printer.Noticef

	exportOpts := export.Options{
		ChannelKeyword:       opts.channel,
		OutputDir:            opts.outputDir,
		MaxPosts:             opts.maxPosts,
		Days:                 opts.days,
		Date:                 opts.date,
		From:                 opts.from,
		To:                   opts.to,
		ExcludeBodyEmoji:     opts.excludeBodyEmoji,
		ExcludeReactionEmoji: opts.excludeReactionEmoji,
		MaxAttachBytes:       opts.maxAttachBytes,
		KeepCache:            opts.keepCache,
		ReuseCache:           opts.reuseCache,
		NoInteractive:        opts.noInteractive,
		PromptTTY:            promptTTY,
		ToolVersion:          version,
	}

	// The trace ends with the run line, the export's start and duration, which
	// its summary sets the requests against (tools/tracereport).
	runStart := time.Now()
	dir, err := export.Run(context.Background(), client, exportOpts, printer)
	client.TraceRun(runStart)
	if err != nil {
		// A phase may still be live when Run fails; clear its spinner line so
		// the error report starts on a clean line.
		printer.StopPhase()
		return reportRunError(printer, err)
	}
	fmt.Fprintln(os.Stdout, dir)
	return exitOK
}

// apiBaseURLFromEnv returns the Slack Web API base URL override, or "" when
// unset (or whitespace-only), in which case the client keeps its default
// https://slack.com/api/ target (internal/slack TestNewDefaults).
func apiBaseURLFromEnv(getenv func(string) string) string {
	return strings.TrimSpace(getenv(apiBaseURLEnv))
}

// newSlackClient builds the Slack client for token with opts, honouring the
// internal apiBaseURLEnv override. The token follows the base URL, so the
// override is applied only when explicitly set; every other run targets the
// default Slack host (doc/guidelines/credential-scope-guidelines.md).
func newSlackClient(token string, getenv func(string) string, opts ...slack.Option) *slack.Client {
	if base := apiBaseURLFromEnv(getenv); base != "" {
		opts = append([]slack.Option{slack.WithBaseURL(base)}, opts...)
	}
	return slack.New(token, opts...)
}
