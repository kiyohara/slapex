package ui

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestStyledForcedPlain(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

	tests := []struct {
		name    string
		env     map[string]string
		noColor bool
	}{
		{name: "no-color flag", noColor: true},
		{name: "NO_COLOR", env: map[string]string{"NO_COLOR": "1"}},
		{name: "TERM=dumb", env: map[string]string{"TERM": "dumb"}},
		{name: "CI=true", env: map[string]string{"CI": "true"}},
		{name: "CI other truthy value", env: map[string]string{"CI": "1"}},
		{name: "non-tty stream", env: nil},
		{name: "nil stream"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := devNull
			if tt.name == "nil stream" {
				f = nil
			}
			if Styled(f, envOf(tt.env), tt.noColor) {
				t.Errorf("Styled() = true, want false")
			}
		})
	}
}

func TestPlainPhaseLines(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, false)

	p.StartPhase("Messages", "fetching since 2026-06-02 ...")
	p.UpdatePhase("fetching since 2026-06-02 ... 200 fetched")
	p.EndPhase(StatusSuccess, "Messages", "345 fetched since 2026-06-02", "threads 12, replies 40")
	p.EndPhase(StatusWarn, "Assets", "30 saved, 2 skipped, 0 failed", "")

	want := strings.Join([]string{
		"INFO: messages: fetching since 2026-06-02 ...",
		"INFO: messages: fetching since 2026-06-02 ... 200 fetched",
		"OK: messages: 345 fetched since 2026-06-02 (threads 12, replies 40)",
		"WARN: assets: 30 saved, 2 skipped, 0 failed",
		"",
	}, "\n")
	if buf.String() != want {
		t.Errorf("plain phase output = %q, want %q", buf.String(), want)
	}
}

func TestPlainOutputHasNoEscapesOrCursorControl(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, false)

	p.StartPhase("Workspace", "checking token ...")
	p.UpdatePhase("still checking ...")
	p.Noticef("rate limited on auth.test, waiting 30s")
	p.Warnf("could not resolve user U123")
	p.Errorf("boom")
	p.Successf("done")
	p.Infof("secondary detail")
	p.Plainf("  output: /tmp/example")
	p.EndPhase(StatusSuccess, "Workspace", "Example", "example.slack.com")
	p.StopPhase()

	out := buf.String()
	for _, banned := range []string{"\x1b", "\r", "✓", "✗", "⠋"} {
		if strings.Contains(out, banned) {
			t.Errorf("plain output contains %q:\n%s", banned, out)
		}
	}
	for _, want := range []string{
		"INFO: workspace: rate limited on auth.test, waiting 30s",
		"WARN: could not resolve user U123",
		"ERROR: boom",
		"OK: done",
		"INFO: secondary detail",
		"  output: /tmp/example",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plain output missing %q:\n%s", want, out)
		}
	}
}

func TestStyledPhaseLifecycle(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, true)

	p.StartPhase("Messages", "fetching ...")
	p.UpdatePhase("fetching ... 200")
	p.EndPhase(StatusSuccess, "Messages", "345 fetched", "threads 12")

	out := buf.String()
	for _, want := range []string{
		"⠋",                          // initial spinner frame
		"\r\x1b[2K",                  // line rewrite, no other cursor control
		"\x1b[32m✓\x1b[0m",           // green success glyph
		"\x1b[1mMessages \x1b[0m",    // bold label padded to the column width
		"\x1b[2m(threads 12)\x1b[0m", // dim parenthesized meta
	} {
		if !strings.Contains(out, want) {
			t.Errorf("styled output missing %q:\n%q", want, out)
		}
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("styled output does not end with a newline: %q", out)
	}
}

func TestStyledStandaloneAboveSpinner(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, true)

	p.StartPhase("Users", "resolving 12 users ...")
	p.Warnf("could not resolve user U123")
	p.EndPhase(StatusSuccess, "Users", "11 resolved", "")

	out := buf.String()
	if !strings.Contains(out, "\x1b[33m!\x1b[0m could not resolve user U123\n") {
		t.Errorf("styled warning line missing or malformed:\n%q", out)
	}
	warnIdx := strings.Index(out, "could not resolve")
	finalIdx := strings.Index(out, "11 resolved")
	if warnIdx > finalIdx {
		t.Errorf("warning printed after final phase line:\n%q", out)
	}
}

func TestEndPhaseWithoutStartPhase(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, true)
	p.EndPhase(StatusSuccess, "Emoji", "58 custom emoji", "from cache")
	if !strings.Contains(buf.String(), "58 custom emoji") {
		t.Errorf("EndPhase without StartPhase produced no final line: %q", buf.String())
	}
}

func TestNoticefUpdatesLivePhase(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, true)

	p.StartPhase("Messages", "fetching ...")
	p.Noticef("rate limited, waiting 30s")
	out := buf.String()
	if !strings.Contains(out, "rate limited, waiting 30s") {
		t.Errorf("Noticef did not update live phase text:\n%q", out)
	}
	if strings.Contains(out, "\x1b[2m- ") {
		t.Errorf("Noticef with live phase should not print a standalone info line:\n%q", out)
	}
	p.StopPhase()
}

func TestStopPhaseErasesLine(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, true)
	p.StartPhase("Channel", "listing channels ...")
	p.StopPhase()
	if !strings.HasSuffix(buf.String(), clearLine) {
		t.Errorf("StopPhase did not erase the live line: %q", buf.String())
	}
}

// TestMuteDropsLaterOutput: a muted printer writes nothing more, in either
// mode, and its spinner stops.
func TestMuteDropsLaterOutput(t *testing.T) {
	for _, styled := range []bool{false, true} {
		w := &gateWriter{}
		p := NewPrinter(w, styled)
		p.StartPhase("Assets", "downloading assets ...")
		p.Mute()

		p.UpdatePhase("downloading assets ... 3")
		p.Noticef("retrying download in 1s")
		p.Infof("info")
		p.Warnf("asset failed")
		p.Errorf("slapex: context canceled")
		p.Successf("done")
		p.Plainf("plain")
		p.EndPhase(StatusSuccess, "Assets", "3 saved", "")
		p.StartPhase("Done", "writing ...")
		// A spinner frame that was being drawn as Mute came has ended by now.
		time.Sleep(spinnerInterval / 2)
		before := w.writes()
		time.Sleep(3 * spinnerInterval)
		p.StopPhase()
		got := w.writes()
		if len(got) != len(before) {
			t.Errorf("styled=%v: went on writing %q after Mute", styled, got[len(before):])
		}
		// Only the phase started before Mute was drawn.
		for _, s := range got {
			if !strings.Contains(s, "downloading assets ...") || strings.Contains(s, "... 3") {
				t.Errorf("styled=%v: wrote %q after Mute", styled, s)
			}
		}
	}
}

// TestMuteWithStuckWrite: Mute does not wait for a write that cannot go on,
// as one to a stderr nobody reads, and what comes after it returns at once
// too. The stuck write ends in its own time, and nothing is written after it:
// the spinner, which came to draw while the write was stuck, draws no more.
func TestMuteWithStuckWrite(t *testing.T) {
	for _, styled := range []bool{false, true} {
		w := &gateWriter{hold: "stuck", held: make(chan struct{}), release: make(chan struct{})}
		release := sync.OnceFunc(func() { close(w.release) })
		defer release()
		p := NewPrinter(w, styled)
		p.StartPhase("Assets", "downloading assets ...")
		warned := make(chan struct{})
		go func() {
			p.Warnf("stuck")
			close(warned)
		}()
		waitFor(t, w.held, "the write to get stuck")
		time.Sleep(2 * spinnerInterval)
		muted := make(chan struct{})
		go func() {
			p.Mute()
			p.UpdatePhase("downloading assets ... 3")
			p.Noticef("retrying download in 1s")
			p.Warnf("asset failed")
			p.Plainf("plain")
			p.EndPhase(StatusSuccess, "Assets", "3 saved", "")
			p.StartPhase("Done", "writing ...")
			p.StopPhase()
			close(muted)
		}()
		waitFor(t, muted, "Mute and what comes after it to return while a write is stuck")
		release()
		waitFor(t, warned, "the stuck write to end")
		time.Sleep(3 * spinnerInterval)

		writes := w.writes()
		n := len(writes)
		ok := n >= 1 && strings.Contains(writes[n-1], "stuck")
		if styled {
			// The phase line is drawn again under the warning.
			ok = n >= 2 && strings.Contains(writes[n-2], "stuck") && strings.Contains(writes[n-1], "downloading assets ...")
		}
		if !ok {
			t.Errorf("styled=%v: wrote %q, want the stuck warning last", styled, writes)
		}
	}
}

// gateWriter records the writes to it. With hold set, the write that contains
// hold waits until release is closed, as one to a stderr nobody reads does.
type gateWriter struct {
	hold    string
	held    chan struct{} // closed when the write with hold starts
	release chan struct{}

	mu   sync.Mutex
	done []string
}

func (w *gateWriter) Write(b []byte) (int, error) {
	if w.hold != "" && bytes.Contains(b, []byte(w.hold)) {
		close(w.held)
		<-w.release
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.done = append(w.done, string(b))
	return len(b), nil
}

func (w *gateWriter) writes() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.done...)
}

// waitFor fails the test when ch is not closed in time.
func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}
