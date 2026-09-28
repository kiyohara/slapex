// The Slack token and the controlling terminal: SLACK_TOKEN, else the prompt
// on the controlling terminal (/dev/tty), which also drives interactive
// channel selection, and the report when neither gives a token
// (doc/design/cli-interface.md「環境変数」, decision log 0044).

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/kiyohara/slapex/internal/ui"
)

const slackTokenEnv = "SLACK_TOKEN"

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

func reportMissingToken(p *ui.Printer) int {
	p.Errorf("%s is not set.", slackTokenEnv)
	p.Plainf("")
	p.Plainf("Set %s from your secret manager or CI secrets, then run slapex again.", slackTokenEnv)
	p.Plainf("")
	p.Plainf("Need to create a Slack App or issue a Slack token?")
	p.Plainf("See: " + helpURL)
	return exitAuth
}
