package convert

import (
	"strings"
	"testing"
)

// TestToMarkdown_Robustness feeds malformed and edge-case storage HTML
// through ToMarkdown to make sure the parser never panics and degrades
// gracefully. The output assertions are deliberately loose — the goal is
// "doesn't crash and produces something usable" rather than exact bytes,
// since the conversion path runs the input through a non-strict XML
// parser with HTMLEntity decoding plus a stripTags fallback on parser
// failure.
func TestToMarkdown_Robustness(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains string // optional: substring that must appear in the output
	}{
		{"empty", "", ""},
		{"plain_text", "Just some text.", "Just some text."},
		{"only_whitespace", "   \n\t  ", ""},
		{"stray_ampersand", "Tom & Jerry", "Tom"},
		{"unclosed_p", "<p>hello", "hello"},
		{"unclosed_nested", "<ul><li>one<li>two</ul>", "one"},
		{"mismatched_tags", "<p><b>hello</p></b>", "hello"},
		{"self_closing_xhtml", `<p>line1<br/>line2</p>`, "line1"},
		{"img_self_closing", `<p>before<img src="x"/>after</p>`, "before"},
		{"html_comment", "<!-- ignore --><p>visible</p>", "visible"},
		{"comment_in_text", "text<!--inline-->more", "text"},
		{"cdata_inside_p", "<p><![CDATA[<raw>&data]]></p>", "raw"},
		{"mixed_case_tags", "<P><B>shouty</B></P>", "shouty"},
		{"numeric_entity", "<p>&#65;BC</p>", "ABC"},
		{"named_entity_amp", "<p>A &amp; B</p>", "A & B"},
		{"unknown_named_entity", "<p>&unknownentity; tail</p>", "tail"},
		{"empty_attr_value", `<img src= alt="x">`, ""},
		{"siblings_no_root", "<p>a</p><p>b</p>", "a"},
		{"bare_br", "before<br>after", "before"},
		{"namespaced_user", `<p>Hello <ac:link><ri:user ri:account-id="123"/><ac:plain-text-link-body><![CDATA[Pascal]]></ac:plain-text-link-body></ac:link></p>`, "@Pascal"},
		{"unknown_hyphenated_tag", "<my-custom-tag>inner</my-custom-tag>", "inner"},
		{"control_bytes_in_text", "<p>a\x01b\x02c</p>", "a"},
		{"unclosed_macro", `<ac:structured-macro ac:name="info"><ac:parameter ac:name="title">Hi</ac:parameter><ac:rich-text-body><p>body`, "body"},
		{"deeply_nested_unclosed", strings.Repeat("<div>", 50) + "tip", "tip"},
		{"ridiculously_long_text", "<p>" + strings.Repeat("ab ", 5000) + "</p>", "ab"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on input %q: %v", tt.input, r)
				}
			}()

			result := ToMarkdown(tt.input, nil, nil)
			if tt.wantContains != "" && !strings.Contains(result.Markdown, tt.wantContains) {
				t.Errorf("expected output to contain %q for input %q\n--- got ---\n%s",
					tt.wantContains, tt.input, result.Markdown)
			}
		})
	}
}

// TestParseXML_NeverPanics is a lower-level safety net: feed a variety of
// malformed XML directly to the parser and ensure neither parseXML nor the
// stripTags fallback ever blows up.
func TestParseXML_NeverPanics(t *testing.T) {
	inputs := []string{
		"",
		"plain text",
		"<",
		">",
		"<><><><>",
		"<unclosed",
		"<a href=>",
		"<a href='unterminated>",
		"<![CDATA[",
		"<![CDATA[never closed",
		"<!--",
		"<!-- never closed",
		"&amp;&lt;&gt;",
		"&#x",
		"&#",
		"\x00\x01\x02 binary garbage",
		strings.Repeat("<<<>>>", 100),
		strings.Repeat("&amp;", 1000),
	}

	for _, in := range inputs {
		in := in
		t.Run("input_len_"+lenLabel(in), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on input %q: %v", in, r)
				}
			}()
			c := &converter{}
			_ = c.convert(rewriteNamespaces(in))
		})
	}
}

func lenLabel(s string) string {
	switch {
	case len(s) == 0:
		return "0"
	case len(s) < 10:
		return "short"
	case len(s) < 100:
		return "medium"
	default:
		return "large"
	}
}
