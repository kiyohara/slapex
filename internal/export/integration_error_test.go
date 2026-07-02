package export

// Integration error / rate-limit scenarios for v1-09 (Issue #23). Each case is
// an independent test that builds a minimal fixture on top of the v1-07 fake
// Slack server harness (exportScenario / runExportScenarioRaw) and injects an
// error or rate-limit fault through the exportScenario.APIFaults /
// AssetFaults maps. The 429 path cannot be exercised safely against the real
// Slack API, so these tests are the practical guard for it.
//
// Expected behaviour follows the confirmed specs:
//   - exit code mapping and partial-failure handling: doc/design/cli-interface.md
//   - rate limit / retry policy (429 + Retry-After, 5xx backoff, max 5 retries):
//     doc/design/slack-api-usage.md「rate limit とリトライ」
//   - help URL guidance for auth / permission failures: doc/design/usage-flow.md
//
// These tests assert the error type export.Run returns for each Slack
// condition. The error-type -> exit code mapping (cmd/slapex.classify) and the
// auth help URL are asserted against the real cmd-layer code in
// cmd/slapex.TestReportRunError; classify itself is unit-tested in
// cmd/slapex.TestClassify (v1-03 / Issue #17). The expected exit code is named
// in each case comment to tie the two halves together.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/slack"
)

// runExportScenarioRaw runs the export against a fresh fake server built from
// sc and returns the result, the durations passed to the injected sleeper, and
// the error from Run, without failing the test. Error-path scenarios assert on
// the returned error; rate-limit scenarios assert on the recorded sleeps. The
// sleeper never actually waits, so the tests run in real time regardless of the
// injected Retry-After / backoff durations.
func runExportScenarioRaw(t *testing.T, sc exportScenario, opts Options) (exportRunResult, []time.Duration, error) {
	t.Helper()

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

func hasSleepAtLeast(sleeps []time.Duration, atLeast time.Duration) bool {
	for _, d := range sleeps {
		if d >= atLeast {
			return true
		}
	}
	return false
}

func logsContain(logs []string, substr string) bool {
	for _, line := range logs {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

// --- case 1: auth.test invalid_auth -> auth error (exit 3) -------------------

func TestRunIntegrationAuthInvalid(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.APIFaults = map[string]*endpointFault{
		"/api/auth.test": {sticky: &faultResponse{slackError: "invalid_auth"}},
	}

	_, _, err := runExportScenarioRaw(t, sc, renderingOptions(t))

	// invalid_auth is in authErrorCodes -> classify => exit 3.
	var apiErr *slack.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "invalid_auth" {
		t.Fatalf("Run() error = %v, want *slack.APIError with code invalid_auth", err)
	}
}

// --- case 2: conversations.history missing_scope -> auth error (exit 3) ------

func TestRunIntegrationMissingScope(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.APIFaults = map[string]*endpointFault{
		"/api/conversations.history": {sticky: &faultResponse{slackError: "missing_scope"}},
	}

	_, _, err := runExportScenarioRaw(t, sc, renderingOptions(t))

	// missing_scope is in authErrorCodes -> classify => exit 3; the cmd layer
	// also prints the setup help URL (TestReportRunError).
	var apiErr *slack.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "missing_scope" {
		t.Fatalf("Run() error = %v, want *slack.APIError with code missing_scope", err)
	}
}

// --- case 3: conversations.history not_in_channel -> auth error (exit 3) -----

func TestRunIntegrationNotInChannel(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.APIFaults = map[string]*endpointFault{
		"/api/conversations.history": {sticky: &faultResponse{slackError: "not_in_channel"}},
	}

	_, _, err := runExportScenarioRaw(t, sc, renderingOptions(t))

	// not_in_channel (bot not a member) -> classify => exit 3; the cmd layer
	// prints the setup help URL (TestReportRunError).
	var apiErr *slack.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "not_in_channel" {
		t.Fatalf("Run() error = %v, want *slack.APIError with code not_in_channel", err)
	}
}

// --- case 4: 429 + Retry-After once, then success (exit 0) -------------------

func TestRunIntegrationRateLimitRetryThenSuccess(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000001.000000", User: "U01", Text: "hello"},
	}
	sc.APIFaults = map[string]*endpointFault{
		"/api/conversations.history": {
			transient: []faultResponse{{httpStatus: http.StatusTooManyRequests, retryAfterSec: 1}},
		},
	}

	got, sleeps, err := runExportScenarioRaw(t, sc, renderingOptions(t))
	if err != nil {
		t.Fatalf("Run() error = %v, want success after honouring Retry-After\nlogs:\n%s",
			err, strings.Join(got.Logs, "\n"))
	}

	// history was hit twice: the 429, then the successful retry.
	if n := got.Server.Count("/api/conversations.history"); n != 2 {
		t.Fatalf("conversations.history count = %d, want 2 (429 then success)", n)
	}
	// The Retry-After wait (>= 1s) was recorded; per-method pacing waits are
	// strictly < 1s, so a >= 1s sleep is the honoured Retry-After.
	if !hasSleepAtLeast(sleeps, time.Second) {
		t.Fatalf("no sleep >= 1s recorded, want the honoured Retry-After wait; sleeps=%v", sleeps)
	}
	// The wait is reported as progress on stderr (slack-api-usage.md).
	if !logsContain(got.Logs, "rate limited on api conversations.history, waiting") {
		t.Fatalf("logs missing rate-limit wait progress message:\n%s", strings.Join(got.Logs, "\n"))
	}
}

// --- case 5: history 429 forever -> retry limit reached (exit 4) -------------

func TestRunIntegrationRateLimitExhausted(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.APIFaults = map[string]*endpointFault{
		"/api/conversations.history": {
			sticky: &faultResponse{httpStatus: http.StatusTooManyRequests, retryAfterSec: 1},
		},
	}

	got, _, err := runExportScenarioRaw(t, sc, renderingOptions(t))
	if err == nil {
		t.Fatalf("Run() succeeded, want a retry-exhaustion error")
	}

	// Retry exhaustion is a plain error: neither a usage error nor a Slack
	// APIError, so classify falls through to exit 4 (cli-interface.md).
	var usageErr *UsageError
	var apiErr *slack.APIError
	if errors.As(err, &usageErr) || errors.As(err, &apiErr) {
		t.Fatalf("Run() error = %v, want a plain retry-exhaustion error (=> exit 4)", err)
	}
	if !strings.Contains(err.Error(), "giving up after 5 retries") {
		t.Fatalf("Run() error = %v, want 'giving up after 5 retries'", err)
	}
	// Initial attempt + 5 retries (slack-api-usage.md「最大 5 回」).
	if n := got.Server.Count("/api/conversations.history"); n != 6 {
		t.Fatalf("conversations.history count = %d, want 6 (initial + 5 retries)", n)
	}
}

// --- case 6: asset download fails for good -> partial failure (exit 0) -------

// Distinct from the v1-08 404 case: a persistent 5xx exercises the download
// retry-then-give-up path. The export still succeeds and the failure is
// recorded in the manifest rather than aborting the run (cli-interface.md
// 部分失敗の扱い).
func TestRunIntegrationAssetDownloadRetriesThenFails(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{
			Type: "message",
			TS:   "1700000001.000000",
			User: "U01",
			Text: "Report attached",
			Files: []slack.File{
				{
					ID:                 "F-FLAKY",
					Name:               "report.pdf",
					Mimetype:           "application/pdf",
					Size:               50, // under the limit: the failure is the 5xx, not the size
					URLPrivateDownload: "{{base}}/files/flaky.pdf",
				},
			},
		},
	}
	sc.AssetFaults = map[string]*endpointFault{
		"/files/flaky.pdf": {sticky: &faultResponse{httpStatus: http.StatusInternalServerError}},
	}

	// runExportScenario fatals if Run returns an error, so reaching the
	// assertions proves the export as a whole succeeded (exit 0).
	got := runExportScenario(t, sc, renderingOptions(t))
	body := readIndexHTML(t, got.OutputDir)

	mustContain(t, body, `<span class="file-link unavailable">📄 report.pdf</span>`)
	mustContain(t, body, "取得に失敗しました。")

	entry, ok := findManifest(readManifestEntries(t, got.OutputDir), func(e manifestEntryFull) bool {
		return e.Kind == "attachment" && e.FileID == "F-FLAKY"
	})
	if !ok || entry.Status != "failed" {
		t.Fatalf("attachment entry = %+v (ok=%v), want status failed", entry, ok)
	}
	// The download was retried to the limit before being recorded as failed.
	if n := got.Server.Count("/files/flaky.pdf"); n != 6 {
		t.Fatalf("download count = %d, want 6 (initial + 5 retries)", n)
	}
}

// --- case 7: transient 5xx -> backoff retry succeeds (exit 0) ----------------

func TestRunIntegrationTransientServerErrorRecovers(t *testing.T) {
	t.Parallel()

	sc := baseScenario()
	sc.Messages = []slack.Message{
		{Type: "message", TS: "1700000001.000000", User: "U01", Text: "hello"},
	}
	sc.APIFaults = map[string]*endpointFault{
		"/api/conversations.history": {
			transient: []faultResponse{{httpStatus: http.StatusServiceUnavailable}},
		},
	}

	got, _, err := runExportScenarioRaw(t, sc, renderingOptions(t))
	if err != nil {
		t.Fatalf("Run() error = %v, want success after backoff retry\nlogs:\n%s",
			err, strings.Join(got.Logs, "\n"))
	}

	// history was hit twice: the 503, then the successful retry.
	if n := got.Server.Count("/api/conversations.history"); n != 2 {
		t.Fatalf("conversations.history count = %d, want 2 (503 then success)", n)
	}
	// The backoff retry is reported on stderr.
	if !logsContain(got.Logs, "retrying api conversations.history") {
		t.Fatalf("logs missing backoff retry message:\n%s", strings.Join(got.Logs, "\n"))
	}
}

// --- exit-2 coverage: no channel matched -> usage error (exit 2) ------------

// Not one of the seven error/rate-limit cases, but the acceptance criteria
// require the exit 2 mapping to be asserted alongside 3 / 4 / partial-failure 0.
// A keyword that matches no channel makes chooseChannel return a UsageError,
// which classify maps to exit 2 (cli-interface.md); asserted at the cmd layer
// in TestReportRunError.
func TestRunIntegrationNoChannelMatch(t *testing.T) {
	t.Parallel()

	sc := baseScenario() // only channel: project-alpha
	opts := renderingOptions(t)
	opts.ChannelKeyword = "does-not-exist"

	_, _, err := runExportScenarioRaw(t, sc, opts)

	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("Run() error = %v, want *UsageError (=> exit 2)", err)
	}
}
