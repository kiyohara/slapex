package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kiyohara/slapex/internal/slack"
	"github.com/kiyohara/slapex/internal/ui"
)

// httpTraceEnv names a file for the HTTP trace of the export: one JSON line
// per HTTP request to Slack or to an asset host (internal/slack trace.go,
// Issue #273). Internal use only, like apiBaseURLEnv: it is for measuring
// where an export spends its time (tools/tracereport), not part of the public
// CLI surface, and stays out of --help and user-facing docs
// (doc/design/cli-interface.md). --demo is not traced: it only talks to its
// in-process fixture server.
const httpTraceEnv = "SLAPEX_HTTP_TRACE"

// httpTraceFile is the trace file of a run. Its methods take a nil
// *httpTraceFile as a run without a trace.
type httpTraceFile struct {
	f   *os.File
	err error // the first failed write
}

// openHTTPTrace creates (or truncates) the file httpTraceEnv names, readable
// by the user only. It returns nil when the variable is unset or blank.
func openHTTPTrace(getenv func(string) string) (*httpTraceFile, error) {
	path := getenv(httpTraceEnv)
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", httpTraceEnv, err)
	}
	return &httpTraceFile{f: f}, nil
}

// Write writes one trace line. The Slack client writes the lines one at a
// time and ignores their errors (slack.WithTrace), so the first one is kept
// for finish.
func (t *httpTraceFile) Write(p []byte) (int, error) {
	n, err := t.f.Write(p)
	if err != nil && t.err == nil {
		t.err = err
	}
	return n, err
}

// clientOptions returns the Slack client options that write the trace to t.
func (t *httpTraceFile) clientOptions() []slack.Option {
	if t == nil {
		return nil
	}
	return []slack.Option{slack.WithTrace(t)}
}

// finish closes the trace file and warns when the trace is incomplete. The
// export's own result and exit code stay as they are.
func (t *httpTraceFile) finish(p *ui.Printer) {
	if t == nil {
		return
	}
	err := t.err
	if cerr := t.f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		p.Warnf("%s: the trace is incomplete: %v", httpTraceEnv, err)
	}
}
