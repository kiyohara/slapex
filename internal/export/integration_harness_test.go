package export

// Integration-test run helpers. runExportScenario / runExportScenarioRaw stand
// up a fake Slack server (integration_fakeserver_test.go) for one
// exportScenario (integration_fixture_test.go), run export.Run against it with
// a plain-mode printer whose lines are captured, and hand back the output
// directory, the server (for request counts) and the captured log lines.
// integrationOptions / renderingOptions build the Options the cases share.
//
// The --reuse-cache harness, which runs the export twice against one shared
// server, lives with its cases in integration_reuse_test.go.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// exportRunResult is what a run helper returns: the directory holding
// index.html, the fake server (query request counts with Server.Count) and the
// printer's output lines in order.
type exportRunResult struct {
	OutputDir string
	Server    *fakeSlackServer
	Logs      []string
}

// runExportScenario is the integration-test entry point for happy-path
// scenarios: define an exportScenario fixture, call this helper, then assert on
// the returned output directory and fake Slack request counters. It fails the
// test if Run returns an error; error-path scenarios use runExportScenarioRaw
// instead.
func runExportScenario(t *testing.T, sc exportScenario, opts Options) exportRunResult {
	t.Helper()

	got, _, err := runExportScenarioRaw(t, sc, opts)
	if err != nil {
		t.Fatalf("Run() error = %v\nlogs:\n%s", err, strings.Join(got.Logs, "\n"))
	}
	return got
}

// runExportScenarioRaw runs the export against a fresh fake server built from
// sc and returns the result, the durations passed to the injected sleeper, and
// the error from Run, without failing the test. Error-path scenarios assert on
// the returned error; rate-limit scenarios assert on the recorded sleeps. The
// sleeper never actually waits, so the tests run in real time regardless of the
// injected Retry-After / backoff durations.
//
// When opts.Now is zero, the export clock is pinned to one hour after the
// newest fixture message so a --days window always covers the fixture.
func runExportScenarioRaw(t *testing.T, sc exportScenario, opts Options) (exportRunResult, []time.Duration, error) {
	t.Helper()
	if opts.Now.IsZero() && len(sc.Messages) > 0 {
		latest := tsTime(sc.Messages[0].TS)
		for i := 1; i < len(sc.Messages); i++ {
			if candidate := tsTime(sc.Messages[i].TS); candidate.After(latest) {
				latest = candidate
			}
		}
		opts.Now = latest.Add(time.Hour)
	}

	fake := newFakeSlackServer(t, &sc)
	t.Cleanup(fake.Close)

	var (
		mu     sync.Mutex
		logs   []string
		sleeps []time.Duration
	)
	printer := testPrinter(func(line string) {
		mu.Lock()
		logs = append(logs, line)
		mu.Unlock()
	})
	client := slack.New(integrationTestToken,
		slack.WithBaseURL(fake.URL()+"/api/"),
		slack.WithSleeper(func(_ context.Context, d time.Duration) error {
			mu.Lock()
			sleeps = append(sleeps, d)
			mu.Unlock()
			return nil
		}),
	)
	client.Logf = printer.Noticef

	outDir, err := Run(context.Background(), client, opts, printer)

	mu.Lock()
	defer mu.Unlock()
	return exportRunResult{OutputDir: outDir, Server: fake, Logs: append([]string(nil), logs...)},
		append([]time.Duration(nil), sleeps...), err
}

// integrationOptions returns the Options every integration scenario shares:
// the "project-alpha" channel the fixtures declare, a fresh output directory, a
// 90-day window, a 1MB attachment limit and a kept cache so the .cache/ files
// can be asserted on. maxPosts is a parameter rather than a default because the
// cap decides what a scenario exercises (refill after an exclusion, the
// boundary at the limit), so every caller states it. Cases that need another
// range mode (--date / --from / --to), a smaller attachment limit or a pinned
// clock set those fields explicitly after the call.
func integrationOptions(t *testing.T, maxPosts int) Options {
	t.Helper()
	return Options{
		ChannelKeyword: "project-alpha",
		OutputDir:      t.TempDir(),
		MaxPosts:       maxPosts,
		Days:           90,
		MaxAttachBytes: 1 << 20, // 1MB
		KeepCache:      true,
		ToolVersion:    "test",
	}
}

// renderingOptions is integrationOptions with a --max-posts cap high enough
// that it never trims the fixture: the rendering and error cases exercise how
// messages display or fail, not how many are fetched.
func renderingOptions(t *testing.T) Options {
	t.Helper()
	return integrationOptions(t, 1000)
}
