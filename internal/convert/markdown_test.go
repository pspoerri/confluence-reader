package convert

import (
	"fmt"
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

func TestToMarkdown_Strikethrough(t *testing.T) {
	for _, tag := range []string{"del", "s", "strike"} {
		input := fmt.Sprintf("<p>This is <%s>removed</%s> text</p>", tag, tag)
		result := ToMarkdown(input, nil)
		if !strings.Contains(result, "~~removed~~") {
			t.Errorf("expected strikethrough from <%s>, got: %s", tag, result)
		}
	}
}

func TestToMarkdown_Underline(t *testing.T) {
	input := `<p>This is <u>underlined</u> text</p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "*underlined*") {
		t.Errorf("expected underline as emphasis, got: %s", result)
	}
}

func TestToMarkdown_Blockquote(t *testing.T) {
	input := `<blockquote><p>Quoted text here</p></blockquote>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "> ") {
		t.Errorf("expected blockquote marker, got: %s", result)
	}
	if !strings.Contains(result, "Quoted text here") {
		t.Errorf("expected blockquote content, got: %s", result)
	}
}

func TestToMarkdown_SuperscriptSubscript(t *testing.T) {
	input := `<p>x<sup>2</sup> + H<sub>2</sub>O</p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "^(2)") {
		t.Errorf("expected superscript, got: %s", result)
	}
	if !strings.Contains(result, "~(2)") {
		t.Errorf("expected subscript, got: %s", result)
	}
}

func TestToMarkdown_DefinitionList(t *testing.T) {
	input := `<dl><dt>Term</dt><dd>Definition of term</dd></dl>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "**Term**") {
		t.Errorf("expected bold term, got: %s", result)
	}
	if !strings.Contains(result, ": Definition of term") {
		t.Errorf("expected definition, got: %s", result)
	}
}

func TestToMarkdown_OrderedList(t *testing.T) {
	input := `<ol><li>First</li><li>Second</li><li>Third</li></ol>`
	result := ToMarkdown(input, nil)
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
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "- Alpha") {
		t.Errorf("expected bullet item, got: %s", result)
	}
	if !strings.Contains(result, "- Beta") {
		t.Errorf("expected bullet item, got: %s", result)
	}
}

func TestToMarkdown_NestedList(t *testing.T) {
	input := `<ul><li>Outer<ul><li>Inner</li></ul></li></ul>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "- Outer") {
		t.Errorf("expected outer item, got: %s", result)
	}
	if !strings.Contains(result, "- Inner") {
		t.Errorf("expected inner item, got: %s", result)
	}
}

func TestToMarkdown_NoformatMacro(t *testing.T) {
	input := `<ac:structured-macro ac:name="noformat"><ac:plain-text-body><![CDATA[some plain text
with newlines]]></ac:plain-text-body></ac:structured-macro>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "```\nsome plain text\nwith newlines\n```") {
		t.Errorf("expected noformat as code block, got: %s", result)
	}
}

func TestToMarkdown_PanelMacro(t *testing.T) {
	input := `<ac:structured-macro ac:name="panel"><ac:parameter ac:name="title">My Panel</ac:parameter><ac:rich-text-body><p>Panel content</p></ac:rich-text-body></ac:structured-macro>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "> **My Panel:**") {
		t.Errorf("expected panel title, got: %s", result)
	}
	if !strings.Contains(result, "Panel content") {
		t.Errorf("expected panel content, got: %s", result)
	}
}

func TestToMarkdown_PanelMacroNoTitle(t *testing.T) {
	input := `<ac:structured-macro ac:name="panel"><ac:rich-text-body><p>Just content</p></ac:rich-text-body></ac:structured-macro>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "> ") {
		t.Errorf("expected blockquote, got: %s", result)
	}
	if !strings.Contains(result, "Just content") {
		t.Errorf("expected panel content, got: %s", result)
	}
}

func TestToMarkdown_ExcerptMacro(t *testing.T) {
	input := `<ac:structured-macro ac:name="excerpt"><ac:rich-text-body><p>This is an excerpt.</p></ac:rich-text-body></ac:structured-macro>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "This is an excerpt.") {
		t.Errorf("expected excerpt content inline, got: %s", result)
	}
}

func TestToMarkdown_StatusLozenge(t *testing.T) {
	input := `<p>Status: <ac:structured-macro ac:name="status"><ac:parameter ac:name="title">IN PROGRESS</ac:parameter><ac:parameter ac:name="colour">Blue</ac:parameter></ac:structured-macro></p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "`IN PROGRESS`") {
		t.Errorf("expected status lozenge as inline code, got: %s", result)
	}
}

func TestToMarkdown_DateMacro(t *testing.T) {
	input := `<p>Due: <ac:structured-macro ac:name="date"><ac:parameter ac:name="date">2024-01-15</ac:parameter></ac:structured-macro></p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "2024-01-15") {
		t.Errorf("expected date value, got: %s", result)
	}
}

func TestToMarkdown_AnchorMacro(t *testing.T) {
	input := `<p>Text before<ac:structured-macro ac:name="anchor"><ac:parameter ac:name="">section1</ac:parameter></ac:structured-macro>Text after</p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "Text beforeText after") {
		t.Errorf("expected anchor silently removed, got: %s", result)
	}
}

func TestToMarkdown_TOCMacro(t *testing.T) {
	input := `<ac:structured-macro ac:name="toc"><ac:parameter ac:name="maxLevel">3</ac:parameter></ac:structured-macro><h1>Title</h1>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "# Title") {
		t.Errorf("expected heading after TOC removal, got: %s", result)
	}
	if strings.Contains(result, "toc") {
		t.Errorf("expected TOC macro removed, got: %s", result)
	}
}

func TestToMarkdown_JIRAMacro(t *testing.T) {
	input := `<p>See <ac:structured-macro ac:name="jira"><ac:parameter ac:name="key">PROJ-123</ac:parameter></ac:structured-macro></p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "`PROJ-123`") {
		t.Errorf("expected JIRA key as inline code, got: %s", result)
	}
}

func TestToMarkdown_TaskList(t *testing.T) {
	input := `<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>Do this</ac:task-body></ac:task><ac:task><ac:task-status>complete</ac:task-status><ac:task-body>Already done</ac:task-body></ac:task></ac:task-list>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "- [ ] Do this") {
		t.Errorf("expected unchecked task, got: %s", result)
	}
	if !strings.Contains(result, "- [x] Already done") {
		t.Errorf("expected checked task, got: %s", result)
	}
}

func TestToMarkdown_UserMention(t *testing.T) {
	input := `<p>Assigned to <ac:link><ri:user ri:account-id="abc123" /><ac:plain-text-link-body><![CDATA[John Doe]]></ac:plain-text-link-body></ac:link></p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "@John Doe") {
		t.Errorf("expected user mention, got: %s", result)
	}
}

func TestToMarkdown_UserMentionNoBody(t *testing.T) {
	input := `<p>Assigned to <ac:link><ri:user ri:account-id="abc123" /></ac:link></p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "@user") {
		t.Errorf("expected fallback user mention, got: %s", result)
	}
}

func TestToMarkdown_PageLink(t *testing.T) {
	input := `<ac:link><ri:page ri:content-title="Getting Started" /><ac:plain-text-link-body><![CDATA[Read the guide]]></ac:plain-text-link-body></ac:link>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "[Read the guide](page:Getting Started)") {
		t.Errorf("expected page link with label, got: %s", result)
	}
}

func TestToMarkdown_PageLinkNoBody(t *testing.T) {
	input := `<ac:link><ri:page ri:content-title="Getting Started" /></ac:link>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "[Getting Started](page:Getting Started)") {
		t.Errorf("expected page link with title as label, got: %s", result)
	}
}

func TestToMarkdown_Emoticon(t *testing.T) {
	input := `<p>Great job <ac:emoticon ac:name="thumbs-up" /></p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "(thumbs-up)") {
		t.Errorf("expected emoticon text, got: %s", result)
	}
}

func TestToMarkdown_EmoticonUnknown(t *testing.T) {
	input := `<p>Custom <ac:emoticon ac:name="custom-emoji" /></p>`
	result := ToMarkdown(input, nil)
	if !strings.Contains(result, "(custom-emoji)") {
		t.Errorf("expected fallback emoticon text, got: %s", result)
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
