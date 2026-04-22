package version

import (
	"strings"
	"testing"
)

func TestInfo_String_DevelOnly(t *testing.T) {
	got := Info{Version: devBuild}.String()
	if got != "confluence-reader development build" {
		t.Errorf("got %q", got)
	}
}

func TestInfo_String_WithCommit(t *testing.T) {
	got := Info{Version: "v1.2.3", Commit: "abc1234def56"}.String()
	if got != "confluence-reader v1.2.3 (abc1234def56)" {
		t.Errorf("got %q", got)
	}
}

func TestInfo_String_WithDirtyCommit(t *testing.T) {
	got := Info{Version: "v1.2.3", Commit: "abc1234def56", Modified: true}.String()
	if got != "confluence-reader v1.2.3 (abc1234def56-dirty)" {
		t.Errorf("got %q", got)
	}
}

func TestInfo_Detailed_OmitsBlankFields(t *testing.T) {
	got := Info{Version: devBuild, Go: "go1.26.1"}.Detailed()
	if strings.Contains(got, "commit:") {
		t.Errorf("expected no commit line, got:\n%s", got)
	}
	if strings.Contains(got, "built:") {
		t.Errorf("expected no built line, got:\n%s", got)
	}
	if !strings.Contains(got, "go:      go1.26.1") {
		t.Errorf("expected Go line, got:\n%s", got)
	}
}

func TestInfo_Detailed_FullStruct(t *testing.T) {
	i := Info{
		Version:  "v1.0.0",
		Commit:   "abc1234def56",
		Time:     "2026-04-22T12:00:00Z",
		Modified: false,
		Go:       "go1.26.1",
	}
	got := i.Detailed()
	for _, want := range []string{
		"confluence-reader v1.0.0",
		"commit:  abc1234def56\n",
		"built:   2026-04-22T12:00:00Z\n",
		"go:      go1.26.1\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}
}

func TestShortHash(t *testing.T) {
	tests := map[string]string{
		"":                 "",
		"abc":              "abc",
		"abcdef0123456789": "abcdef012345",
		"0e5b2469fd1234567890abcdef0123456789abcd": "0e5b2469fd12",
	}
	for in, want := range tests {
		if got := shortHash(in); got != want {
			t.Errorf("shortHash(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRead_ReturnsSomething is a smoke test: when run from a real git
// checkout, Read must populate at least the Go version.
func TestRead_ReturnsSomething(t *testing.T) {
	info := Read()
	if info.Go == "" {
		t.Error("Go version should always be populated")
	}
	// Version should be a semver, "development build", or some other non-empty token.
	if info.Version == "" {
		t.Error("Version should never be empty")
	}
}
