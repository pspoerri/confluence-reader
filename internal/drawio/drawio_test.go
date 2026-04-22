package drawio

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// resetCLICache clears the cached CLI lookup result so a test can re-trigger
// FindCLI under a controlled environment, then restores it on cleanup.
func resetCLICache(t *testing.T) {
	t.Helper()
	cliMu.Lock()
	prevCached, prevPath := cliCached, cliPath
	cliCached, cliPath = false, ""
	cliMu.Unlock()
	t.Cleanup(func() {
		cliMu.Lock()
		cliCached, cliPath = prevCached, prevPath
		cliMu.Unlock()
	})
}

// preloadCLICache pre-arms the cache so RenderToPDF sees the supplied path
// without performing a real lookup. Restored on test cleanup.
func preloadCLICache(t *testing.T, path string) {
	t.Helper()
	cliMu.Lock()
	prevCached, prevPath := cliCached, cliPath
	cliCached, cliPath = true, path
	cliMu.Unlock()
	t.Cleanup(func() {
		cliMu.Lock()
		cliCached, cliPath = prevCached, prevPath
		cliMu.Unlock()
	})
}

func TestRenderToPDF_NoCLIReturnsSentinel(t *testing.T) {
	resetCLICache(t)
	t.Setenv("PATH", "")
	// On macOS the locator also checks the app bundle path; if a real
	// install is present this assertion doesn't apply.
	if _, err := os.Stat("/Applications/draw.io.app/Contents/MacOS/draw.io"); err == nil {
		t.Skip("real drawio install present; skip the no-CLI assertion")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "in.drawio")
	if err := os.WriteFile(src, []byte("<mxfile/>"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	err := RenderToPDF(src, filepath.Join(dir, "out.pdf"))
	if !errors.Is(err, ErrCLINotFound) {
		t.Fatalf("expected ErrCLINotFound, got %v", err)
	}
}

func TestRenderToPDF_SkipsExistingOutput(t *testing.T) {
	preloadCLICache(t, "/never/invoked")

	dir := t.TempDir()
	src := filepath.Join(dir, "in.drawio")
	out := filepath.Join(dir, "out.pdf")
	if err := os.WriteFile(src, []byte("<mxfile/>"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(out, []byte("%PDF-1.4 ..."), 0o644); err != nil {
		t.Fatalf("write existing out: %v", err)
	}

	if err := RenderToPDF(src, out); err != nil {
		t.Fatalf("expected skip-on-existing to succeed, got %v", err)
	}
}

func TestAvailable_MatchesFindCLI(t *testing.T) {
	if Available() != (FindCLI() != "") {
		t.Errorf("Available()=%v but FindCLI()=%q", Available(), FindCLI())
	}
}
