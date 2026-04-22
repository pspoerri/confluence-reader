// Package drawio shells out to the draw.io desktop CLI to render .drawio
// diagrams to PNG. The CLI ships with the draw.io desktop application
// (https://www.drawio.com/) on Linux, macOS, and Windows. When the CLI is
// not installed, RenderToPNG returns ErrCLINotFound so callers can skip the
// step gracefully and fall back to a source link.
package drawio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ErrCLINotFound is returned when no usable draw.io CLI is on PATH or in a
// known platform install location.
var ErrCLINotFound = errors.New("drawio: CLI not found")

const renderTimeout = 120 * time.Second

var (
	cliMu     sync.RWMutex
	cliCached bool
	cliPath   string // empty when not available

	// renderMu serializes drawio CLI invocations. The Electron-based desktop
	// CLI uses a single-instance lock — a second concurrent process defers
	// to the first and exits early, so two parallel renders can silently
	// produce no output. Holding this mutex around the exec keeps the CLI
	// happy without blocking the rest of the mirror pipeline.
	renderMu sync.Mutex
)

// FindCLI returns the absolute path of the draw.io CLI, or empty string if
// not installed. The result is cached after the first call so repeated mirror
// runs don't re-stat the filesystem.
func FindCLI() string {
	cliMu.RLock()
	if cliCached {
		p := cliPath
		cliMu.RUnlock()
		return p
	}
	cliMu.RUnlock()

	cliMu.Lock()
	defer cliMu.Unlock()
	if cliCached {
		return cliPath
	}
	cliPath = locateCLI()
	cliCached = true
	return cliPath
}

// locateCLI does the actual filesystem search. Separate from FindCLI so the
// caching layer is easy to reason about (and reset in tests).
func locateCLI() string {
	for _, name := range []string{"drawio", "draw.io"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if runtime.GOOS == "darwin" {
		const macPath = "/Applications/draw.io.app/Contents/MacOS/draw.io"
		if _, err := os.Stat(macPath); err == nil {
			return macPath
		}
	}
	return ""
}

// Available reports whether the draw.io CLI is installed and usable.
func Available() bool { return FindCLI() != "" }

// RenderToPDF converts drawioPath to a cropped PDF at outputPath using the
// draw.io CLI. Returns ErrCLINotFound when the CLI isn't installed. If
// outputPath already exists with non-zero size, returns nil without
// re-rendering.
//
// PDF + --crop is used because PDF export is the most reliable drawio CLI
// output format — PNG export has been observed to silently exit zero
// without producing a file on some inputs.
//
// Argument order matters: --no-sandbox MUST appear after the input path or
// drawio's argv parser misinterprets the positional args and fails with
// "Error: input file/directory not found" — see jgraph/drawio-desktop#249.
// This was the actual cause of the path-related render failures we saw on
// nested Confluence page directories; the special characters in the path
// were a red herring.
func RenderToPDF(drawioPath, outputPath string) error {
	cli := FindCLI()
	if cli == "" {
		return ErrCLINotFound
	}

	if info, err := os.Stat(outputPath); err == nil && info.Size() > 0 {
		return nil
	}

	if _, err := os.Stat(drawioPath); err != nil {
		return fmt.Errorf("drawio: source missing: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("drawio: create output dir: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), renderTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cli,
		"--export",
		"--format", "pdf",
		"--crop",
		"--output", outputPath,
		drawioPath,
		// Trailing flags — see comment above for why these go last.
		"--no-sandbox",
		"--disable-gpu",
	)

	// Serialize CLI invocations to avoid Electron's single-instance lock
	// turning a concurrent render into a silent no-op.
	renderMu.Lock()
	out, err := cmd.CombinedOutput()
	renderMu.Unlock()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("drawio: render timed out after %s", renderTimeout)
		}
		return fmt.Errorf("drawio: render failed: %w (CLI said: %s)", err, trimCLIOutput(out))
	}

	if info, err := os.Stat(outputPath); err != nil || info.Size() == 0 {
		return fmt.Errorf("drawio: render produced no output for %s (CLI said: %s)", filepath.Base(drawioPath), trimCLIOutput(out))
	}
	return nil
}

// trimCLIOutput collapses the CLI's combined output to a compact, single-line
// excerpt suitable for an error message.
func trimCLIOutput(b []byte) string {
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " | ")
	if len(s) > 500 {
		s = s[:500] + "..."
	}
	if s == "" {
		s = "(no output)"
	}
	return s
}
