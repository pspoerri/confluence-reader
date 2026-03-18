package ui

import (
	"bytes"
	"testing"
)

func TestErrorf(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf)
	w.Errorf("something went %s", "wrong")
	if got := buf.String(); got != "error: something went wrong\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestWarnf(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf)
	w.Warnf("disk almost full")
	if got := buf.String(); got != "warning: disk almost full\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestSuccessf(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf)
	w.Successf("done in %dms", 42)
	if got := buf.String(); got != "done in 42ms\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestInfof(t *testing.T) {
	var buf bytes.Buffer
	w := New(&buf)
	w.Infof("processing %d items", 5)
	if got := buf.String(); got != "processing 5 items\n" {
		t.Errorf("unexpected output: %q", got)
	}
}
