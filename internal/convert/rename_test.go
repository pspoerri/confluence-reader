package convert

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Getting Started", "getting-started"},
		{"Hello World!", "hello-world"},
		{"API Reference (v2)", "api-reference-v2"},
		{"  spaces  ", "spaces"},
		{"under_score", "under-score"},
		{"MixedCase123", "mixedcase123"},
		{"", "file"},
		{"---", "file"},
		{"file.name.with.dots", "file-name-with-dots"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRenameAttachment(t *testing.T) {
	tests := []struct {
		pageTitle string
		filename  string
		fileID    string
		want      string
	}{
		{"Getting Started", "screenshot.png", "att123", "getting-started-screenshot-att123.png"},
		{"API Docs", "diagram.PNG", "att456", "api-docs-diagram-att456.png"},
		{"My Page", "my-page-image.jpg", "att789", "my-page-image-att789.jpg"}, // stem starts with page slug
		{"Report", "Untitled.pdf", "att001", "report-untitled-att001.pdf"},
	}
	for _, tt := range tests {
		got := RenameAttachment(tt.pageTitle, tt.filename, tt.fileID)
		if got != tt.want {
			t.Errorf("RenameAttachment(%q, %q, %q) = %q, want %q",
				tt.pageTitle, tt.filename, tt.fileID, got, tt.want)
		}
	}
}
