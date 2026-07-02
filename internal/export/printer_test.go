package export

import (
	"bytes"
	"strings"

	"github.com/kiyohara/slapex/internal/ui"
)

// testPrinter returns a plain-mode ui.Printer whose complete output lines are
// passed to emit. Tests use it where they previously captured logf lines; the
// emit closure is responsible for any locking.
func testPrinter(emit func(string)) *ui.Printer {
	return ui.NewPrinter(&lineWriter{emit: emit}, false)
}

type lineWriter struct {
	emit func(string)
	buf  bytes.Buffer
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.WriteString(line)
			break
		}
		w.emit(strings.TrimSuffix(line, "\n"))
	}
	return len(p), nil
}
