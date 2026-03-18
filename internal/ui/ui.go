package ui

import (
	"fmt"
	"io"
	"os"
)

// Writer provides standardized output formatting with optional color support.
type Writer struct {
	w     io.Writer
	color bool
}

// Stderr creates a Writer that writes to os.Stderr with color auto-detected.
func Stderr() *Writer {
	return &Writer{w: os.Stderr, color: isTTY(os.Stderr)}
}

// New creates a Writer for the given destination with no color.
// Useful for testing.
func New(w io.Writer) *Writer {
	return &Writer{w: w, color: false}
}

// Errorf prints an error message: "error: message\n".
// In color mode the "error:" prefix is red.
func (u *Writer) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if u.color {
		fmt.Fprintf(u.w, "\033[31merror:\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(u.w, "error: %s\n", msg)
	}
}

// Warnf prints a warning message: "warning: message\n".
// In color mode the "warning:" prefix is yellow.
func (u *Writer) Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if u.color {
		fmt.Fprintf(u.w, "\033[33mwarning:\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(u.w, "warning: %s\n", msg)
	}
}

// Successf prints a success message with a checkmark prefix in color mode.
func (u *Writer) Successf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if u.color {
		fmt.Fprintf(u.w, "\033[32m✔\033[0m %s\n", msg)
	} else {
		fmt.Fprintf(u.w, "%s\n", msg)
	}
}

// Infof prints an informational message.
func (u *Writer) Infof(format string, args ...any) {
	fmt.Fprintf(u.w, format+"\n", args...)
}

// isTTY reports whether w is a terminal.
func isTTY(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
