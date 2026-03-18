package convert

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pspoerri/confluence-reader/internal/api"
)

// toMarkdown is a test helper that calls ToMarkdown and returns just the markdown string.
func toMarkdown(input string, attachments []api.Attachment) string {
	return ToMarkdown(input, attachments).Markdown
}

func TestToMarkdown_Headings(t *testing.T) {
	input := `<h1>Title</h1><h2>Subtitle</h2><h3>Section</h3>`
	result := toMarkdown(input, nil)

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
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "**bold**") {
		t.Errorf("expected bold conversion, got: %s", result)
	}
}

func TestToMarkdown_Italic(t *testing.T) {
	input := `<p>This is <em>italic</em> text</p>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "*italic*") {
		t.Errorf("expected italic conversion, got: %s", result)
	}
}

func TestToMarkdown_InlineCode(t *testing.T) {
	input := `<p>Use <code>fmt.Println</code> here</p>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "`fmt.Println`") {
		t.Errorf("expected inline code conversion, got: %s", result)
	}
}

func TestToMarkdown_Links(t *testing.T) {
	input := `<p>Visit <a href="https://example.com">Example</a></p>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "[Example](https://example.com)") {
		t.Errorf("expected link conversion, got: %s", result)
	}
}

func TestToMarkdown_ACImage(t *testing.T) {
	input := `<ac:image><ri:attachment ri:filename="diagram.png" /></ac:image>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "![diagram.png](attachment:diagram.png)") {
		t.Errorf("expected image conversion, got: %s", result)
	}
}

func TestToMarkdown_ACLink(t *testing.T) {
	input := `<ac:link><ri:attachment ri:filename="report.pdf" /><ac:plain-text-link-body><![CDATA[Download Report]]></ac:plain-text-link-body></ac:link>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "[Download Report](attachment:report.pdf)") {
		t.Errorf("expected ac:link conversion, got: %s", result)
	}
}

func TestToMarkdown_AttachmentList(t *testing.T) {
	attachments := []api.Attachment{
		{Title: "file.pdf", MediaType: "application/pdf", FileSize: 1024},
		{Title: "image.png", MediaType: "image/png", FileSize: 2048},
	}
	result := toMarkdown("<p>Hello</p>", attachments)

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
	result := toMarkdown(input, nil)

	if !strings.Contains(result, `A & B < C > D "E"`) {
		t.Errorf("expected entity decoding, got: %s", result)
	}
}

func TestToMarkdown_NamedHTMLEntities(t *testing.T) {
	input := `<h3>Suggested &ldquo;bridging&rdquo; model &mdash; hybrid stage-gate &amp; agile</h3>`
	result := toMarkdown(input, nil)

	expected := `### Suggested \x{201c}bridging\x{201d} model \x{2014} hybrid stage-gate & agile`
	if !strings.Contains(result, "Suggested \u201cbridging\u201d model \u2014 hybrid stage-gate & agile") {
		t.Errorf("expected named entity decoding, got: %s (expected %s)", result, expected)
	}
}

func TestToMarkdown_Table(t *testing.T) {
	input := `<table><tr><th>Name</th><th>Value</th></tr><tr><td>foo</td><td>bar</td></tr><tr><td>baz</td><td>qux</td></tr></table>`
	result := toMarkdown(input, nil)

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
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "| a | b |") {
		t.Errorf("expected first row, got: %s", result)
	}
	if !strings.Contains(result, "| --- | --- |") {
		t.Errorf("expected separator after first row, got: %s", result)
	}
}

func TestToMarkdown_TableMixedThTd(t *testing.T) {
	input := `<table><tr><th>H1</th><td>D1</td></tr><tr><td>A</td><td>B</td></tr></table>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "| H1 | D1 |") {
		t.Errorf("expected mixed th/td row, got: %s", result)
	}
	if !strings.Contains(result, "| --- | --- |") {
		t.Errorf("expected separator, got: %s", result)
	}
	if !strings.Contains(result, "| A | B |") {
		t.Errorf("expected data row, got: %s", result)
	}
}

func TestToMarkdown_TableWithTheadTbody(t *testing.T) {
	input := `<table><thead><tr><th>Name</th><th>Value</th></tr></thead><tbody><tr><td>foo</td><td>bar</td></tr></tbody></table>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "| Name | Value |") {
		t.Errorf("expected header row, got: %s", result)
	}
	if !strings.Contains(result, "| --- | --- |") {
		t.Errorf("expected separator, got: %s", result)
	}
	if !strings.Contains(result, "| foo | bar |") {
		t.Errorf("expected data row, got: %s", result)
	}
}

func TestToMarkdown_TableWithSelfClosingEmoticon(t *testing.T) {
	input := `<table><tr><td><ac:emoticon ac:name="tick" /></td><td>Approved</td></tr><tr><td><ac:emoticon ac:name="cross" /></td><td>Rejected</td></tr></table>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "| (check) | Approved |") {
		t.Errorf("expected self-closing emoticon in first cell, got: %s", result)
	}
	if !strings.Contains(result, "| (x) | Rejected |") {
		t.Errorf("expected self-closing emoticon in second row, got: %s", result)
	}
}

func TestToMarkdown_TableWithSelfClosingImage(t *testing.T) {
	input := `<table><tr><th>Preview</th><th>Name</th></tr><tr><td><ac:image><ri:attachment ri:filename="icon.png" /></ac:image></td><td>Icon</td></tr></table>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "| Preview | Name |") {
		t.Errorf("expected header row, got: %s", result)
	}
	if !strings.Contains(result, "![icon.png](attachment:icon.png)") {
		t.Errorf("expected image in cell, got: %s", result)
	}
	if !strings.Contains(result, "| Icon |") {
		t.Errorf("expected Icon in separate cell (self-closing ri:attachment must not swallow siblings), got: %s", result)
	}
}

func TestToMarkdown_TableWithSelfClosingStatus(t *testing.T) {
	input := `<table><tr><th>Task</th><th>Status</th></tr><tr><td>Deploy</td><td><ac:structured-macro ac:name="status"><ac:parameter ac:name="title">DONE</ac:parameter></ac:structured-macro></td></tr></table>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "| Task | Status |") {
		t.Errorf("expected header row, got: %s", result)
	}
	if !strings.Contains(result, "`DONE`") {
		t.Errorf("expected status lozenge in table, got: %s", result)
	}
	if !strings.Contains(result, "| Deploy |") {
		t.Errorf("expected Deploy cell, got: %s", result)
	}
}

func TestToMarkdown_CodeBlockWithLanguage(t *testing.T) {
	input := `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">python</ac:parameter><ac:plain-text-body><![CDATA[def hello():
    print("world")]]></ac:plain-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

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
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "a < b && c > d") {
		t.Errorf("expected raw operators preserved in code block, got: %s", result)
	}
}

func TestToMarkdown_InlineCodePreservesContent(t *testing.T) {
	input := `<p>Use <code>&lt;div&gt;</code> for containers</p>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "`<div>`") {
		t.Errorf("expected decoded entities in inline code, got: %s", result)
	}
}

func TestToMarkdown_PreBlock(t *testing.T) {
	input := `<pre>line 1
line 2</pre>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "```\nline 1\nline 2\n```") {
		t.Errorf("expected pre block conversion, got: %s", result)
	}
}

func TestToMarkdown_CodeBlockPreservesIndentation(t *testing.T) {
	input := `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[func main() {
	if a < b && c > d {
		fmt.Println("hello & world")
	}
}]]></ac:plain-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "```go\n") {
		t.Errorf("expected go code fence, got: %s", result)
	}
	if !strings.Contains(result, "\t\tfmt.Println") {
		t.Errorf("expected preserved indentation, got: %s", result)
	}
	if !strings.Contains(result, `"hello & world"`) {
		t.Errorf("expected literal ampersand in CDATA code, got: %s", result)
	}
}

func TestToMarkdown_CodeBlockInList(t *testing.T) {
	input := `<ul><li>Example:<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">python</ac:parameter><ac:plain-text-body><![CDATA[print("hi")]]></ac:plain-text-body></ac:structured-macro></li><li>Next item</li></ul>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "- Example:") {
		t.Errorf("expected list item before code block, got: %s", result)
	}
	if !strings.Contains(result, "```python") {
		t.Errorf("expected python code fence, got: %s", result)
	}
	if !strings.Contains(result, `print("hi")`) {
		t.Errorf("expected code content, got: %s", result)
	}
	if !strings.Contains(result, "- Next item") {
		t.Errorf("expected next list item after code block, got: %s", result)
	}
}

func TestToMarkdown_PreBlockPreservesIndentation(t *testing.T) {
	input := `<pre>line1
line2
  indented</pre>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "```\nline1\nline2\n  indented\n```") {
		t.Errorf("expected pre block with preserved indentation, got: %s", result)
	}
}

func TestToMarkdown_InfoPanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="info"><ac:rich-text-body><p>This is important information.</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!NOTE]") {
		t.Errorf("expected NOTE alert, got: %s", result)
	}
	if !strings.Contains(result, "This is important information.") {
		t.Errorf("expected info content, got: %s", result)
	}
}

func TestToMarkdown_WarningPanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="warning"><ac:rich-text-body><p>Be careful!</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!WARNING]") {
		t.Errorf("expected WARNING alert, got: %s", result)
	}
}

func TestToMarkdown_ErrorPanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="error"><ac:rich-text-body><p>Something failed.</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!CAUTION]") {
		t.Errorf("expected CAUTION alert, got: %s", result)
	}
	if !strings.Contains(result, "Something failed.") {
		t.Errorf("expected error content, got: %s", result)
	}
}

func TestToMarkdown_SuccessPanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="success"><ac:rich-text-body><p>All good!</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!NOTE]") {
		t.Errorf("expected NOTE alert for success panel, got: %s", result)
	}
	if !strings.Contains(result, "All good!") {
		t.Errorf("expected success content, got: %s", result)
	}
}

func TestToMarkdown_DecisionPanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="decision"><ac:rich-text-body><p>We decided X.</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!IMPORTANT]") {
		t.Errorf("expected IMPORTANT alert for decision panel, got: %s", result)
	}
	if !strings.Contains(result, "We decided X.") {
		t.Errorf("expected decision content, got: %s", result)
	}
}

func TestToMarkdown_NotePanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="note"><ac:rich-text-body><p>Take note of this.</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!NOTE]") {
		t.Errorf("expected NOTE alert, got: %s", result)
	}
	if !strings.Contains(result, "Take note of this.") {
		t.Errorf("expected note content, got: %s", result)
	}
}

func TestToMarkdown_TipPanel(t *testing.T) {
	input := `<ac:structured-macro ac:name="tip"><ac:rich-text-body><p>Here is a tip.</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!TIP]") {
		t.Errorf("expected TIP alert, got: %s", result)
	}
	if !strings.Contains(result, "Here is a tip.") {
		t.Errorf("expected tip content, got: %s", result)
	}
}

func TestToMarkdown_PanelWithTitle(t *testing.T) {
	input := `<ac:structured-macro ac:name="warning"><ac:parameter ac:name="title">Watch Out</ac:parameter><ac:rich-text-body><p>Danger ahead.</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!WARNING]") {
		t.Errorf("expected WARNING alert, got: %s", result)
	}
	if !strings.Contains(result, "> **Watch Out**") {
		t.Errorf("expected title line, got: %s", result)
	}
	if !strings.Contains(result, "Danger ahead.") {
		t.Errorf("expected panel content, got: %s", result)
	}
}

func TestToMarkdown_PanelMultiLine(t *testing.T) {
	input := `<ac:structured-macro ac:name="info"><ac:rich-text-body><p>Line one</p><p>Line two</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)

	if !strings.Contains(result, "> [!NOTE]") {
		t.Errorf("expected NOTE alert, got: %s", result)
	}
	// Each content line should be prefixed with "> "
	for _, line := range strings.Split(result, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ">") {
			t.Errorf("expected all non-empty lines to start with '>', got line: %q", line)
		}
	}
}

func TestToMarkdown_EmptyBody(t *testing.T) {
	result := toMarkdown("", nil)
	if result != "" {
		t.Errorf("expected empty string for empty input, got: %q", result)
	}
}

func TestToMarkdown_Strikethrough(t *testing.T) {
	for _, tag := range []string{"del", "s"} {
		input := fmt.Sprintf("<p>This is <%s>removed</%s> text</p>", tag, tag)
		result := toMarkdown(input, nil)
		if !strings.Contains(result, "~~removed~~") {
			t.Errorf("expected strikethrough from <%s>, got: %s", tag, result)
		}
	}
}

func TestToMarkdown_Underline(t *testing.T) {
	input := `<p>This is <u>underlined</u> text</p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "*underlined*") {
		t.Errorf("expected underline as emphasis, got: %s", result)
	}
}

func TestToMarkdown_Blockquote(t *testing.T) {
	input := `<blockquote><p>Quoted text here</p></blockquote>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "> ") {
		t.Errorf("expected blockquote marker, got: %s", result)
	}
	if !strings.Contains(result, "Quoted text here") {
		t.Errorf("expected blockquote content, got: %s", result)
	}
}

func TestToMarkdown_SuperscriptSubscript(t *testing.T) {
	input := `<p>x<sup>2</sup> + H<sub>2</sub>O</p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "^(2)") {
		t.Errorf("expected superscript, got: %s", result)
	}
	if !strings.Contains(result, "~(2)") {
		t.Errorf("expected subscript, got: %s", result)
	}
}

func TestToMarkdown_DefinitionList(t *testing.T) {
	input := `<dl><dt>Term</dt><dd>Definition of term</dd></dl>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "**Term**") {
		t.Errorf("expected bold term, got: %s", result)
	}
	if !strings.Contains(result, ": Definition of term") {
		t.Errorf("expected definition, got: %s", result)
	}
}

func TestToMarkdown_OrderedList(t *testing.T) {
	input := `<ol><li>First</li><li>Second</li><li>Third</li></ol>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "1. First") {
		t.Errorf("expected numbered item 1, got: %s", result)
	}
	if !strings.Contains(result, "2. Second") {
		t.Errorf("expected numbered item 2, got: %s", result)
	}
	if !strings.Contains(result, "3. Third") {
		t.Errorf("expected numbered item 3, got: %s", result)
	}
}

func TestToMarkdown_UnorderedList(t *testing.T) {
	input := `<ul><li>Alpha</li><li>Beta</li></ul>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "- Alpha") {
		t.Errorf("expected bullet item, got: %s", result)
	}
	if !strings.Contains(result, "- Beta") {
		t.Errorf("expected bullet item, got: %s", result)
	}
}

func TestToMarkdown_NestedList(t *testing.T) {
	input := `<ul><li>Outer<ul><li>Inner</li></ul></li></ul>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "- Outer") {
		t.Errorf("expected outer item, got: %s", result)
	}
	if !strings.Contains(result, "  - Inner") {
		t.Errorf("expected indented inner item, got: %s", result)
	}
}

func TestToMarkdown_DeeplyNestedList(t *testing.T) {
	input := `<ul><li>A<ul><li>A1</li><li>A2<ul><li>A2a</li></ul></li></ul></li><li>B</li></ul>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "- A\n") {
		t.Errorf("expected top-level A, got: %q", result)
	}
	if !strings.Contains(result, "  - A1\n") {
		t.Errorf("expected indented A1, got: %q", result)
	}
	if !strings.Contains(result, "  - A2\n") {
		t.Errorf("expected indented A2, got: %q", result)
	}
	if !strings.Contains(result, "    - A2a\n") {
		t.Errorf("expected double-indented A2a, got: %q", result)
	}
	if !strings.Contains(result, "\n- B") {
		t.Errorf("expected top-level B, got: %q", result)
	}
}

func TestToMarkdown_MixedNestedList(t *testing.T) {
	input := `<ul><li>Fruits<ol><li>Apple</li><li>Banana</li></ol></li><li>Veggies</li></ul>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "- Fruits\n") {
		t.Errorf("expected unordered Fruits, got: %q", result)
	}
	if !strings.Contains(result, "  1. Apple\n") {
		t.Errorf("expected numbered Apple, got: %q", result)
	}
	if !strings.Contains(result, "  2. Banana\n") {
		t.Errorf("expected numbered Banana, got: %q", result)
	}
	if !strings.Contains(result, "\n- Veggies") {
		t.Errorf("expected unordered Veggies, got: %q", result)
	}
}

func TestToMarkdown_DeeplyNestedOrderedList(t *testing.T) {
	input := `<ol><li>One<ol><li>One-A<ol><li>One-A-i</li><li>One-A-ii</li></ol></li><li>One-B</li></ol></li><li>Two</li></ol>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "1. One\n") {
		t.Errorf("expected top-level 1, got: %q", result)
	}
	if !strings.Contains(result, "  1. One-A\n") {
		t.Errorf("expected nested One-A, got: %q", result)
	}
	if !strings.Contains(result, "    1. One-A-i\n") {
		t.Errorf("expected double-nested One-A-i, got: %q", result)
	}
	if !strings.Contains(result, "    2. One-A-ii\n") {
		t.Errorf("expected double-nested One-A-ii, got: %q", result)
	}
	if !strings.Contains(result, "  2. One-B\n") {
		t.Errorf("expected nested One-B, got: %q", result)
	}
	if !strings.Contains(result, "\n2. Two") {
		t.Errorf("expected top-level 2, got: %q", result)
	}
}

func TestToMarkdown_ListWithInlineCode(t *testing.T) {
	input := `<ul><li>Use <code>fmt.Println</code> for output</li><li>Use <code>log.Fatal</code> for errors</li></ul>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "- Use `fmt.Println` for output") {
		t.Errorf("expected inline code in list item, got: %s", result)
	}
	if !strings.Contains(result, "- Use `log.Fatal` for errors") {
		t.Errorf("expected inline code in second item, got: %s", result)
	}
}

func TestToMarkdown_ListWithFormatting(t *testing.T) {
	input := `<ul><li><strong>Bold item</strong><ul><li><em>Italic sub-item</em></li></ul></li></ul>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "- **Bold item**") {
		t.Errorf("expected bold in list item, got: %s", result)
	}
	if !strings.Contains(result, "  - *Italic sub-item*") {
		t.Errorf("expected italic in nested item, got: %s", result)
	}
}

func TestToMarkdown_NoformatMacro(t *testing.T) {
	input := `<ac:structured-macro ac:name="noformat"><ac:plain-text-body><![CDATA[some plain text
with newlines]]></ac:plain-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "```\nsome plain text\nwith newlines\n```") {
		t.Errorf("expected noformat as code block, got: %s", result)
	}
}

func TestToMarkdown_PanelMacro(t *testing.T) {
	input := `<ac:structured-macro ac:name="panel"><ac:parameter ac:name="title">My Panel</ac:parameter><ac:rich-text-body><p>Panel content</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "> **My Panel:**") {
		t.Errorf("expected panel title, got: %s", result)
	}
	if !strings.Contains(result, "Panel content") {
		t.Errorf("expected panel content, got: %s", result)
	}
}

func TestToMarkdown_PanelMacroNoTitle(t *testing.T) {
	input := `<ac:structured-macro ac:name="panel"><ac:rich-text-body><p>Just content</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "> ") {
		t.Errorf("expected blockquote, got: %s", result)
	}
	if !strings.Contains(result, "Just content") {
		t.Errorf("expected panel content, got: %s", result)
	}
}

func TestToMarkdown_ExcerptMacro(t *testing.T) {
	input := `<ac:structured-macro ac:name="excerpt"><ac:rich-text-body><p>This is an excerpt.</p></ac:rich-text-body></ac:structured-macro>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "This is an excerpt.") {
		t.Errorf("expected excerpt content inline, got: %s", result)
	}
}

func TestToMarkdown_StatusLozenge(t *testing.T) {
	input := `<p>Status: <ac:structured-macro ac:name="status"><ac:parameter ac:name="title">IN PROGRESS</ac:parameter><ac:parameter ac:name="colour">Blue</ac:parameter></ac:structured-macro></p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "`IN PROGRESS`") {
		t.Errorf("expected status lozenge as inline code, got: %s", result)
	}
}

func TestToMarkdown_DateMacro(t *testing.T) {
	input := `<p>Due: <ac:structured-macro ac:name="date"><ac:parameter ac:name="date">2024-01-15</ac:parameter></ac:structured-macro></p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "2024-01-15") {
		t.Errorf("expected date value, got: %s", result)
	}
}

func TestToMarkdown_AnchorMacro(t *testing.T) {
	input := `<p>Text before<ac:structured-macro ac:name="anchor"><ac:parameter ac:name="">section1</ac:parameter></ac:structured-macro>Text after</p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "Text beforeText after") {
		t.Errorf("expected anchor silently removed, got: %s", result)
	}
}

func TestToMarkdown_TOCMacro(t *testing.T) {
	input := `<ac:structured-macro ac:name="toc"><ac:parameter ac:name="maxLevel">3</ac:parameter></ac:structured-macro><h1>Title</h1>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "# Title") {
		t.Errorf("expected heading after TOC removal, got: %s", result)
	}
	if strings.Contains(result, "toc") {
		t.Errorf("expected TOC macro removed, got: %s", result)
	}
}

func TestToMarkdown_JIRAMacro(t *testing.T) {
	input := `<p>See <ac:structured-macro ac:name="jira"><ac:parameter ac:name="key">PROJ-123</ac:parameter></ac:structured-macro></p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "`PROJ-123`") {
		t.Errorf("expected JIRA key as inline code, got: %s", result)
	}
}

func TestToMarkdown_TaskList(t *testing.T) {
	input := `<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>Do this</ac:task-body></ac:task><ac:task><ac:task-status>complete</ac:task-status><ac:task-body>Already done</ac:task-body></ac:task></ac:task-list>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "- [ ] Do this") {
		t.Errorf("expected unchecked task, got: %s", result)
	}
	if !strings.Contains(result, "- [x] Already done") {
		t.Errorf("expected checked task, got: %s", result)
	}
}

func TestToMarkdown_UserMention(t *testing.T) {
	input := `<p>Assigned to <ac:link><ri:user ri:account-id="abc123" /><ac:plain-text-link-body><![CDATA[John Doe]]></ac:plain-text-link-body></ac:link></p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "@John Doe") {
		t.Errorf("expected user mention, got: %s", result)
	}
}

func TestToMarkdown_UserMentionNoBody(t *testing.T) {
	input := `<p>Assigned to <ac:link><ri:user ri:account-id="abc123" /></ac:link></p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "@user") {
		t.Errorf("expected fallback user mention, got: %s", result)
	}
}

func TestToMarkdown_PageLink(t *testing.T) {
	input := `<ac:link><ri:page ri:content-title="Getting Started" /><ac:plain-text-link-body><![CDATA[Read the guide]]></ac:plain-text-link-body></ac:link>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "[Read the guide](page:Getting Started)") {
		t.Errorf("expected page link with label, got: %s", result)
	}
}

func TestToMarkdown_PageLinkNoBody(t *testing.T) {
	input := `<ac:link><ri:page ri:content-title="Getting Started" /></ac:link>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "[Getting Started](page:Getting Started)") {
		t.Errorf("expected page link with title as label, got: %s", result)
	}
}

func TestToMarkdown_Emoticon(t *testing.T) {
	input := `<p>Great job <ac:emoticon ac:name="thumbs-up" /></p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "(thumbs-up)") {
		t.Errorf("expected emoticon text, got: %s", result)
	}
}

func TestToMarkdown_EmoticonUnknown(t *testing.T) {
	input := `<p>Custom <ac:emoticon ac:name="custom-emoji" /></p>`
	result := toMarkdown(input, nil)
	if !strings.Contains(result, "(custom-emoji)") {
		t.Errorf("expected fallback emoticon text, got: %s", result)
	}
}

func TestToMarkdown_ReferencedAttachments(t *testing.T) {
	input := `<ac:image><ri:attachment ri:filename="pic.png" /></ac:image><ac:link><ri:attachment ri:filename="doc.pdf" /></ac:link>`
	result := ReferencedAttachments(input)
	if !result["pic.png"] {
		t.Errorf("expected pic.png in referenced attachments, got: %v", result)
	}
	if !result["doc.pdf"] {
		t.Errorf("expected doc.pdf in referenced attachments, got: %v", result)
	}
}

func TestToMarkdown_UnknownTagsLogged(t *testing.T) {
	input := `<p>Hello <custom-widget>content</custom-widget></p>`
	result := ToMarkdown(input, nil)
	if len(result.UnknownTags) == 0 {
		t.Errorf("expected unknown tags to be logged, got none")
	}
	found := false
	for _, tag := range result.UnknownTags {
		if tag == "custom-widget" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected custom-widget in unknown tags, got: %v", result.UnknownTags)
	}
}

func TestToMarkdown_UnknownMacroLogged(t *testing.T) {
	input := `<ac:structured-macro ac:name="somethingcustom"><ac:parameter ac:name="key">val</ac:parameter></ac:structured-macro>`
	result := ToMarkdown(input, nil)
	found := false
	for _, tag := range result.UnknownTags {
		if tag == "macro:somethingcustom" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected macro:somethingcustom in unknown tags, got: %v", result.UnknownTags)
	}
}

func TestToMarkdown_KnownTagsNotLogged(t *testing.T) {
	input := `<p>Simple <strong>bold</strong> text</p>`
	result := ToMarkdown(input, nil)
	if len(result.UnknownTags) > 0 {
		t.Errorf("expected no unknown tags for basic HTML, got: %v", result.UnknownTags)
	}
}

func TestToMarkdown_NewTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "time element",
			input: `<p>Due <time datetime="2024-03-15" /></p>`,
			want:  "Due 2024-03-15",
		},
		{
			name:  "layout pass-through",
			input: `<ac:layout><ac:layout-section ac:type="two_equal"><ac:layout-cell><p>Left</p></ac:layout-cell><ac:layout-cell><p>Right</p></ac:layout-cell></ac:layout-section></ac:layout>`,
			want:  "Left\n\nRight",
		},
		{
			name:  "drawio macro with name",
			input: `<ac:structured-macro ac:name="drawio"><ac:parameter ac:name="diagramName">arch</ac:parameter></ac:structured-macro>`,
			want:  "*(diagram: arch)*",
		},
		{
			name:  "drawio macro without name",
			input: `<ac:structured-macro ac:name="drawio"></ac:structured-macro>`,
			want:  "*(diagram)*",
		},
		{
			name:  "view-file macro",
			input: `<ac:structured-macro ac:name="view-file"><ac:parameter ac:name="name">report.xlsx</ac:parameter></ac:structured-macro>`,
			want:  "[report.xlsx](attachment:report.xlsx)",
		},
		{
			name:  "viewpdf macro",
			input: `<ac:structured-macro ac:name="viewpdf"><ac:parameter ac:name="name">spec.pdf</ac:parameter></ac:structured-macro>`,
			want:  "[spec.pdf](attachment:spec.pdf)",
		},
		{
			name:  "ADF fallback rendered",
			input: `<ac:adf-extension><ac:adf-node type="extension"><ac:adf-attribute key="type">com.example</ac:adf-attribute></ac:adf-node><ac:adf-fallback><p>Fallback text</p></ac:adf-fallback></ac:adf-extension>`,
			want:  "Fallback text",
		},
		{
			name:  "inline comment marker pass-through",
			input: `<p>Some <ac:inline-comment-marker ac:ref="abc">commented</ac:inline-comment-marker> text</p>`,
			want:  "Some commented text",
		},
		{
			name:  "children macro without page",
			input: `<p>Above</p><ac:structured-macro ac:name="children"></ac:structured-macro><p>Below</p>`,
			want:  "Above\n\n*(children)*\nBelow",
		},
		{
			name:  "children macro with page",
			input: `<ac:structured-macro ac:name="children"><ac:parameter ac:name="page">Parent Page</ac:parameter></ac:structured-macro>`,
			want:  "*(children of [Parent Page](page:Parent Page))*",
		},
		{
			name:  "content-report-table with labels",
			input: `<ac:structured-macro ac:name="content-report-table"><ac:parameter ac:name="labels">roadmap,q1</ac:parameter></ac:structured-macro>`,
			want:  "*(content report: roadmap,q1)*",
		},
		{
			name:  "decisionreport with label",
			input: `<ac:structured-macro ac:name="decisionreport"><ac:parameter ac:name="label">team-decisions</ac:parameter></ac:structured-macro>`,
			want:  "*(decision report: team-decisions)*",
		},
		{
			name:  "listlabels macro",
			input: `<ac:structured-macro ac:name="listlabels"></ac:structured-macro>`,
			want:  "*(labels)*",
		},
		{
			name:  "details macro renders body",
			input: `<ac:structured-macro ac:name="details"><ac:rich-text-body><p>Detail content</p></ac:rich-text-body></ac:structured-macro>`,
			want:  "Detail content",
		},
		{
			name:  "drawio-sketch macro",
			input: `<ac:structured-macro ac:name="drawio-sketch"><ac:parameter ac:name="diagramName">sketch1</ac:parameter></ac:structured-macro>`,
			want:  "*(diagram: sketch1)*",
		},
		{
			name:  "pagetree macro with root",
			input: `<ac:structured-macro ac:name="pagetree"><ac:parameter ac:name="root">Engineering</ac:parameter></ac:structured-macro>`,
			want:  "*(page tree: [Engineering](page:Engineering))*",
		},
		{
			name:  "pagetree macro without root",
			input: `<ac:structured-macro ac:name="pagetree"></ac:structured-macro>`,
			want:  "*(page tree)*",
		},
		{
			name:  "attachments macro",
			input: `<ac:structured-macro ac:name="attachments"></ac:structured-macro>`,
			want:  "*(attachments)*",
		},
		{
			name:  "tasks-report-macro dropped",
			input: `<p>Before</p><ac:structured-macro ac:name="tasks-report-macro"></ac:structured-macro><p>After</p>`,
			want:  "Before\n\nAfter",
		},
		{
			name:  "contentbylabel with label",
			input: `<ac:structured-macro ac:name="contentbylabel"><ac:parameter ac:name="label">architecture</ac:parameter></ac:structured-macro>`,
			want:  "*(content by label: architecture)*",
		},
		{
			name:  "ac-placeholder pass-through",
			input: `<p>Some <ac:placeholder>placeholder</ac:placeholder> text</p>`,
			want:  "Some placeholder text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToMarkdown(tt.input, nil)
			got := strings.TrimSpace(result.Markdown)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if len(result.UnknownTags) > 0 {
				t.Errorf("unexpected unknown tags: %v", result.UnknownTags)
			}
		})
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
