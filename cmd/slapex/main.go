// Command slapex exports Slack channel posts as locally browsable HTML with
// assets. CLI shape, options and exit codes follow doc/design/cli-interface.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/kiyohara/slapex/internal/datetime"
	"github.com/kiyohara/slapex/internal/demo"
	"github.com/kiyohara/slapex/internal/emoji"
	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

var version = "dev"

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
	"invalid_auth":      true,
	"not_authed":        true,
	"account_inactive":  true,
	"token_revoked":     true,
	"token_expired":     true,
	"missing_scope":     true,
	"no_permission":     true,
	"not_in_channel":    true,
	"ekm_access_denied": true,
}

const (
	helpURL       = "https://github.com/kiyohara/slapex/blob/main/doc/help/slack-app-setup.md"
	slackTokenEnv = "SLACK_TOKEN"
	// apiBaseURLEnv overrides the Slack Web API base URL. Internal use only
	// (local fixture servers for demo recordings, Issue #115; decision log
	// 0046): it is not part of the public CLI surface and stays out of --help
	// and user-facing docs. Overriding the base URL redirects the Slack token,
	// so newSlackClient applies it only when the variable is explicitly
	// non-empty (doc/guidelines/credential-scope-guidelines.md).
	apiBaseURLEnv = "SLAPEX_API_BASE_URL"
)

var errUsage = errors.New("usage error")

func main() {
	os.Exit(run())
}

type cliOptions struct {
	channel              string
	outputDir            string
	maxPosts             int
	days                 int
	date                 string
	from                 string
	to                   string
	excludeBodyEmoji     []string
	excludeReactionEmoji []string
	maxAttachBytes       int64
	keepCache            bool
	reuseCache           string
	noInteractive        bool
	noColor              bool
	demo                 bool
	showVersion          bool
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

	client := newSlackClient(token, os.Getenv)
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

	dir, err := export.Run(context.Background(), client, exportOpts, printer)
	if err != nil {
		// A phase may still be live when Run fails; clear its spinner line so
		// the error report starts on a clean line.
		printer.StopPhase()
		return reportRunError(printer, err)
	}
	fmt.Fprintln(os.Stdout, dir)
	return exitOK
}

// runDemo exports a bundled fictional fixture through the real export pipeline
// without a Slack token (Issue #113, decision log 0047). It starts an
// in-process fake Slack API server for the fixture and points the client at it
// with an internal fake token, so nothing reaches a real Slack host and the
// user needs neither a Slack App nor a token to see the output. The stdout
// contract is unchanged: on success the output directory path is printed to
// stdout. The fixture has a single channel, so selection is non-interactive.
func runDemo(opts *cliOptions, printer *ui.Printer, getenv func(string) string) int {
	sc := demoScenario(getenv)
	printer.Noticef("Running the bundled demo fixture (#%s, fictional data, no Slack token used).", sc.ChannelName)

	dir, err := demo.Export(context.Background(), sc, demo.Options{
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
		ToolVersion:          version,
	}, printer)
	if err != nil {
		printer.StopPhase()
		return reportRunError(printer, err)
	}
	fmt.Fprintln(os.Stdout, dir)
	return exitOK
}

// demoScenario picks the bundled demo fixture. It prefers the Japanese fixture
// when the environment's locale starts with "ja" and the English one otherwise,
// so the demo reads naturally for either audience without adding a public
// option.
func demoScenario(getenv func(string) string) *demo.Scenario {
	if demoPrefersJapanese(getenv) {
		return demo.ScenarioJA(time.Now())
	}
	return demo.ScenarioEN(time.Now())
}

// demoPrefersJapanese reports whether the environment's locale
// (LC_ALL / LC_MESSAGES / LANG, in POSIX precedence order) selects Japanese.
func demoPrefersJapanese(getenv func(string) string) bool {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(key); v != "" {
			return strings.HasPrefix(strings.ToLower(v), "ja")
		}
	}
	return false
}

func slackTokenFromEnv(getenv func(string) string) string {
	return getenv(slackTokenEnv)
}

// apiBaseURLFromEnv returns the Slack Web API base URL override, or "" when
// unset (or whitespace-only), in which case the client keeps its default
// https://slack.com/api/ target (internal/slack TestNewDefaults).
func apiBaseURLFromEnv(getenv func(string) string) string {
	return strings.TrimSpace(getenv(apiBaseURLEnv))
}

// newSlackClient builds the Slack client for token, honouring the internal
// apiBaseURLEnv override. The token follows the base URL, so the override is
// applied only when explicitly set; every other run targets the default
// Slack host (doc/guidelines/credential-scope-guidelines.md).
func newSlackClient(token string, getenv func(string) string) *slack.Client {
	if base := apiBaseURLFromEnv(getenv); base != "" {
		return slack.New(token, slack.WithBaseURL(base))
	}
	return slack.New(token)
}

// resolveToken returns the Slack token to use for this run. It prefers the
// environment value (envToken); when that is empty it falls back to an
// interactive prompt, but only when a controlling terminal is available
// (tty != nil) and interactive input is not disabled (--no-interactive). This
// lets evaluators paste a token without leaving it in shell history, while CI /
// pipe runs stay non-interactive and deterministic
// (doc/design/cli-interface.md, decision log 0044). It returns "" when no token
// could be obtained, so the caller reports the missing-token error.
func resolveToken(envToken string, tty *os.File, noInteractive bool, prompt func(*os.File) string) string {
	if envToken != "" {
		return envToken
	}
	if tty == nil || noInteractive {
		return ""
	}
	return prompt(tty)
}

// promptForToken reads a Slack token from the controlling terminal without
// echoing it, returning the trimmed value or "" if nothing usable was entered
// or the read failed. Guidance, prompt and input all go to tty so the value
// never touches stdout/stderr and is not echoed to the screen. The token is
// used in memory only; it is never written to files, cache, logs or HTML
// (Issue #97). Like selectChannel, the raw terminal interaction is not unit
// tested; writeTokenPrompt and resolveToken cover the observable behaviour.
func promptForToken(tty *os.File) string {
	writeTokenPrompt(tty)
	b, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty) // ReadPassword consumes the newline without echoing it.
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// writeTokenPrompt writes the interactive token-entry guidance to w. It states
// that the value is used for this run only and is not stored, and points to
// secret managers / CI secrets for repeated use (Issue #97,
// doc/help/token-injection.md).
func writeTokenPrompt(w io.Writer) {
	fmt.Fprintf(w, "%s is not set.\n", slackTokenEnv)
	fmt.Fprintln(w, "Paste a Slack OAuth token to use for this run only.")
	fmt.Fprintln(w, "It is kept in memory only: not echoed, and not written to files, cache, logs or HTML.")
	fmt.Fprintln(w, "For repeated use, provide it from a secret manager (e.g. 1Password CLI) or CI secrets.")
	fmt.Fprintf(w, "Enter %s (input hidden): ", slackTokenEnv)
}

// openControllingTerminal returns the process's controlling terminal for
// interactive prompts, or nil when it is unavailable (no controlling terminal,
// e.g. CI or a bare pipe).
//
// Interactive selection targets /dev/tty rather than the stdio streams so it
// keeps working when stdout/stderr are redirected or wrapped. In particular,
// 1Password's `op run` enables secret masking by default, which turns BOTH
// stdout and stderr into pipes; /dev/tty still refers to the real terminal, so
// channel selection works under `op run` without --no-masking. slapex targets
// only macOS and Linux (see .goreleaser.yaml), where /dev/tty is available.
func openControllingTerminal() *os.File {
	return openTerminal("/dev/tty")
}

// openTerminal opens path for read/write and returns it only when it is a
// terminal; otherwise it returns nil, closing the file if it was opened. It is
// split from openControllingTerminal so the non-terminal and open-failure
// branches can be unit tested without a real controlling terminal.
func openTerminal(path string) *os.File {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil
	}
	if !term.IsTerminal(int(f.Fd())) {
		f.Close()
		return nil
	}
	return f
}

func reportMissingToken(p *ui.Printer) int {
	p.Errorf("%s is not set.", slackTokenEnv)
	p.Plainf("")
	p.Plainf("Set %s from your secret manager or CI secrets, then run slapex again.", slackTokenEnv)
	p.Plainf("")
	p.Plainf("Need to create a Slack App or issue a Slack token?")
	p.Plainf("See: " + helpURL)
	return exitAuth
}

// reportRunError prints the user-facing message for a failed export.Run via p
// and returns the process exit code (doc/design/cli-interface.md). Auth /
// permission failures (exit 3) also point the user at the setup help page
// (doc/design/usage-flow.md「情報が足りない場合の案内」).
func reportRunError(p *ui.Printer, err error) int {
	code := classify(err)
	p.Errorf("slapex: %s", err)
	if code == exitAuth {
		p.Plainf("See: " + helpURL)
	}
	return code
}

func parseCLIArgs(args []string, diagnostics io.Writer) (*cliOptions, error) {
	fs := flag.NewFlagSet("slapex", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	var (
		outputDir            = fs.String("output", "", "output root directory (default: ./slapex-<yyyymmdd>-<hhmm>)")
		maxPosts             = fs.Int("max-posts", 1000, "maximum number of timeline parent messages (1-10000)")
		days                 = fs.Int("days", 30, "fetch messages newer than this many days (1-90)")
		date                 = fs.String("date", "", "fetch timeline messages on the local date containing this date/time")
		from                 = fs.String("from", "", "fetch timeline messages at or after this date/time (requires --to)")
		to                   = fs.String("to", "", "fetch timeline messages before this date/time (requires --from)")
		excludeBodyEmoji     = fs.String("exclude-body-emoji", "", "exclude messages containing any comma-separated emoji shortcode")
		excludeReactionEmoji = fs.String("exclude-reaction-emoji", "", "exclude messages with any comma-separated emoji reaction")
		maxAttach            = fs.String("max-attachment-size", "10MB", "per-file save limit for attachments and original images (e.g. 10MB, 512KB, 10485760)")
		keepCache            = fs.Bool("keep-cache", false, "keep the .cache/ directory regardless of the result")
		reuseCache           = fs.String("reuse-cache", "", "reuse a previously kept cache (path to output directory or .cache/)")
		noInteractive        = fs.Bool("no-interactive", false, "never prompt interactively (channel selection or SLACK_TOKEN entry)")
		noColor              = fs.Bool("no-color", false, "plain progress output: no colors, icons or animations (also via NO_COLOR, CI, TERM=dumb)")
		demoMode             = fs.Bool("demo", false, "export a bundled fictional sample without a Slack token or Slack App")
		showVersion          = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(diagnostics, "Usage: slapex [channel] [options]\n\n")
		fmt.Fprintf(diagnostics, "Exports Slack channel posts as locally browsable HTML with assets.\n")
		fmt.Fprintf(diagnostics, "The Slack OAuth token is taken from the %s environment variable.\n", slackTokenEnv)
		fmt.Fprintf(diagnostics, "To try it first without a Slack App or token, run: slapex --demo\n\n")
		fmt.Fprintf(diagnostics, "Options:\n")
		fs.PrintDefaults()
	}
	// The standard flag package stops parsing at the first non-flag
	// argument, but the spec allows options after the positional channel
	// (cli-interface.md: slapex [channel] [options]). Parse in two passes.
	if err := fs.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			err = fmt.Errorf("%w: %v", errUsage, err)
		}
		return nil, err
	}
	channel := ""
	if fs.NArg() > 0 {
		channel = fs.Arg(0)
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			if !errors.Is(err, flag.ErrHelp) {
				err = fmt.Errorf("%w: %v", errUsage, err)
			}
			return nil, err
		}
	}
	if *showVersion {
		return &cliOptions{showVersion: true}, nil
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(diagnostics, "slapex: too many arguments: %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return nil, errUsage
	}
	if *maxPosts < 1 || *maxPosts > 10000 {
		fmt.Fprintln(diagnostics, "slapex: --max-posts must be between 1 and 10000")
		return nil, errUsage
	}
	daysExplicit := false
	dateExplicit := false
	fromExplicit := false
	toExplicit := false
	excludeBodyEmojiExplicit := false
	excludeReactionEmojiExplicit := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "days":
			daysExplicit = true
		case "date":
			dateExplicit = true
		case "from":
			fromExplicit = true
		case "to":
			toExplicit = true
		case "exclude-body-emoji":
			excludeBodyEmojiExplicit = true
		case "exclude-reaction-emoji":
			excludeReactionEmojiExplicit = true
		}
	})
	if fromExplicit || toExplicit {
		if !fromExplicit || !toExplicit {
			fmt.Fprintln(diagnostics, "slapex: --from and --to must be used together")
			return nil, errUsage
		}
		if dateExplicit {
			fmt.Fprintln(diagnostics, "slapex: --from/--to and --date cannot be used together")
			return nil, errUsage
		}
		if daysExplicit {
			fmt.Fprintln(diagnostics, "slapex: --from/--to and --days cannot be used together")
			return nil, errUsage
		}
		fromTime, err := datetime.Parse(*from, time.Local)
		if err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --from %q (unsupported date/time format)\n", *from)
			return nil, errUsage
		}
		toTime, err := datetime.Parse(*to, time.Local)
		if err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --to %q (unsupported date/time format)\n", *to)
			return nil, errUsage
		}
		if !fromTime.Before(toTime) {
			fmt.Fprintln(diagnostics, "slapex: --from must be before --to")
			return nil, errUsage
		}
		*days = 0
	} else if dateExplicit {
		if _, err := datetime.Parse(*date, time.Local); err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --date %q (unsupported date/time format)\n", *date)
			return nil, errUsage
		}
		if daysExplicit {
			fmt.Fprintln(diagnostics, "slapex: --date and --days cannot be used together")
			return nil, errUsage
		}
		*days = 0
	}
	if !dateExplicit && !fromExplicit && !toExplicit && (*days < 1 || *days > 90) {
		fmt.Fprintln(diagnostics, "slapex: --days must be between 1 and 90")
		return nil, errUsage
	}
	maxAttachBytes, err := parseSize(*maxAttach)
	if err != nil || maxAttachBytes < 1024 {
		fmt.Fprintf(diagnostics, "slapex: invalid --max-attachment-size %q (expected e.g. 10MB, 512KB, or a byte count >= 1KB)\n", *maxAttach)
		return nil, errUsage
	}
	var excludedBodyEmoji []string
	if excludeBodyEmojiExplicit {
		excludedBodyEmoji, err = emoji.ParseList(*excludeBodyEmoji)
		if err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --exclude-body-emoji %q: %v\n", *excludeBodyEmoji, err)
			return nil, errUsage
		}
	}
	var excludedReactionEmoji []string
	if excludeReactionEmojiExplicit {
		excludedReactionEmoji, err = emoji.ParseList(*excludeReactionEmoji)
		if err != nil {
			fmt.Fprintf(diagnostics, "slapex: invalid --exclude-reaction-emoji %q: %v\n", *excludeReactionEmoji, err)
			return nil, errUsage
		}
	}
	return &cliOptions{
		channel:              channel,
		outputDir:            *outputDir,
		maxPosts:             *maxPosts,
		days:                 *days,
		date:                 *date,
		from:                 *from,
		to:                   *to,
		excludeBodyEmoji:     excludedBodyEmoji,
		excludeReactionEmoji: excludedReactionEmoji,
		maxAttachBytes:       maxAttachBytes,
		keepCache:            *keepCache,
		reuseCache:           *reuseCache,
		noInteractive:        *noInteractive,
		noColor:              *noColor,
		demo:                 *demoMode,
		showVersion:          *showVersion,
	}, nil
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
