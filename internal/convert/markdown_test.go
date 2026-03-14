package convert

import (
	"strings"
	"testing"

	"github.com/pspoerri/confluence-reader/internal/api"
)

func TestToMarkdown_Headings(t *testing.T) {
	input := `<h1>Title</h1><h2>Subtitle</h2><h3>Section</h3>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "# Title") {
		t.Errorf("expected h1 conversion, got: %s", result)
	}
	if !strings.Contains(result, "## Subtitle") {
		t.Errorf("expected h2 conversion, got: %s", result)
	}
	if !strings.Contains(result, "### Section") {
		t.Errorf("expected h3 conversion, got: %s", result)
	}
}

func TestToMarkdown_Bold(t *testing.T) {
	input := `<p>This is <strong>bold</strong> text</p>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "**bold**") {
		t.Errorf("expected bold conversion, got: %s", result)
	}
}

func TestToMarkdown_Italic(t *testing.T) {
	input := `<p>This is <em>italic</em> text</p>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "*italic*") {
		t.Errorf("expected italic conversion, got: %s", result)
	}
}

func TestToMarkdown_InlineCode(t *testing.T) {
	input := `<p>Use <code>fmt.Println</code> here</p>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "`fmt.Println`") {
		t.Errorf("expected inline code conversion, got: %s", result)
	}
}

func TestToMarkdown_Links(t *testing.T) {
	input := `<p>Visit <a href="https://example.com">Example</a></p>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "[Example](https://example.com)") {
		t.Errorf("expected link conversion, got: %s", result)
	}
}

func TestToMarkdown_ACImage(t *testing.T) {
	input := `<ac:image><ri:attachment ri:filename="diagram.png" /></ac:image>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "![diagram.png](attachment:diagram.png)") {
		t.Errorf("expected image conversion, got: %s", result)
	}
}

func TestToMarkdown_ACLink(t *testing.T) {
	input := `<ac:link><ri:attachment ri:filename="report.pdf" /><ac:plain-text-link-body><![CDATA[Download Report]]></ac:plain-text-link-body></ac:link>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "[Download Report](attachment:report.pdf)") {
		t.Errorf("expected ac:link conversion, got: %s", result)
	}
}

func TestToMarkdown_AttachmentList(t *testing.T) {
	attachments := []api.Attachment{
		{Title: "file.pdf", MediaType: "application/pdf", FileSize: 1024},
		{Title: "image.png", MediaType: "image/png", FileSize: 2048},
	}
	result := ToMarkdown("<p>Hello</p>", attachments)

	if !strings.Contains(result, "## Attachments") {
		t.Errorf("expected attachment section, got: %s", result)
	}
	if !strings.Contains(result, "[file.pdf](attachment:file.pdf)") {
		t.Errorf("expected file.pdf reference, got: %s", result)
	}
	if !strings.Contains(result, "[image.png](attachment:image.png)") {
		t.Errorf("expected image.png reference, got: %s", result)
	}
}

func TestToMarkdown_HTMLEntities(t *testing.T) {
	input := `<p>A &amp; B &lt; C &gt; D &quot;E&quot;</p>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, `A & B < C > D "E"`) {
		t.Errorf("expected entity decoding, got: %s", result)
	}
}

func TestToMarkdown_NamedHTMLEntities(t *testing.T) {
	input := `<h3>Suggested &ldquo;bridging&rdquo; model &mdash; hybrid stage-gate &amp; agile</h3>`
	result := ToMarkdown(input, nil)

	expected := `### Suggested \x{201c}bridging\x{201d} model \x{2014} hybrid stage-gate & agile`
	if !strings.Contains(result, "Suggested \u201cbridging\u201d model \u2014 hybrid stage-gate & agile") {
		t.Errorf("expected named entity decoding, got: %s (expected %s)", result, expected)
	}
}

func TestToMarkdown_Table(t *testing.T) {
	input := `<table><tr><th>Name</th><th>Value</th></tr><tr><td>foo</td><td>bar</td></tr><tr><td>baz</td><td>qux</td></tr></table>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "| Name | Value |") {
		t.Errorf("expected header row, got: %s", result)
	}
	if !strings.Contains(result, "| --- | --- |") {
		t.Errorf("expected separator row, got: %s", result)
	}
	if !strings.Contains(result, "| foo | bar |") {
		t.Errorf("expected data row, got: %s", result)
	}
}

func TestToMarkdown_TableNoHeader(t *testing.T) {
	input := `<table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "| a | b |") {
		t.Errorf("expected first row, got: %s", result)
	}
	if !strings.Contains(result, "| --- | --- |") {
		t.Errorf("expected separator after first row, got: %s", result)
	}
}

func TestToMarkdown_CodeBlockWithLanguage(t *testing.T) {
	input := `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">python</ac:parameter><ac:plain-text-body><![CDATA[def hello():
    print("world")]]></ac:plain-text-body></ac:structured-macro>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "```python") {
		t.Errorf("expected language in code fence, got: %s", result)
	}
	if !strings.Contains(result, `print("world")`) {
		t.Errorf("expected code content, got: %s", result)
	}
}

func TestToMarkdown_CodeBlockPreservesEntities(t *testing.T) {
	input := `<ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[if (a < b && c > d) {
    x &= y;
}]]></ac:plain-text-body></ac:structured-macro>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "a < b && c > d") {
		t.Errorf("expected raw operators preserved in code block, got: %s", result)
	}
}

func TestToMarkdown_InlineCodePreservesContent(t *testing.T) {
	input := `<p>Use <code>&lt;div&gt;</code> for containers</p>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "`<div>`") {
		t.Errorf("expected decoded entities in inline code, got: %s", result)
	}
}

func TestToMarkdown_PreBlock(t *testing.T) {
	input := `<pre>line 1
line 2</pre>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "```\nline 1\nline 2\n```") {
		t.Errorf("expected pre block conversion, got: %s", result)
	}
}

func TestToMarkdown_InfoPanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="info"><ac:rich-text-body><p>This is important information.</p></ac:rich-text-body></ac:structured-macro>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "> **Info:**") {
		t.Errorf("expected info callout, got: %s", result)
	}
	if !strings.Contains(result, "This is important information.") {
		t.Errorf("expected info content, got: %s", result)
	}
}

func TestToMarkdown_WarningPanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="warning"><ac:rich-text-body><p>Be careful!</p></ac:rich-text-body></ac:structured-macro>`
	result := ToMarkdown(input, nil)

	if !strings.Contains(result, "> **Warning:**") {
		t.Errorf("expected warning callout, got: %s", result)
	}
}

func TestToMarkdown_EmptyBody(t *testing.T) {
	result := ToMarkdown("", nil)
	if result != "" {
		t.Errorf("expected empty string for empty input, got: %q", result)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
