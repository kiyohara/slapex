// Command gensample regenerates the anonymized sample exports under
// doc/samples/ (Issue #51). Each sample is built from an entirely fictional
// fixture (workspace, users, messages, assets) in internal/demo, served by an
// in-process fake Slack API server and run through the real export pipeline
// (internal/export), so the committed samples always match the current
// renderer output. No real Slack workspace or network access is involved.
//
// Sample generation derives the fixture message dates and the footer "Exported"
// timestamp from the current time, so regenerating refreshes the sample dates to
// the day it is run — the intended behaviour when the committed samples are
// refreshed at release time (Issue #137). Pass -time <RFC3339> to pin the clock
// instead, which makes a regeneration reproducible; that is used to keep the diff
// down to the real change when only the renderer or asset layout moved and the
// dates should stay put (e.g. the content-hash asset rename in Issue #135). Run
// via:
//
//	docker compose run --rm -e TZ=Asia/Tokyo dev go run ./tools/gensample
//
// With -serve it instead keeps one scenario's fake Slack API running until
// the process is stopped, so the demo GIF recording (Issue #115,
// tools/demo/) can run the real slapex binary against it via the internal
// SLAPEX_API_BASE_URL override. -asset-delay slows asset downloads so the
// progress indicator stays visible on screen. The user-facing token-free demo
// run instead uses the same fixtures in-process through `slapex --demo`
// (Issue #113).
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kiyohara/slapex/internal/demo"
	"github.com/kiyohara/slapex/internal/ui"
)

func main() {
	out := flag.String("out", "doc/samples", "directory that receives one sample export per language")
	serve := flag.Bool("serve", false, "serve one scenario's fake Slack API until stopped (for demo recordings) instead of generating samples")
	lang := flag.String("lang", "ja", "scenario to serve with -serve (ja or en)")
	listen := flag.String("listen", "127.0.0.1:8765", "listen address for -serve")
	assetDelay := flag.Duration("asset-delay", 0, "artificial delay per asset download in -serve mode (e.g. 350ms)")
	timeArg := flag.String("time", "", "pin the sample clock (message dates and footer timestamp) to this RFC3339 instant instead of the current time, e.g. 2026-07-04T16:32:41+09:00; use it to reproduce a regeneration without churning dates")
	flag.Parse()
	if *serve {
		if err := serveScenario(*lang, *listen, *assetDelay); err != nil {
			fmt.Fprintln(os.Stderr, "gensample:", err)
			os.Exit(1)
		}
		return
	}
	now, err := sampleClock(*timeArg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gensample:", err)
		os.Exit(1)
	}
	if err := run(*out, now); err != nil {
		fmt.Fprintln(os.Stderr, "gensample:", err)
		os.Exit(1)
	}
}

// sampleClock resolves the base instant sample generation runs against. An empty
// arg means the current time (the default: regenerating refreshes sample dates
// to today). A non-empty arg is an RFC3339 timestamp that pins message dates and
// the footer timestamp so a regeneration is reproducible.
func sampleClock(arg string) (time.Time, error) {
	if arg == "" {
		return time.Now(), nil
	}
	t, err := time.Parse(time.RFC3339, arg)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid -time %q (want RFC3339 like 2026-07-04T16:32:41+09:00): %w", arg, err)
	}
	return t, nil
}

// serveScenario serves the lang scenario's fake Slack API on addr until the
// process is stopped. Unlike sample generation it accepts any Bearer token
// (demo.AllowAnyToken), because demo recordings type an arbitrary fake value at
// the token prompt; no real credential is involved either way, and the server
// only listens on the given (loopback by default) address.
func serveScenario(lang, addr string, assetDelay time.Duration) error {
	sc, err := scenario(lang)
	if err != nil {
		return err
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	baseURL := "http://" + l.Addr().String()
	sc.ReplaceBaseURL(baseURL)
	handler := demo.Handler(sc, demo.AllowAnyToken(), demo.WithAssetDelay(assetDelay))
	fmt.Printf("serving the %s sample scenario (channel #%s) on %s\n", lang, sc.ChannelName, baseURL)
	fmt.Printf("run slapex against it with:\n\n  SLAPEX_API_BASE_URL=%s/api/ slapex\n\n", baseURL)
	fmt.Println("stop with Ctrl-C")
	return http.Serve(l, handler)
}

// run regenerates every language sample against the base instant now (the fixture
// message dates and the footer timestamp derive from it).
func run(out string, now time.Time) error {
	for _, sc := range []*demo.Scenario{demo.ScenarioJA(now), demo.ScenarioEN(now)} {
		if err := buildSample(sc, now, out); err != nil {
			return fmt.Errorf("%s: %w", sc.Lang, err)
		}
	}
	return nil
}

// scenario returns the demo fixture for lang.
func scenario(lang string) (*demo.Scenario, error) {
	switch lang {
	case "ja":
		return demo.ScenarioJA(time.Now()), nil
	case "en":
		return demo.ScenarioEN(time.Now()), nil
	default:
		return nil, fmt.Errorf("unknown -lang %q (expected ja or en)", lang)
	}
}

// buildSample runs the shared demo export driver against sc and replaces
// out/<lang>/ with the generated index.html + style.css + assets/. It goes
// through demo.Export so sample generation and `slapex --demo` share the exact
// same fixture-serving wiring. now is the export clock (footer timestamp), kept
// equal to the fixture clock so the footer and message dates agree.
func buildSample(sc *demo.Scenario, now time.Time, out string) error {
	tmp, err := os.MkdirTemp("", "gensample-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	printer := ui.NewPrinter(os.Stderr, false)
	dir, err := demo.Export(context.Background(), sc, demo.Options{
		OutputDir:      tmp,
		MaxPosts:       1000,
		Days:           30,
		MaxAttachBytes: 10 << 20,
		ToolVersion:    "dev",
		Now:            now,
	}, printer)
	if err != nil {
		return err
	}

	dst := filepath.Join(out, sc.Lang)
	if err := replaceDir(dir, dst); err != nil {
		return err
	}
	fmt.Printf("%s: wrote %s\n", sc.Lang, dst)
	return nil
}

// replaceDir replaces dst with a copy of the src tree.
func replaceDir(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
