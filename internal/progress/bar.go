package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Bar renders a terminal progress bar to stderr.
type Bar struct {
	mu      sync.Mutex
	w       io.Writer
	label   string
	total   int
	current int
	width   int
	done    bool
}

// New creates a progress bar that writes to stderr.
func New(label string, total int) *Bar {
	b := &Bar{
		w:     os.Stderr,
		label: label,
		total: total,
		width: 30,
	}
	b.render()
	return b
}

// newBar creates a progress bar writing to w (for testing).
func newBar(w io.Writer, label string, total int) *Bar {
	b := &Bar{
		w:     w,
		label: label,
		total: total,
		width: 30,
	}
	b.render()
	return b
}

// Increment advances the bar by one.
func (b *Bar) Increment() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return
	}
	b.current++
	b.render()
}

// Log prints a message above the progress bar without disrupting it.
func (b *Bar) Log(format string, args ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	fmt.Fprintf(b.w, "\r\033[K")
	fmt.Fprintf(b.w, format+"\n", args...)
	b.render()
}

// Finish completes the bar and moves to a new line.
func (b *Bar) Finish() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current = b.total
	b.done = true
	b.render()
	fmt.Fprintln(b.w)
}

func (b *Bar) render() {
	if b.total <= 0 {
		return
	}
	filled := (b.current * b.width) / b.total
	if filled > b.width {
		filled = b.width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", b.width-filled)
	fmt.Fprintf(b.w, "\r\033[K%s  [%s]  %d/%d", b.label, bar, b.current, b.total)
}
