package export

// Integration case for the HTTP trace (Issue #273): an export with the trace
// on writes one line per request the fake server answered, and produces the
// same output as an export without it.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/kiyohara/slapex/internal/slack"
)

func TestRunIntegrationHTTPTrace(t *testing.T) {
	t.Parallel()

	// The happy path, with one download retried after a 503.
	scenario := func() exportScenario {
		sc := happyPathScenario()
		sc.AssetFaults = map[string]*endpointFault{
			"/files/og-launch.png": {transient: []faultResponse{{httpStatus: http.StatusServiceUnavailable}}},
		}
		return sc
	}
	plain := runExportScenario(t, scenario(), integrationOptions(t, 10))
	var trace bytes.Buffer
	traced, _, err := runExportScenarioRaw(t, scenario(), integrationOptions(t, 10), slack.WithTrace(&trace))
	if err != nil {
		t.Fatalf("Run() with the trace error = %v\nlogs:\n%s", err, strings.Join(traced.Logs, "\n"))
	}

	var recs []slack.TraceRecord
	sc := bufio.NewScanner(&trace)
	for sc.Scan() {
		var rec slack.TraceRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("trace line %q: %v", sc.Text(), err)
		}
		recs = append(recs, rec)
	}
	if n := traced.Server.Total(); len(recs) != n {
		t.Fatalf("trace has %d lines, want one per request the server answered (%d)", len(recs), n)
	}
	calls := map[string]int{}
	var ogImage []slack.TraceRecord
	for _, rec := range recs {
		switch rec.Type {
		case slack.TraceAPI:
			calls["/api/"+rec.Method]++
		case slack.TraceDownload:
			if rec.Kind == "" {
				t.Errorf("download line without an asset kind: %+v", rec)
			}
			if rec.Kind == "og_image" {
				ogImage = append(ogImage, rec)
			}
		default:
			t.Errorf("line of type %q: %+v", rec.Type, rec)
		}
	}
	for path, n := range calls {
		if got := traced.Server.Count(path); got != n {
			t.Errorf("trace has %d lines for %s, the server answered %d", n, path, got)
		}
	}
	if len(ogImage) != 2 || ogImage[0].Attempt != 0 || ogImage[0].Status != http.StatusServiceUnavailable ||
		ogImage[1].Attempt != 1 || ogImage[1].Status != http.StatusOK {
		t.Errorf("og_image lines = %+v, want the 503 attempt and the retry", ogImage)
	}
	for _, secret := range []string{integrationTestToken, "Bearer", "Acme", "acme", "project-alpha", "C123", "/files/", "/api/"} {
		if strings.Contains(trace.String(), secret) {
			t.Errorf("trace contains %q", secret)
		}
	}

	// The trace changes nothing else.
	a, b := exportFiles(t, plain), exportFiles(t, traced)
	if !slices.Equal(slices.Sorted(maps.Keys(a)), slices.Sorted(maps.Keys(b))) {
		t.Errorf("files with the trace differ:\nwithout: %v\nwith:    %v",
			slices.Sorted(maps.Keys(a)), slices.Sorted(maps.Keys(b)))
	}
	for name, content := range a {
		if other, ok := b[name]; ok && other != content {
			t.Errorf("%s differs with the trace:\nwithout:\n%s\nwith:\n%s", name, content, other)
		}
	}
	if a, b := normalizedLogs(plain), normalizedLogs(traced); !slices.Equal(a, b) {
		t.Errorf("logs with the trace differ:\nwithout:\n%s\nwith:\n%s", strings.Join(a, "\n"), strings.Join(b, "\n"))
	}
}

// exportFiles reads every file of the export, with the fake server URL, which
// differs between two runs, replaced.
func exportFiles(t *testing.T, got exportRunResult) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(got.OutputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(got.OutputDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files[rel] = strings.ReplaceAll(string(data), got.Server.URL(), "{{base}}")
		return nil
	})
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	return files
}

// durationInLog matches the durations the logs round to the second: a retry
// notice's wait has jitter, and the Done line reports the run's time.
var durationInLog = regexp.MustCompile(`in [0-9hms.]+`)

// normalizedLogs is the run's log with what differs between two runs
// replaced: the durations and the output directory.
func normalizedLogs(got exportRunResult) []string {
	out := make([]string, len(got.Logs))
	for i, line := range got.Logs {
		line = strings.ReplaceAll(line, got.OutputDir, "<out>")
		out[i] = durationInLog.ReplaceAllString(line, "in <d>")
	}
	return out
}
