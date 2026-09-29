package main

// Ctrl-C and SIGTERM during an export (Issue #275): the watch cancels the
// export's context and mutes the printer, and the process then ends by the
// signal. The watch's own rules are tested in process, with fake hooks; the
// signals themselves in a child process, which the test sends them to.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/kiyohara/slapex/internal/ui"
)

// fakeInterruptHooks records what an interruptWatch does. die returns, where
// the real one ends the process.
type fakeInterruptHooks struct {
	ctx    context.Context
	muted  chan struct{}
	resets chan struct{}
	died   chan os.Signal
}

func newFakeInterruptHooks(grace time.Duration) (*fakeInterruptHooks, interruptHooks) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeInterruptHooks{
		ctx:    ctx,
		muted:  make(chan struct{}, 4),
		resets: make(chan struct{}, 4),
		died:   make(chan os.Signal, 4),
	}
	return f, interruptHooks{
		cancel: cancel,
		mute:   func() { f.muted <- struct{}{} },
		reset:  func() { f.resets <- struct{}{} },
		die:    func(sig os.Signal) { f.died <- sig },
		grace:  grace,
	}
}

func TestInterruptWatchWithoutSignal(t *testing.T) {
	t.Parallel()

	f, hooks := newFakeInterruptHooks(time.Hour)
	w := startInterruptWatch(make(chan os.Signal, 1), hooks)
	w.stop()
	if f.ctx.Err() != nil || len(f.muted) != 0 || len(f.resets) != 0 || len(f.died) != 0 {
		t.Fatalf("without a signal: canceled %v, muted %d, reset %d, died %d; want nothing done",
			f.ctx.Err(), len(f.muted), len(f.resets), len(f.died))
	}
}

// TestInterruptWatchOnSignal: the first signal mutes the printer, cancels the
// export and lets the next signal end the process; once the export has
// returned, the process ends by the signal.
func TestInterruptWatchOnSignal(t *testing.T) {
	t.Parallel()

	f, hooks := newFakeInterruptHooks(time.Hour)
	sigs := make(chan os.Signal, 1)
	w := startInterruptWatch(sigs, hooks)
	sigs <- syscall.SIGTERM
	within(t, f.ctx.Done(), "the export to be canceled")
	within(t, f.muted, "the printer to be muted")
	within(t, f.resets, "the signals to be reset")
	if len(f.died) != 0 {
		t.Fatalf("ended the process before the export returned")
	}
	w.stop()
	if sig := within(t, f.died, "the process to end"); sig != syscall.SIGTERM {
		t.Fatalf("ended by %v, want SIGTERM", sig)
	}
}

// TestInterruptWatchSecondSignal: the signals are reset before anything else,
// so the next signal ends the process at once whatever comes after (here, a
// mute that waits); a second signal that came before the reset ends it too.
func TestInterruptWatchSecondSignal(t *testing.T) {
	t.Parallel()

	f, hooks := newFakeInterruptHooks(time.Hour)
	unmute := make(chan struct{})
	hooks.mute = func() { <-unmute; f.muted <- struct{}{} }
	sigs := make(chan os.Signal, 2)
	sigs <- os.Interrupt
	sigs <- syscall.SIGTERM
	w := startInterruptWatch(sigs, hooks)
	defer w.stop()
	release := sync.OnceFunc(func() { close(unmute) })
	defer release()
	within(t, f.resets, "the signals to be reset while muting waits")
	release()
	if sig := within(t, f.died, "the second signal to end the process"); sig != syscall.SIGTERM {
		t.Fatalf("ended by %v, want the second signal", sig)
	}
}

// within receives from ch, and fails the test when nothing comes.
func within[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		panic("unreachable")
	}
}

// TestInterruptWatchGrace: an export that does not return within the grace
// after the signal does not keep the process alive.
func TestInterruptWatchGrace(t *testing.T) {
	t.Parallel()

	f, hooks := newFakeInterruptHooks(20 * time.Millisecond)
	sigs := make(chan os.Signal, 1)
	w := startInterruptWatch(sigs, hooks)
	defer w.stop()
	sigs <- os.Interrupt
	if sig := within(t, f.died, "the process to end after the grace"); sig != os.Interrupt {
		t.Fatalf("ended by %v, want the interrupt", sig)
	}
}

// TestInterruptWatchSignalAsExportEnds: a signal that came before the watch
// ended ends the process, even when the watch had not handled it yet.
func TestInterruptWatchSignalAsExportEnds(t *testing.T) {
	t.Parallel()

	for range 20 {
		f, hooks := newFakeInterruptHooks(time.Hour)
		sigs := make(chan os.Signal, 1)
		w := startInterruptWatch(sigs, hooks)
		sigs <- os.Interrupt
		w.stop()
		select {
		case sig := <-f.died:
			if sig != os.Interrupt {
				t.Fatalf("ended by %v, want the interrupt", sig)
			}
		default:
			t.Fatal("the process was not ended by the signal")
		}
	}
}

// interruptHelperEnv makes the test binary act as the child process of
// TestInterruptEndsProcessBySignal: its value names the child's mode and the
// directory it keeps its temporary file in.
const interruptHelperEnv = "SLAPEX_TEST_INTERRUPT_HELPER"

// TestInterruptHelperProcess is the child process. In mode "stop", it stands
// in for an export that stops when its context is canceled: it keeps a
// temporary file until then, removes it, reports something through the
// printer and returns. In mode "stuck", the export does not stop. In mode
// "stderr-full", it stops as in "stop", but nothing reads stderr, and a write
// to it through the printer waits from before the signal.
func TestInterruptHelperProcess(t *testing.T) {
	mode, dir, ok := strings.Cut(os.Getenv(interruptHelperEnv), ":")
	if !ok {
		t.Skip("run by TestInterruptEndsProcessBySignal only")
	}
	stuck := make(chan struct{})
	printer := ui.NewPrinter(&markWriter{w: os.Stderr, mark: "stuck", seen: stuck}, false)
	ctx, endWatch := watchInterrupts(printer)
	tmp := filepath.Join(dir, "asset-123")
	if err := os.WriteFile(tmp, []byte("part"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitOther)
	}
	printer.StartPhase("Assets", "downloading assets ...")
	if mode == "stderr-full" {
		if err := fillStderr(); err != nil {
			fmt.Println("filling stderr:", err)
			os.Exit(exitOther)
		}
		go printer.Infof("stuck")
		select {
		case <-stuck:
		case <-time.After(10 * time.Second):
			fmt.Println("the printer did not write")
			os.Exit(exitOther)
		}
	}
	fmt.Println("ready")
	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
		os.Exit(exitOther)
	}
	if mode == "stuck" {
		time.Sleep(30 * time.Second)
	}
	os.Remove(tmp)
	printer.Warnf("asset failed (avatar): context canceled")
	printer.Errorf("slapex: context canceled")
	endWatch()
	fmt.Println("the process went on")
	os.Exit(exitOK)
}

// TestInterruptEndsProcessBySignal sends a child process standing in for an
// export (TestInterruptHelperProcess) SIGINT or SIGTERM. The export stops and
// removes its temporary file, nothing more is printed, and the process ends
// by the signal, as one that does not catch it would. A second signal ends a
// process whose export does not stop, well within the grace. A process whose
// stderr nobody reads, with a write to it waiting, stops all the same, well
// within the grace.
func TestInterruptEndsProcessBySignal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		sig     syscall.Signal
		mode    string
		signals int
	}{
		{"SIGINT", syscall.SIGINT, "stop", 1},
		{"SIGTERM", syscall.SIGTERM, "stop", 1},
		{"second SIGINT", syscall.SIGINT, "stuck", 2},
		{"SIGTERM with stderr full", syscall.SIGTERM, "stderr-full", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if signal.Ignored(tc.sig) {
				t.Skipf("%v is ignored here, and so in the child", tc.sig)
			}

			dir := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=^TestInterruptHelperProcess$")
			cmd.Env = append(os.Environ(), interruptHelperEnv+"="+tc.mode+":"+dir)
			var stderr syncBuffer
			cmd.Stderr = &stderr
			// In mode "stderr-full", stderr is a pipe read only once the
			// child has ended.
			var stderrPipe, stderrEnd *os.File
			if tc.mode == "stderr-full" {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				defer w.Close()
				stderrPipe, stderrEnd = r, w
				cmd.Stderr = w
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			if stderrEnd != nil {
				stderrEnd.Close()
			}
			lines := bufio.NewScanner(stdout)
			if !lines.Scan() || lines.Text() != "ready" {
				cmd.Process.Kill()
				cmd.Wait()
				t.Fatalf("the child did not start: %q\nstderr:\n%s", lines.Text(), stderr.String())
			}
			start := time.Now()
			for i := range tc.signals {
				if i > 0 {
					// The Go runtime merges a signal into the same one still
					// pending: the second goes once the first has arrived.
					time.Sleep(200 * time.Millisecond)
				}
				if err := cmd.Process.Signal(tc.sig); err != nil {
					t.Fatal(err)
				}
			}
			var rest []string
			for lines.Scan() {
				rest = append(rest, lines.Text())
			}
			err = cmd.Wait()
			took := time.Since(start)
			printed := stderr.String()
			if stderrPipe != nil {
				out, err := io.ReadAll(stderrPipe)
				if err != nil {
					t.Fatal(err)
				}
				// What filled the pipe is not the child's output.
				printed = strings.TrimRight(string(out), "x")
			}

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("the child ended with %v, want it ended by %v", err, tc.sig)
			}
			status, ok := exitErr.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != tc.sig {
				t.Fatalf("the child ended with %v, want it ended by %v\nstdout: %q\nstderr:\n%s", err, tc.sig, rest, printed)
			}
			if len(rest) != 0 {
				t.Errorf("the child went on after the signal: %q", rest)
			}
			if got, want := printed, "INFO: assets: downloading assets ...\n"; got != want {
				t.Errorf("the child printed %q, want only %q from before the signal", got, want)
			}
			if tc.mode != "stuck" {
				if _, err := os.Stat(filepath.Join(dir, "asset-123")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("the temporary file was left: %v", err)
				}
			}
			if tc.mode != "stop" && took >= interruptGrace {
				t.Errorf("the child ended after %v, want it at once", took)
			}
		})
	}
}

// syncBuffer is a bytes.Buffer that a child process's output can be copied
// into while the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// markWriter closes seen when a write that contains mark starts.
type markWriter struct {
	w    io.Writer
	mark string
	seen chan struct{}
}

func (m *markWriter) Write(b []byte) (int, error) {
	if bytes.Contains(b, []byte(m.mark)) {
		close(m.seen)
	}
	return m.w.Write(b)
}

// fillStderr writes to stderr, a pipe nobody reads, until the pipe is full:
// the next write to it waits.
func fillStderr() error {
	fd := syscall.Stderr
	if err := syscall.SetNonblock(fd, true); err != nil {
		return err
	}
	defer syscall.SetNonblock(fd, false)
	for _, size := range []int{4096, 1} {
		chunk := bytes.Repeat([]byte("x"), size)
		for {
			_, err := syscall.Write(fd, chunk)
			if errors.Is(err, syscall.EAGAIN) {
				break
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
