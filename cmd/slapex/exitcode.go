// Exit codes and the report of a failed run: the exit code an error maps to,
// and its message, with the setup help page for auth problems
// (doc/design/cli-interface.md「exit code」).

package main

import (
	"errors"

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

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

const helpURL = "https://github.com/kiyohara/slapex/blob/main/doc/help/slack-app-setup.md"

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
