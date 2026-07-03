// Command gensample regenerates the anonymized sample exports under
// doc/samples/ (Issue #51). Each sample is built from an entirely fictional
// fixture (workspace, users, messages, assets), served by an in-process fake
// Slack API server and run through the real export pipeline
// (internal/export), so the committed samples always match the current
// renderer output. No real Slack workspace or network access is involved.
//
// Timestamps are derived from the current time, so regenerating updates the
// dates in the samples. Run via:
//
//	docker compose run --rm -e TZ=Asia/Tokyo dev go run ./tools/gensample
//
// With -serve it instead keeps one scenario's fake Slack API running until
// the process is stopped, so the demo GIF recording (Issue #115,
// tools/demo/) can run the real slapex binary against it via the internal
// SLAPEX_API_BASE_URL override. -asset-delay slows asset downloads so the
// progress indicator stays visible on screen.
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

	"github.com/kiyohara/slapex/internal/export"
	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// fakeToken authenticates against the in-process fake server only; it is not
// a real credential and never leaves the process.
const fakeToken = "xoxp-gensample-fake-token"

func main() {
	out := flag.String("out", "doc/samples", "directory that receives one sample export per language")
	serve := flag.Bool("serve", false, "serve one scenario's fake Slack API until stopped (for demo recordings) instead of generating samples")
	lang := flag.String("lang", "ja", "scenario to serve with -serve (ja or en)")
	listen := flag.String("listen", "127.0.0.1:8765", "listen address for -serve")
	assetDelay := flag.Duration("asset-delay", 0, "artificial delay per asset download in -serve mode (e.g. 350ms)")
	flag.Parse()
	if *serve {
		if err := serveScenario(*lang, *listen, *assetDelay); err != nil {
			fmt.Fprintln(os.Stderr, "gensample:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "gensample:", err)
		os.Exit(1)
	}
}

// serveScenario serves the lang scenario's fake Slack API on addr until the
// process is stopped. Unlike sample generation it accepts any Bearer token,
// because demo recordings type an arbitrary fake value at the token prompt;
// no real credential is involved either way, and the server only listens on
// the given (loopback by default) address.
func serveScenario(lang, addr string, assetDelay time.Duration) error {
	var sc *scenario
	switch lang {
	case "ja":
		sc = scenarioJA(time.Now())
	case "en":
		sc = scenarioEN(time.Now())
	default:
		return fmt.Errorf("unknown -lang %q (expected ja or en)", lang)
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	baseURL := "http://" + l.Addr().String()
	sc.replaceBaseURL(baseURL)
	f := &fakeSlackServer{sc: sc, assetDelay: assetDelay, anyBearer: true}
	fmt.Printf("serving the %s sample scenario (channel #%s) on %s\n", lang, sc.ChannelName, baseURL)
	fmt.Printf("run slapex against it with:\n\n  SLAPEX_API_BASE_URL=%s/api/ slapex\n\n", baseURL)
	fmt.Println("stop with Ctrl-C")
	return http.Serve(l, f.mux())
}

func run(out string) error {
	now := time.Now()
	for _, sc := range []*scenario{scenarioJA(now), scenarioEN(now)} {
		if err := buildSample(sc, out); err != nil {
			return fmt.Errorf("%s: %w", sc.Lang, err)
		}
	}
	return nil
}

// buildSample runs the export pipeline against sc's fake server and replaces
// out/<lang>/ with the generated index.html + style.css + assets/.
func buildSample(sc *scenario, out string) error {
	srv := newFakeSlackServer(sc)
	defer srv.Close()

	tmp, err := os.MkdirTemp("", "gensample-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	client := slack.New(fakeToken, slack.WithBaseURL(srv.URL()+"/api/"))
	printer := ui.NewPrinter(os.Stderr, false)
	dir, err := export.Run(context.Background(), client, export.Options{
		ChannelKeyword: sc.ChannelName,
		OutputDir:      tmp,
		MaxPosts:       1000,
		Days:           30,
		MaxAttachBytes: 10 << 20,
		NoInteractive:  true,
		ToolVersion:    "gensample",
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
