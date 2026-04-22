package cli

import (
	"strings"
	"testing"

	"github.com/pspoerri/confluence-reader/internal/api"
)

func TestPdfFilename(t *testing.T) {
	tests := map[string]string{
		"diagram.drawio":               "diagram.pdf",
		"page-slug-diagram-fid.drawio": "page-slug-diagram-fid.pdf",
		"no-extension":                 "no-extension.pdf",
		"DIAGRAM.DRAWIO":               "DIAGRAM.DRAWIO.pdf", // case-sensitive: only ".drawio" stripped
	}
	for in, want := range tests {
		got := pdfFilename(in)
		if got != want {
			t.Errorf("pdfFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsDrawio(t *testing.T) {
	tests := []struct {
		att  api.Attachment
		want bool
	}{
		{api.Attachment{Title: "x.drawio"}, true},
		{api.Attachment{Title: "x.DRAWIO"}, true},
		{api.Attachment{Title: "x.png"}, false},
		{api.Attachment{Title: "x.png", MediaType: "application/vnd.jgraph.mxfile"}, false},
		{api.Attachment{Title: "x.bin", MediaType: "application/x-drawio"}, true},
	}
	for _, tt := range tests {
		got := isDrawio(tt.att)
		if got != tt.want {
			t.Errorf("isDrawio(%+v) = %v, want %v", tt.att, got, tt.want)
		}
	}
}

func TestReplaceDrawioPlaceholder_BothForms(t *testing.T) {
	md := "Look here: *(diagram: MyDiagram)* and there: *(diagram: Other.drawio)*."
	got := replaceDrawioPlaceholder(md, "MyDiagram.drawio", "page-mydiagram-1.drawio", "page-mydiagram-1.pdf")
	got = replaceDrawioPlaceholder(got, "Other.drawio", "page-other-2.drawio", "page-other-2.pdf")

	if strings.Contains(got, "*(diagram:") {
		t.Errorf("expected all placeholders to be replaced, got:\n%s", got)
	}
	if !strings.Contains(got, "[MyDiagram (PDF)](page-mydiagram-1.pdf)") {
		t.Errorf("expected MyDiagram PDF link, got:\n%s", got)
	}
	if !strings.Contains(got, "[Other (PDF)](page-other-2.pdf)") {
		t.Errorf("expected Other PDF link, got:\n%s", got)
	}
	if !strings.Contains(got, "[source](page-mydiagram-1.drawio)") {
		t.Errorf("expected source link for MyDiagram, got:\n%s", got)
	}
}

func TestReplaceDrawioPlaceholder_NoMatchLeavesMarkdown(t *testing.T) {
	md := "no placeholder here"
	got := replaceDrawioPlaceholder(md, "x.drawio", "x.drawio", "x.pdf")
	if got != md {
		t.Errorf("expected markdown unchanged, got %q", got)
	}
}
