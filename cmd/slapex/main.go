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

	"golang.org/x/term"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
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
)

var errUsage = errors.New("usage error")

func main() {
	os.Exit(run())
}

type cliOptions struct {
	channel        string
	outputDir      string
	maxPosts       int
	days           int
	maxAttachBytes int64
	keepCache      bool
	reuseCache     string
	noInteractive  bool
	showVersion    bool
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
		return reportMissingToken(os.Stderr)
	}

	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
	client := slack.New(token)
	client.Logf = logf

	exportOpts := export.Options{
		ChannelKeyword: opts.channel,
		OutputDir:      opts.outputDir,
		MaxPosts:       opts.maxPosts,
		Days:           opts.days,
		MaxAttachBytes: opts.maxAttachBytes,
		KeepCache:      opts.keepCache,
		ReuseCache:     opts.reuseCache,
		NoInteractive:  opts.noInteractive,
		PromptTTY:      promptTTY,
		ToolVersion:    version,
	}

	dir, err := export.Run(context.Background(), client, exportOpts, logf)
	if err != nil {
		return reportRunError(os.Stderr, err)
	}
	fmt.Fprintln(os.Stdout, dir)
	return exitOK
}

func slackTokenFromEnv(getenv func(string) string) string {
	return getenv(slackTokenEnv)
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

func reportMissingToken(w io.Writer) int {
	fmt.Fprintf(w, "%s is not set.\n", slackTokenEnv)
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Set %s from your secret manager or CI secrets, then run slapex again.\n", slackTokenEnv)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Need to create a Slack App or issue a Slack token?")
	fmt.Fprintln(w, "See: "+helpURL)
	return exitAuth
}

// reportRunError writes the user-facing message for a failed export.Run to w
// and returns the process exit code (doc/design/cli-interface.md). Auth /
// permission failures (exit 3) also point the user at the setup help page
// (doc/design/usage-flow.md「情報が足りない場合の案内」).
func reportRunError(w io.Writer, err error) int {
	code := classify(err)
	fmt.Fprintf(w, "slapex: %s\n", err)
	if code == exitAuth {
		fmt.Fprintln(w, "See: "+helpURL)
	}
	return code
}

func parseCLIArgs(args []string, diagnostics io.Writer) (*cliOptions, error) {
	fs := flag.NewFlagSet("slapex", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	var (
		outputDir     = fs.String("output", "", "output root directory (default: ./slapex-<yyyymmdd>-<hhmm>)")
		maxPosts      = fs.Int("max-posts", 1000, "maximum number of timeline parent messages (1-10000)")
		days          = fs.Int("days", 30, "fetch messages newer than this many days (1-90)")
		maxAttach     = fs.String("max-attachment-size", "10MB", "per-file save limit for attachments and original images (e.g. 10MB, 512KB, 10485760)")
		keepCache     = fs.Bool("keep-cache", false, "keep the .cache/ directory regardless of the result")
		reuseCache    = fs.String("reuse-cache", "", "reuse a previously kept .cache/ directory (path to .cache/)")
		noInteractive = fs.Bool("no-interactive", false, "never prompt interactively (channel selection or SLACK_TOKEN entry)")
		showVersion   = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(diagnostics, "Usage: slapex [channel] [options]\n\n")
		fmt.Fprintf(diagnostics, "Exports Slack channel posts as locally browsable HTML with assets.\n")
		fmt.Fprintf(diagnostics, "The Slack OAuth token is taken from the %s environment variable.\n\n", slackTokenEnv)
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
	if *days < 1 || *days > 90 {
		fmt.Fprintln(diagnostics, "slapex: --days must be between 1 and 90")
		return nil, errUsage
	}
	maxAttachBytes, err := parseSize(*maxAttach)
	if err != nil || maxAttachBytes < 1024 {
		fmt.Fprintf(diagnostics, "slapex: invalid --max-attachment-size %q (expected e.g. 10MB, 512KB, or a byte count >= 1KB)\n", *maxAttach)
		return nil, errUsage
	}
	return &cliOptions{
		channel:        channel,
		outputDir:      *outputDir,
		maxPosts:       *maxPosts,
		days:           *days,
		maxAttachBytes: maxAttachBytes,
		keepCache:      *keepCache,
		reuseCache:     *reuseCache,
		noInteractive:  *noInteractive,
		showVersion:    *showVersion,
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
