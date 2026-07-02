// Package ui renders slapex's stderr progress and diagnostics
// (doc/design/cli-interface.md「入出力ストリーム」). It has two modes:
//
//   - styled: for interactive terminals. Status glyphs (✓ ! ✗) in basic ANSI
//     colors, a bold phase-label column, dim metadata, and a braille spinner
//     that live-updates the current phase line.
//   - plain: for CI / pipes / log collectors. Deterministic, append-only
//     lines with ASCII prefixes (OK: / WARN: / ERROR: / INFO:); no ANSI
//     escape sequences, no cursor control, no decorative symbols.
//
// The style follows the phase-line model chosen in Issue #100 (see
// working-branch-notes and decision log 0045): colors mark status glyphs
// only, values stay in the terminal's default color.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// Status classifies a finished phase or a standalone message.
type Status int

const (
	StatusInfo Status = iota
	StatusSuccess
	StatusWarn
	StatusError
)

// plainPrefix is the ASCII prefix contract for plain mode
// (Issue #100「表示設計案」).
func (s Status) plainPrefix() string {
	switch s {
	case StatusSuccess:
		return "OK:"
	case StatusWarn:
		return "WARN:"
	case StatusError:
		return "ERROR:"
	default:
		return "INFO:"
	}
}

func (s Status) glyph() (symbol, color string) {
	switch s {
	case StatusSuccess:
		return "✓", ansiGreen
	case StatusWarn:
		return "!", ansiYellow
	case StatusError:
		return "✗", ansiRed
	default:
		return "-", ansiDim
	}
}

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"

	// clearLine returns the cursor to column 1 and erases the line; the only
	// cursor control used, and only in styled mode.
	clearLine = "\r\x1b[2K"
)

// labelWidth aligns the phase-label column ("Workspace" is the widest label).
const labelWidth = 9

// spinnerFrames is the braille spinner (Issue #100 user decision; the same
// set uv and the JS ora library use).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 100 * time.Millisecond

// Styled reports whether decorated output should be used for f. Plain output
// is forced by --no-color (noColor), NO_COLOR, TERM=dumb, CI, or f not being
// a terminal (doc/design/cli-interface.md「出力制御」). CI is treated as "set
// to any non-empty value": major CI services set CI=true, but some set other
// truthy values, and log stability is the safe default there.
func Styled(f *os.File, getenv func(string) string, noColor bool) bool {
	if noColor || getenv("NO_COLOR") != "" || getenv("TERM") == "dumb" || getenv("CI") != "" {
		return false
	}
	return f != nil && term.IsTerminal(int(f.Fd()))
}

// Printer writes progress and diagnostics to a single stream (stderr in
// production). All methods are safe for concurrent use.
type Printer struct {
	mu     sync.Mutex
	w      io.Writer
	styled bool

	phase   *phaseState
	frame   int
	stopped chan struct{} // closes to stop the spinner goroutine
}

type phaseState struct {
	label string
	text  string
}

// NewPrinter returns a Printer writing to w. styled selects the decorated
// TTY mode; callers derive it via Styled.
func NewPrinter(w io.Writer, styled bool) *Printer {
	return &Printer{w: w, styled: styled}
}

// StartPhase opens a live phase line. Styled mode draws a spinner line that
// is redrawn in place until EndPhase or StopPhase; plain mode prints one
// "INFO: <label>: <text>" line.
func (p *Printer) StartPhase(label, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.styled {
		p.phase = &phaseState{label: label, text: text}
		fmt.Fprintf(p.w, "INFO: %s: %s\n", strings.ToLower(label), text)
		return
	}
	p.clearPhaseLocked()
	p.phase = &phaseState{label: label, text: text}
	p.drawPhaseLocked()
	p.startSpinnerLocked()
}

// UpdatePhase replaces the live phase text (progress counters, waits). Plain
// mode prints the update as its own INFO line so long waits remain visible
// in CI logs.
func (p *Printer) UpdatePhase(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.phase == nil {
		return
	}
	p.phase.text = text
	if !p.styled {
		fmt.Fprintf(p.w, "INFO: %s: %s\n", strings.ToLower(p.phase.label), text)
		return
	}
	p.drawPhaseLocked()
}

// EndPhase closes any live phase and prints the final status line for label.
// meta is secondary detail rendered dim and parenthesized in styled mode,
// and parenthesized in plain mode; pass "" for none. EndPhase also works
// without a preceding StartPhase (e.g. results that came from cache).
func (p *Printer) EndPhase(status Status, label, text, meta string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopSpinnerLocked()
	if !p.styled {
		line := fmt.Sprintf("%s %s: %s", status.plainPrefix(), strings.ToLower(label), text)
		if meta != "" {
			line += " (" + meta + ")"
		}
		fmt.Fprintln(p.w, line)
		p.phase = nil
		return
	}
	p.clearPhaseLocked()
	p.phase = nil
	symbol, color := status.glyph()
	line := color + symbol + ansiReset + " " + ansiBold + pad(label) + ansiReset + " " + text
	if meta != "" {
		line += " " + ansiDim + "(" + meta + ")" + ansiReset
	}
	fmt.Fprintln(p.w, line)
}

// StopPhase abandons the live phase without a final line: the spinner stops
// and the styled line is erased. Used before interactive prompts and before
// error reporting, so the spinner never fights other terminal output.
func (p *Printer) StopPhase() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopSpinnerLocked()
	p.clearPhaseLocked()
	p.phase = nil
}

// Noticef reports a transient client-level event (rate limit wait, retry).
// While a phase is live it replaces the spinner text until the next update;
// otherwise it behaves like Infof. Plain mode always prints an INFO line.
func (p *Printer) Noticef(format string, args ...any) {
	p.mu.Lock()
	if p.phase != nil {
		p.mu.Unlock()
		p.UpdatePhase(fmt.Sprintf(format, args...))
		return
	}
	p.mu.Unlock()
	p.Infof(format, args...)
}

// Infof prints a standalone secondary line (dim in styled mode).
func (p *Printer) Infof(format string, args ...any) {
	p.standalone(StatusInfo, fmt.Sprintf(format, args...))
}

// Warnf prints a standalone warning line ("! ..." / "WARN: ...").
func (p *Printer) Warnf(format string, args ...any) {
	p.standalone(StatusWarn, fmt.Sprintf(format, args...))
}

// Errorf prints a standalone error line ("✗ ..." / "ERROR: ...").
func (p *Printer) Errorf(format string, args ...any) {
	p.standalone(StatusError, fmt.Sprintf(format, args...))
}

// Successf prints a standalone success line ("✓ ..." / "OK: ...").
func (p *Printer) Successf(format string, args ...any) {
	p.standalone(StatusSuccess, fmt.Sprintf(format, args...))
}

// Plainf prints text identically in both modes: no prefix, no decoration.
// Used for usage guidance blocks and summary detail lines whose content must
// stay copy-pasteable.
func (p *Printer) Plainf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	line := fmt.Sprintf(format, args...)
	if p.styled && p.phase != nil {
		fmt.Fprint(p.w, clearLine)
		fmt.Fprintln(p.w, line)
		p.drawPhaseLocked()
		return
	}
	fmt.Fprintln(p.w, line)
}

// standalone prints a status-prefixed line, keeping a live spinner line
// intact by printing above it.
func (p *Printer) standalone(status Status, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var line string
	if p.styled {
		symbol, color := status.glyph()
		if status == StatusInfo {
			line = ansiDim + symbol + " " + text + ansiReset
		} else {
			line = color + symbol + ansiReset + " " + text
		}
	} else {
		line = status.plainPrefix() + " " + text
	}
	if p.styled && p.phase != nil {
		fmt.Fprint(p.w, clearLine)
		fmt.Fprintln(p.w, line)
		p.drawPhaseLocked()
		return
	}
	fmt.Fprintln(p.w, line)
}

// --- styled internals --------------------------------------------------------

func (p *Printer) drawPhaseLocked() {
	frame := spinnerFrames[p.frame%len(spinnerFrames)]
	fmt.Fprint(p.w, clearLine+ansiCyan+frame+ansiReset+" "+ansiBold+pad(p.phase.label)+ansiReset+" "+ansiDim+p.phase.text+ansiReset)
}

func (p *Printer) clearPhaseLocked() {
	if p.phase != nil {
		fmt.Fprint(p.w, clearLine)
	}
}

func (p *Printer) startSpinnerLocked() {
	if p.stopped != nil {
		return
	}
	stop := make(chan struct{})
	p.stopped = stop
	go func() {
		ticker := time.NewTicker(spinnerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				p.mu.Lock()
				if p.phase != nil {
					p.frame++
					p.drawPhaseLocked()
				}
				p.mu.Unlock()
			}
		}
	}()
}

func (p *Printer) stopSpinnerLocked() {
	if p.stopped != nil {
		close(p.stopped)
		p.stopped = nil
	}
}

func pad(label string) string {
	if len(label) >= labelWidth {
		return label
	}
	return label + strings.Repeat(" ", labelWidth-len(label))
}
