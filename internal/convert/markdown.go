package convert

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pspoerri/confluence-reader/internal/api"
	"golang.org/x/net/html"
)

// ToMarkdown converts Confluence storage format HTML into Markdown.
// Attachment references are rewritten to [filename](attachment:filename).
func ToMarkdown(storageHTML string, attachments []api.Attachment) string {
	attachMap := make(map[string]api.Attachment, len(attachments))
	for _, a := range attachments {
		attachMap[a.Title] = a
	}

	c := &converter{
		attachments: attachMap,
	}
	s := c.convert(storageHTML)

	// Append attachment list at the end if any exist.
	if len(attachments) > 0 {
		s += "\n\n---\n\n## Attachments\n\n"
		for _, a := range attachments {
			s += fmt.Sprintf("- [%s](attachment:%s) (%s, %s)\n", a.Title, a.Title, a.MediaType, formatSize(a.FileSize))
		}
	}

	return strings.TrimSpace(s)
}

// emoticonMap maps Confluence emoticon names to text representations.
var emoticonMap = map[string]string{
	"smile": "(smile)", "sad": "(sad)", "cheeky": "(cheeky)",
	"laugh": "(laugh)", "wink": "(wink)", "thumbs-up": "(thumbs-up)",
	"thumbs-down": "(thumbs-down)", "information": "(i)",
	"tick": "(check)", "cross": "(x)", "warning": "(warning)",
	"plus": "(+)", "minus": "(-)", "question": "(?)",
	"light-on": "(idea)", "light-off": "(off)", "yellow-star": "(star)",
	"red-star": "(star)", "green-star": "(star)", "blue-star": "(star)",
	"heart": "(heart)", "broken-heart": "(broken-heart)",
}

// panelNames maps Confluence panel macro names to their display labels.
var panelNames = map[string]string{
	"info": "Info", "note": "Note", "warning": "Warning",
	"tip": "Tip", "success": "Success", "error": "Error",
	"decision": "Decision", "expand": "Details",
}

// converter holds state during HTML-to-Markdown conversion.
type converter struct {
	attachments map[string]api.Attachment
	buf         strings.Builder
	listDepth   int
	inCode      bool // inside <pre> or code macro — suppress formatting
}

// convert is the main entry point: pre-process namespaces, parse, walk the tree.
func (c *converter) convert(storageHTML string) string {
	if storageHTML == "" {
		return ""
	}

	// Confluence storage format uses XML namespaces (ac:, ri:) that are not
	// valid HTML5. Rewrite the colons to hyphens so the HTML parser treats
	// them as regular custom elements.
	s := storageHTML
	s = strings.ReplaceAll(s, "<ac:", "<ac-")
	s = strings.ReplaceAll(s, "</ac:", "</ac-")
	s = strings.ReplaceAll(s, "<ri:", "<ri-")
	s = strings.ReplaceAll(s, "</ri:", "</ri-")
	// Also handle attribute prefixes.
	s = strings.ReplaceAll(s, " ac:", " ac-")
	s = strings.ReplaceAll(s, " ri:", " ri-")

	// CDATA sections are XML-only and not supported by the HTML5 parser.
	// Convert <![CDATA[...]]> to HTML-escaped text so the content survives parsing.
	cdataRe := regexp.MustCompile(`<!\[CDATA\[([\s\S]*?)\]\]>`)
	s = cdataRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := cdataRe.FindStringSubmatch(match)[1]
		// Escape HTML special chars so the parser treats them as text.
		inner = strings.ReplaceAll(inner, "&", "&amp;")
		inner = strings.ReplaceAll(inner, "<", "&lt;")
		inner = strings.ReplaceAll(inner, ">", "&gt;")
		return inner
	})

	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		// Fallback: return the raw HTML with tags stripped.
		return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(storageHTML, "")
	}

	c.renderNode(doc)

	result := c.buf.String()

	// Collapse excessive newlines.
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

	return result
}

// renderNode dispatches rendering for a single node and its children.
func (c *converter) renderNode(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		text := n.Data
		if !c.inCode {
			c.buf.WriteString(text)
		} else {
			c.buf.WriteString(text)
		}
		return

	case html.ElementNode:
		if c.renderElement(n) {
			return // element handled itself and its children
		}

	case html.DocumentNode:
		// fall through to render children
	}

	// Default: render children.
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.renderNode(child)
	}
}

// renderElement handles a single HTML element. Returns true if it handled
// its own children (so the caller should not recurse into them).
func (c *converter) renderElement(n *html.Node) bool {
	tag := n.Data

	// Confluence custom elements (namespace rewritten from ac: to ac-).
	switch tag {
	case "ac-structured-macro":
		c.renderMacro(n)
		return true
	case "ac-task-list":
		c.renderTaskList(n)
		return true
	case "ac-link":
		c.renderACLink(n)
		return true
	case "ac-image":
		c.renderACImage(n)
		return true
	case "ac-emoticon":
		name := attr(n, "ac-name")
		if text, ok := emoticonMap[name]; ok {
			c.buf.WriteString(text)
		} else if name != "" {
			c.buf.WriteString("(" + name + ")")
		}
		return true
	}

	// Standard HTML elements.
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(tag[1] - '0')
		c.buf.WriteString(strings.Repeat("#", level) + " ")
		c.renderChildren(n)
		c.buf.WriteString("\n\n")
		return true

	case "strong", "b":
		c.buf.WriteString("**")
		c.renderChildren(n)
		c.buf.WriteString("**")
		return true

	case "em", "i":
		c.buf.WriteString("*")
		c.renderChildren(n)
		c.buf.WriteString("*")
		return true

	case "del", "s", "strike":
		c.buf.WriteString("~~")
		c.renderChildren(n)
		c.buf.WriteString("~~")
		return true

	case "u":
		c.buf.WriteString("*")
		c.renderChildren(n)
		c.buf.WriteString("*")
		return true

	case "sup":
		c.buf.WriteString("^(")
		c.renderChildren(n)
		c.buf.WriteString(")")
		return true

	case "sub":
		c.buf.WriteString("~(")
		c.renderChildren(n)
		c.buf.WriteString(")")
		return true

	case "code":
		inner := c.collectText(n)
		c.buf.WriteString("`" + inner + "`")
		return true

	case "pre":
		inner := c.collectText(n)
		c.buf.WriteString("\n```\n" + inner + "\n```\n")
		return true

	case "a":
		href := attr(n, "href")
		c.buf.WriteString("[")
		c.renderChildren(n)
		c.buf.WriteString("](")
		c.buf.WriteString(href)
		c.buf.WriteString(")")
		return true

	case "p":
		c.renderChildren(n)
		c.buf.WriteString("\n\n")
		return true

	case "br":
		c.buf.WriteString("\n")
		return true

	case "hr":
		c.buf.WriteString("\n---\n")
		return true

	case "blockquote":
		// Render children into a sub-buffer, then prefix each line with "> ".
		inner := c.renderToString(n)
		inner = strings.TrimSpace(inner)
		for _, line := range strings.Split(inner, "\n") {
			c.buf.WriteString("\n> " + line)
		}
		c.buf.WriteString("\n")
		return true

	case "ul":
		c.renderList(n, false)
		return true

	case "ol":
		c.renderList(n, true)
		return true

	case "table":
		c.renderTable(n)
		return true

	case "dl":
		c.renderDefinitionList(n)
		return true

	// Structural wrappers that we just pass through.
	case "html", "head", "body", "div", "span",
		"thead", "tbody", "tfoot", "colgroup", "col":
		return false // render children normally
	}

	return false // unknown tag — render children
}

// renderChildren renders all child nodes of n.
func (c *converter) renderChildren(n *html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.renderNode(child)
	}
}

// renderToString renders a node's children into a temporary buffer and returns the result.
func (c *converter) renderToString(n *html.Node) string {
	saved := c.buf
	c.buf = strings.Builder{}
	c.renderChildren(n)
	result := c.buf.String()
	c.buf = saved
	return result
}

// collectText extracts all text content from a node tree, stripping tags.
func (c *converter) collectText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return buf.String()
}

// --- Confluence macros ---

// renderMacro dispatches ac-structured-macro elements by their ac-name attribute.
func (c *converter) renderMacro(n *html.Node) {
	name := attr(n, "ac-name")

	switch name {
	case "code":
		lang := macroParam(n, "language")
		body := macroCDATA(n)
		c.buf.WriteString(fmt.Sprintf("\n```%s\n%s\n```\n", lang, body))

	case "noformat":
		body := macroCDATA(n)
		c.buf.WriteString("\n```\n" + body + "\n```\n")

	case "panel":
		title := macroParam(n, "title")
		inner := c.renderMacroBody(n)
		if title != "" {
			c.buf.WriteString(fmt.Sprintf("\n> **%s:**\n> %s\n", title, inner))
		} else {
			c.buf.WriteString(fmt.Sprintf("\n> %s\n", inner))
		}

	case "excerpt":
		inner := c.renderMacroBody(n)
		c.buf.WriteString(inner)

	case "status":
		title := macroParam(n, "title")
		c.buf.WriteString(" `" + title + "` ")

	case "date":
		date := macroParam(n, "date")
		c.buf.WriteString(date)

	case "jira":
		key := macroParam(n, "key")
		c.buf.WriteString("`" + key + "`")

	case "anchor", "toc":
		// Drop silently.

	default:
		// Named panels (info, note, warning, tip, etc.)
		if label, ok := panelNames[name]; ok {
			title := macroParam(n, "title")
			inner := c.renderMacroBody(n)
			header := label
			if title != "" {
				header = label + ": " + title
			}
			c.buf.WriteString(fmt.Sprintf("\n> **%s:**\n> %s\n", header, inner))
		}
		// Unknown macros: silently drop.
	}
}

// renderMacroBody finds the ac-rich-text-body child and renders its contents.
func (c *converter) renderMacroBody(n *html.Node) string {
	body := findChild(n, "ac-rich-text-body")
	if body == nil {
		return ""
	}
	return strings.TrimSpace(c.renderToString(body))
}

// renderTaskList handles ac-task-list elements.
func (c *converter) renderTaskList(n *html.Node) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "ac-task" {
			c.renderTask(child)
		}
	}
}

// renderTask handles a single ac-task element.
func (c *converter) renderTask(n *html.Node) {
	status := ""
	body := ""
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		switch child.Data {
		case "ac-task-status":
			status = strings.TrimSpace(c.collectText(child))
		case "ac-task-body":
			body = strings.TrimSpace(c.renderToString(child))
		}
	}
	checkbox := "- [ ] "
	if status == "complete" {
		checkbox = "- [x] "
	}
	c.buf.WriteString(checkbox + body + "\n")
}

// renderACLink handles ac-link elements (user mentions, page links, attachment links).
// Note: the HTML5 parser does not know that ri-* elements are void/self-closing,
// so subsequent siblings may be nested inside them. We use findDescendant to
// search the full subtree rather than just direct children.
func (c *converter) renderACLink(n *html.Node) {
	// Helper to get the CDATA body text from anywhere in the subtree.
	label := func() string { return descendantText(n, "ac-plain-text-link-body") }

	// Check for ri-attachment → attachment link.
	if att := findDescendant(n, "ri-attachment"); att != nil {
		filename := attr(att, "ri-filename")
		l := label()
		if l == "" {
			l = filename
		}
		c.buf.WriteString(fmt.Sprintf("[%s](attachment:%s)", l, filename))
		return
	}

	// Check for ri-user → user mention.
	if findDescendant(n, "ri-user") != nil {
		l := label()
		if l == "" {
			l = "user"
		}
		c.buf.WriteString("@" + l)
		return
	}

	// Check for ri-page → page link.
	if page := findDescendant(n, "ri-page"); page != nil {
		title := attr(page, "ri-content-title")
		l := label()
		if l == "" {
			l = title
		}
		c.buf.WriteString(fmt.Sprintf("[%s](page:%s)", l, title))
		return
	}

	// Unknown ac-link — render children as fallback.
	c.renderChildren(n)
}

// renderACImage handles ac-image elements.
func (c *converter) renderACImage(n *html.Node) {
	if att := findDescendant(n, "ri-attachment"); att != nil {
		filename := attr(att, "ri-filename")
		c.buf.WriteString(fmt.Sprintf("![%s](attachment:%s)", filename, filename))
		return
	}
	// Fallback: render nothing for unrecognized image sources.
}

// --- Lists ---

// renderList renders an <ol> or <ul> element with proper nesting.
func (c *converter) renderList(n *html.Node, ordered bool) {
	itemNum := 0
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.Data != "li" {
			continue
		}
		itemNum++

		indent := strings.Repeat("  ", c.listDepth)
		if ordered {
			c.buf.WriteString(fmt.Sprintf("%s%d. ", indent, itemNum))
		} else {
			c.buf.WriteString(indent + "- ")
		}

		// Render the li's children. Nested lists will be handled recursively.
		for liChild := child.FirstChild; liChild != nil; liChild = liChild.NextSibling {
			if liChild.Type == html.ElementNode && (liChild.Data == "ul" || liChild.Data == "ol") {
				c.buf.WriteString("\n")
				c.listDepth++
				if liChild.Data == "ol" {
					c.renderList(liChild, true)
				} else {
					c.renderList(liChild, false)
				}
				c.listDepth--
			} else {
				c.renderNode(liChild)
			}
		}

		// Ensure the item ends with a newline.
		s := c.buf.String()
		if len(s) > 0 && s[len(s)-1] != '\n' {
			c.buf.WriteString("\n")
		}
	}
}

// --- Tables ---

// renderTable collects all rows/cells from a <table> then emits a markdown table.
func (c *converter) renderTable(n *html.Node) {
	var rows [][]string
	c.collectRows(n, &rows)

	if len(rows) == 0 {
		return
	}

	c.buf.WriteString("\n")
	for i, row := range rows {
		c.buf.WriteString("| " + strings.Join(row, " | ") + " |")
		c.buf.WriteString("\n")
		if i == 0 {
			seps := make([]string, len(row))
			for j := range seps {
				seps[j] = "---"
			}
			c.buf.WriteString("| " + strings.Join(seps, " | ") + " |")
			c.buf.WriteString("\n")
		}
	}
}

// collectRows walks the table tree collecting rows from thead/tbody or directly.
func (c *converter) collectRows(n *html.Node, rows *[][]string) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		switch child.Data {
		case "thead", "tbody", "tfoot":
			c.collectRows(child, rows)
		case "tr":
			var cols []string
			for cell := child.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type == html.ElementNode && (cell.Data == "th" || cell.Data == "td") {
					text := strings.TrimSpace(c.renderToString(cell))
					// Collapse internal newlines for table cells.
					text = regexp.MustCompile(`\s*\n\s*`).ReplaceAllString(text, " ")
					cols = append(cols, text)
				}
			}
			if len(cols) > 0 {
				*rows = append(*rows, cols)
			}
		}
	}
}

// --- Definition lists ---

func (c *converter) renderDefinitionList(n *html.Node) {
	c.buf.WriteString("\n")
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		switch child.Data {
		case "dt":
			c.buf.WriteString("**")
			c.renderChildren(child)
			c.buf.WriteString("**\n")
		case "dd":
			c.buf.WriteString(": ")
			c.renderChildren(child)
			c.buf.WriteString("\n")
		}
	}
	c.buf.WriteString("\n")
}

// --- Helper functions ---

// attr returns the value of an attribute on a node (empty string if not found).
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// findChild returns the first child element with the given tag name, or nil.
func findChild(n *html.Node, tag string) *html.Node {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == tag {
			return child
		}
	}
	return nil
}

// findDescendant returns the first descendant element with the given tag, depth-first.
func findDescendant(n *html.Node, tag string) *html.Node {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			if child.Data == tag {
				return child
			}
			if found := findDescendant(child, tag); found != nil {
				return found
			}
		}
	}
	return nil
}

// macroParam extracts a named parameter from an ac-structured-macro.
// <ac-parameter ac-name="language">python</ac-parameter>
func macroParam(macro *html.Node, name string) string {
	for child := macro.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "ac-parameter" && attr(child, "ac-name") == name {
			return strings.TrimSpace(collectAllText(child))
		}
	}
	return ""
}

// macroCDATA extracts the CDATA content from an ac-plain-text-body descendant.
func macroCDATA(macro *html.Node) string {
	body := findDescendant(macro, "ac-plain-text-body")
	if body == nil {
		return ""
	}
	return collectAllText(body)
}

// descendantText finds a descendant element by tag and returns its text content.
func descendantText(n *html.Node, tag string) string {
	child := findDescendant(n, tag)
	if child == nil {
		return ""
	}
	return strings.TrimSpace(collectAllText(child))
}

// collectAllText extracts all text from a node tree.
func collectAllText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return buf.String()
}

// ReferencedAttachments returns the set of attachment filenames that are
// referenced in the Confluence storage-format HTML (via <ri:attachment>).
func ReferencedAttachments(storageHTML string) map[string]bool {
	re := regexp.MustCompile(`<ri:attachment\s[^>]*ri:filename="([^"]+)"`)
	matches := re.FindAllStringSubmatch(storageHTML, -1)
	result := make(map[string]bool, len(matches))
	for _, m := range matches {
		result[m[1]] = true
	}
	return result
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
