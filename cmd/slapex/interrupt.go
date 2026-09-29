// Ctrl-C (SIGINT) and SIGTERM during an export (Issue #275). The export's
// context is canceled, so the parallel downloads stop and remove their
// temporary files (internal/output), and the process then ends by the same
// signal, as it did before slapex caught it: the shell sees the signal (exit
// status 130 for Ctrl-C), and nothing more is printed. A second signal, or an
// export that takes longer than interruptGrace to stop, ends the process at
// once.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kiyohara/slapex/internal/ui"
)

// interruptSignals stop an export.
var interruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// interruptGrace is how long an interrupted export has to stop.
const interruptGrace = 5 * time.Second

// watchInterrupts watches for interruptSignals from here on, and returns the
// context to run the export with and the function that ends the watch. When a
// signal comes, p is muted and the context canceled. The end function returns
// when no signal came; when one did, it ends the process by that signal and
// does not return. A signal the process started out ignoring (a background
// job of a shell script ignores SIGINT) stays ignored.
//
// It is called after the token prompt, which does not stop on a canceled
// context: Ctrl-C at the prompt still ends the process at once.
func watchInterrupts(p *ui.Printer) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	var sigs []os.Signal
	for _, sig := range interruptSignals {
		if !signal.Ignored(sig) {
			sigs = append(sigs, sig)
		}
	}
	if len(sigs) == 0 {
		return ctx, cancel
	}
	// Two signals fit: a second one that comes before the watch has reset
	// the signals still ends the process (interruptWatch.run).
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, sigs...)
	w := startInterruptWatch(ch, interruptHooks{
		cancel: cancel,
		mute:   p.Mute,
		// From the first signal on, the next one ends the process at once.
		reset: func() { signal.Reset(sigs...) },
		die:   dieBySignal,
		grace: interruptGrace,
	})
	return ctx, func() {
		signal.Stop(ch)
		w.stop()
		cancel()
	}
}

// interruptHooks are what an interruptWatch does when a signal comes.
type interruptHooks struct {
	cancel context.CancelFunc
	mute   func() // must not wait: canceling and the grace come after it
	reset  func()
	die    func(os.Signal) // ends the process by the signal
	grace  time.Duration
}

// interruptWatch handles the first signal from sigs.
type interruptWatch struct {
	sigs  <-chan os.Signal
	hooks interruptHooks
	quit  chan struct{} // closed by stop
	done  chan struct{} // closed when the watch has ended

	caught os.Signal // read once done is closed
}

func startInterruptWatch(sigs <-chan os.Signal, hooks interruptHooks) *interruptWatch {
	w := &interruptWatch{sigs: sigs, hooks: hooks, quit: make(chan struct{}), done: make(chan struct{})}
	go w.run()
	return w
}

func (w *interruptWatch) run() {
	defer close(w.done)
	select {
	case sig := <-w.sigs:
		w.caught = sig
		// The reset comes first, so that the next signal ends the process
		// whatever comes after. A signal that came before the reset is read
		// here.
		w.hooks.reset()
		w.hooks.mute()
		w.hooks.cancel()
		select {
		case next := <-w.sigs:
			w.hooks.die(next)
		case <-time.After(w.hooks.grace):
			w.hooks.die(sig)
		case <-w.quit:
		}
	case <-w.quit:
		// A signal that came before the watch ended counts.
		select {
		case sig := <-w.sigs:
			w.caught = sig
		default:
		}
	}
}

// stop ends the watch once the export has returned. When a signal came, it
// ends the process by that signal.
func (w *interruptWatch) stop() {
	close(w.quit)
	<-w.done
	if w.caught != nil {
		w.hooks.die(w.caught)
	}
}

// dieBySignal ends the process by sig, as the Go runtime does for a signal
// nothing catches. Should the process outlive the signal, it exits with the
// status a shell gives a process a signal ended: 128 + the signal's number.
func dieBySignal(sig os.Signal) {
	signal.Reset(sig)
	if proc, err := os.FindProcess(os.Getpid()); err == nil && proc.Signal(sig) == nil {
		time.Sleep(time.Second)
	}
	code := exitOther
	if s, ok := sig.(syscall.Signal); ok {
		code = 128 + int(s)
	}
	os.Exit(code)
}
