// Package version reports the binary's version, git commit, and build time.
// Values are preferentially taken from linker-injected variables (set by the
// Makefile via `-ldflags -X`); when unset (e.g. for `go install` builds),
// Read falls back to runtime/debug.ReadBuildInfo and its embedded VCS metadata.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Linker-injected values. Kept lowercase to avoid exporting them; set via
// `-ldflags "-X github.com/pspoerri/confluence-reader/internal/version.ldX=..."`.
var (
	ldVersion string // e.g. "v1.2.3", "v1.2.3-5-gabcdef", or a short hash
	ldCommit  string // full or short commit hash, optionally suffixed with "-dirty"
	ldTime    string // RFC3339 build timestamp
)

// Info bundles the data shown by `confluence-reader version`.
type Info struct {
	Version  string // module version, "development build" when no git info is baked in
	Commit   string // git commit short hash, empty when not available
	Time     string // ISO-8601 commit timestamp, empty when not available
	Modified bool   // true when built from a dirty working tree
	Go       string // Go runtime version, e.g. "go1.26.1"
}

// devBuild is the version string shown when no git metadata is baked in.
const devBuild = "development build"

// Read extracts version information, preferring linker-injected values and
// falling back to the binary's embedded VCS metadata. When nothing is
// available, the version reads "development build".
func Read() Info {
	info := Info{
		Go: runtime.Version(),
	}

	if ldVersion != "" {
		info.Version = ldVersion
	}
	if ldCommit != "" {
		raw := strings.TrimSuffix(ldCommit, "-dirty")
		info.Commit = shortHash(raw)
		info.Modified = raw != ldCommit
	}
	if ldTime != "" {
		info.Time = ldTime
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		if info.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info.Version = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = shortHash(s.Value)
				}
			case "vcs.time":
				if info.Time == "" {
					info.Time = s.Value
				}
			case "vcs.modified":
				if ldCommit == "" {
					info.Modified = s.Value == "true"
				}
			}
		}
	}

	if info.Version == "" {
		info.Version = devBuild
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
