package cli

import (
	"strings"
	"testing"
)

func TestYAMLQuote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Getting Started", "\"Getting Started\""},
		{"unicode", "Über/Weiß", "\"Über/Weiß\""},
		{"empty", "", "\"\""},
		{"double_quotes", "She said \"hi\"", "\"She said \\\"hi\\\"\""},
		{"backslash", "C:\\foo\\bar", "\"C:\\\\foo\\\\bar\""},
		{"newline", "line1\nline2", "\"line1\\nline2\""},
		{"tab", "col1\tcol2", "\"col1\\tcol2\""},
		{"crlf", "a\r\nb", "\"a\\r\\nb\""},
		{"control_byte", "x\x01y", "\"x\\u0001y\""},
		{"del_byte", "x\x7fy", "\"x\\u007fy\""},
		{"emoji", "🚀 launch", "\"🚀 launch\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := yamlQuote(tt.in)
			if got != tt.want {
				t.Errorf("yamlQuote(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFrontmatter_Encode_FullStruct(t *testing.T) {
	fm := Frontmatter{
		Title:      "Some Page",
		PageID:     "12345",
		Version:    7,
		CreatedAt:  "2024-01-15T10:00:00Z",
		AuthorID:   "user-abc",
		ModifiedAt: "2024-02-20T14:30:00Z",
		ModifiedBy: "user-xyz",
		Source:     "https://example.atlassian.net/wiki/x",
	}
	want := "---\n" +
		"title: \"Some Page\"\n" +
		"page_id: \"12345\"\n" +
		"version: 7\n" +
		"created_at: \"2024-01-15T10:00:00Z\"\n" +
		"author_id: \"user-abc\"\n" +
		"modified_at: \"2024-02-20T14:30:00Z\"\n" +
		"modified_by: \"user-xyz\"\n" +
		"source: \"https://example.atlassian.net/wiki/x\"\n" +
		"---\n"
	got := fm.Encode()
	if got != want {
		t.Errorf("Encode() mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFrontmatter_Encode_OmitsEmpty(t *testing.T) {
	fm := Frontmatter{
		Title:      "Minimal",
		PageID:     "1",
		Version:    1,
		AuthorID:   "u",
		ModifiedBy: "u",
	}
	got := fm.Encode()
	if strings.Contains(got, "created_at") {
		t.Errorf("expected created_at to be omitted, got:\n%s", got)
	}
	if strings.Contains(got, "modified_at") {
		t.Errorf("expected modified_at to be omitted, got:\n%s", got)
	}
	if strings.Contains(got, "source") {
		t.Errorf("expected source to be omitted, got:\n%s", got)
	}
}

func TestFrontmatter_Encode_PreservesSpecialChars(t *testing.T) {
	fm := Frontmatter{
		Title:      "Title with \" quote and \\ backslash",
		PageID:     "1",
		Version:    1,
		AuthorID:   "u",
		ModifiedBy: "u",
	}
	got := fm.Encode()
	wantTitle := "title: \"Title with \\\" quote and \\\\ backslash\""
	if !strings.Contains(got, wantTitle) {
		t.Errorf("expected encoded title %q, got:\n%s", wantTitle, got)
	}
}
