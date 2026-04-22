// Package version reports the binary's version, git commit, and build time.
// All values come from runtime/debug.ReadBuildInfo, which Go populates from
// the module cache (when installed via `go install`) and the embedded VCS
// metadata (when built from a git checkout). No -ldflags trickery required.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Info bundles the data shown by `confluence-reader version`.
type Info struct {
	Version  string // module version, "(devel)" when unset
	Commit   string // git commit short hash, empty when not available
	Time     string // ISO-8601 commit timestamp, empty when not available
	Modified bool   // true when built from a dirty working tree
	Go       string // Go runtime version, e.g. "go1.26.1"
}

// Read extracts version information from the binary's embedded build info.
func Read() Info {
	info := Info{
		Version: "(devel)",
		Go:      runtime.Version(),
	}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = bi.Main.Version
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = shortHash(s.Value)
		case "vcs.time":
			info.Time = s.Value
		case "vcs.modified":
			info.Modified = s.Value == "true"
		}
	}
	return info
}

// String returns a one-line summary suitable for `--version` output.
func (i Info) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "confluence-reader %s", i.Version)
	if i.Commit != "" {
		fmt.Fprintf(&b, " (%s", i.Commit)
		if i.Modified {
			b.WriteString("-dirty")
		}
		b.WriteString(")")
	}
	return b.String()
}

// Detailed returns a multi-line block with version, commit, build time, and
// Go runtime — appropriate for the `version` subcommand.
func (i Info) Detailed() string {
	var b strings.Builder
	fmt.Fprintf(&b, "confluence-reader %s\n", i.Version)
	if i.Commit != "" {
		commit := i.Commit
		if i.Modified {
			commit += "-dirty"
		}
		fmt.Fprintf(&b, "  commit:  %s\n", commit)
	}
	if i.Time != "" {
		fmt.Fprintf(&b, "  built:   %s\n", i.Time)
	}
	fmt.Fprintf(&b, "  go:      %s\n", i.Go)
	return b.String()
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
