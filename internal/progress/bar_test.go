package progress

import (
	"bytes"
	"strings"
	"testing"
)

func TestBar_Render(t *testing.T) {
	var buf bytes.Buffer
	b := newBar(&buf, "Test", 5)

	for range 5 {
		b.Increment()
	}

	out := buf.String()
	if !strings.Contains(out, "5/5") {
		t.Errorf("expected bar to reach 5/5, got: %s", out)
	}
	if !strings.Contains(out, "Test") {
		t.Errorf("expected label 'Test' in output, got: %s", out)
	}
}

func TestBar_Finish(t *testing.T) {
	var buf bytes.Buffer
	b := newBar(&buf, "Done", 3)
	b.Increment()
	b.Finish()

	out := buf.String()
	if !strings.Contains(out, "3/3") {
		t.Errorf("expected Finish to set 3/3, got: %s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("expected trailing newline after Finish")
	}

	// Subsequent Increment should be a no-op.
	lenBefore := buf.Len()
	b.Increment()
	if buf.Len() != lenBefore {
		t.Error("expected no output after Finish + Increment")
	}
}

func TestBar_Log(t *testing.T) {
	var buf bytes.Buffer
	b := newBar(&buf, "Log", 10)
	b.Increment()
	b.Log("warning: %s", "test message")

	out := buf.String()
	if !strings.Contains(out, "warning: test message") {
		t.Errorf("expected log message in output, got: %s", out)
	}
}

func TestBar_ZeroTotal(t *testing.T) {
	var buf bytes.Buffer
	b := newBar(&buf, "Empty", 0)
	b.Increment() // should not panic
	b.Finish()

	// No bar rendered for zero total.
	if strings.Contains(buf.String(), "[") {
		t.Error("expected no bar rendering for zero total")
	}
}
