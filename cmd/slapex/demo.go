// The --demo run: the bundled fictional fixture exported through the real
// pipeline without a Slack token (doc/design/cli-interface.md, decision log
// 0047).

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kiyohara/slapex/internal/demo"
	"github.com/kiyohara/slapex/internal/ui"
)

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
