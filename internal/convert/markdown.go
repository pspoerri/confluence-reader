package convert

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/pspoerri/confluence-reader/internal/api"
)

// Result holds the output of a Confluence-to-Markdown conversion.
type Result struct {
	Markdown    string
	UnknownTags []string // unhandled HTML/XML tags encountered during conversion
}

// ToMarkdown converts Confluence storage format HTML into Markdown.
// Attachment references are rewritten to [filename](attachment:filename).
func ToMarkdown(storageHTML string, attachments []api.Attachment) Result {
	c := &converter{}
	s := c.convert(storageHTML)

	// Append attachment list at the end if any exist.
	if len(attachments) > 0 {
		s += "\n\n---\n\n## Attachments\n\n"
		for _, a := range attachments {
			s += fmt.Sprintf("- [%s](attachment:%s) (%s, %s)\n", a.Title, a.Title, a.MediaType, formatSize(a.FileSize))
		}
	}

	return Result{
		Markdown:    strings.TrimSpace(s),
		UnknownTags: c.unknownTags(),
	}
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

// nodeKind identifies the type of a parsed XML node.
type nodeKind int

const (
	documentNode nodeKind = iota
	elementNode
	textNode
)

// node is a simple tree node for the parsed XML content.
type node struct {
	kind        nodeKind
	data        string     // tag name for elements, text content for text nodes
	attr        []xml.Attr // attributes (elements only)
	firstChild  *node
	lastChild   *node
	nextSibling *node
}

// appendChild adds a child to the end of n's children list.
func (n *node) appendChild(child *node) {
	if n.lastChild == nil {
		n.firstChild = child
	} else {
		n.lastChild.nextSibling = child
	}
	n.lastChild = child
}

// converter holds state during HTML-to-Markdown conversion.
type converter struct {
	buf       strings.Builder
	listDepth int
	unknown   map[string]struct{} // unhandled tags seen during conversion
}

// logUnknown records a tag name as unhandled.
func (c *converter) logUnknown(tag string) {
	if c.unknown == nil {
		c.unknown = make(map[string]struct{})
	}
	c.unknown[tag] = struct{}{}
}

// unknownTags returns a sorted list of unhandled tag names.
func (c *converter) unknownTags() []string {
	if len(c.unknown) == 0 {
		return nil
	}
	tags := make([]string, 0, len(c.unknown))
	for tag := range c.unknown {
		tags = append(tags, tag)
	}
	// Sort is not critical but makes output deterministic.
	for i := range tags {
		for j := i + 1; j < len(tags); j++ {
			if tags[i] > tags[j] {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}
	return tags
}

// rewriteNamespaces rewrites Confluence XML namespace prefixes (ac:, ri:)
// to hyphens so the XML parser treats them as regular element/attribute names.
func rewriteNamespaces(s string) string {
	s = strings.ReplaceAll(s, "<ac:", "<ac-")
	s = strings.ReplaceAll(s, "</ac:", "</ac-")
	s = strings.ReplaceAll(s, "<ri:", "<ri-")
	s = strings.ReplaceAll(s, "</ri:", "</ri-")
	s = strings.ReplaceAll(s, " ac:", " ac-")
	s = strings.ReplaceAll(s, " ri:", " ri-")
	return s
}

// convert is the main entry point: pre-process namespaces, parse, walk the tree.
func (c *converter) convert(storageHTML string) string {
	if storageHTML == "" {
		return ""
	}

	s := rewriteNamespaces(storageHTML)

	doc, err := parseXML(s)
	if err != nil {
		// Fallback: return the raw HTML with tags stripped.
		return stripTags(storageHTML)
	}

	c.renderNode(doc)

	result := c.buf.String()

	// Collapse excessive newlines.
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}

// parseXML parses a pre-processed HTML/XML string into a simple node tree.
// It wraps the input in a synthetic <root> element for well-formedness.
func parseXML(s string) (*node, error) {
	decoder := xml.NewDecoder(strings.NewReader("<root>" + s + "</root>"))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	doc := &node{kind: documentNode}
	stack := []*node{doc}

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		parent := stack[len(stack)-1]
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if name == "root" && len(stack) == 1 {
				continue // skip our synthetic wrapper
			}
			elem := &node{
				kind: elementNode,
				data: name,
				attr: make([]xml.Attr, len(t.Attr)),
			}
			copy(elem.attr, t.Attr)
			parent.appendChild(elem)
			stack = append(stack, elem)

		case xml.EndElement:
			name := t.Name.Local
			if name == "root" && len(stack) == 1 {
				continue
			}
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}

		case xml.CharData:
			text := string(t)
			if text != "" {
				tn := &node{kind: textNode, data: text}
				parent.appendChild(tn)
			}
		}
	}

	return doc, nil
}

// stripTags removes all XML/HTML tags from a string and decodes HTML entities.
func stripTags(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			buf.WriteRune(r)
		}
	}
	return html.UnescapeString(buf.String())
}

// renderNode dispatches rendering for a single node and its children.
func (c *converter) renderNode(n *node) {
	switch n.kind {
	case textNode:
		c.buf.WriteString(n.data)
		return

	case elementNode:
		if c.renderElement(n) {
			return // element handled itself and its children
		}

	case documentNode:
		// fall through to render children
	}

	// Default: render children.
	for child := n.firstChild; child != nil; child = child.nextSibling {
		c.renderNode(child)
	}
}

// renderElement handles a single element. Returns true if it handled
// its own children (so the caller should not recurse into them).
func (c *converter) renderElement(n *node) bool {
	switch n.data {

	// --- Confluence custom elements (namespace rewritten from ac:/ri: to ac-/ri-) ---

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
	case "ac-adf-node", "ac-adf-attribute", "ac-adf-content":
		// ADF internals — the ac-adf-fallback sibling has the readable content.
		return true
	case "time":
		if dt := attr(n, "datetime"); dt != "" {
			c.buf.WriteString(dt)
		}
		return true

	// --- Block elements ---

	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(n.data[1] - '0')
		c.buf.WriteString(strings.Repeat("#", level) + " ")
		c.renderChildren(n)
		c.buf.WriteString("\n\n")
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
		inner := c.renderToString(n)
		inner = strings.TrimSpace(inner)
		for _, line := range strings.Split(inner, "\n") {
			c.buf.WriteString("\n> " + line)
		}
		c.buf.WriteString("\n")
		return true
	case "pre":
		inner := collectAllText(n)
		c.buf.WriteString("\n```\n" + inner + "\n```\n")
		return true

	// --- Inline formatting ---

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
		c.buf.WriteString("`" + collectAllText(n) + "`")
		return true
	case "a":
		c.buf.WriteString("[")
		c.renderChildren(n)
		c.buf.WriteString("](" + attr(n, "href") + ")")
		return true

	// --- Lists ---

	case "ul":
		c.renderList(n, false)
		return true
	case "ol":
		c.renderList(n, true)
		return true

	// --- Tables ---

	case "table":
		c.renderTable(n)
		return true

	// --- Definition lists ---

	case "dl":
		c.renderDefinitionList(n)
		return true
	}

	// Log truly unknown tags. Skip known structural/pass-through elements.
	switch n.data {
	case "div", "span", "li",
		"thead", "tbody", "tfoot", "tr", "th", "td",
		"colgroup", "col",
		"html", "head", "body",
		"ac-parameter", "ac-rich-text-body", "ac-plain-text-body",
		"ac-task", "ac-task-status", "ac-task-body",
		"ac-layout", "ac-layout-section", "ac-layout-cell",
		"ac-link-body", "ac-adf-extension", "ac-adf-fallback",
		"ac-inline-comment-marker",
		"ri-attachment", "ri-user", "ri-page", "ri-space",
		"img", "figure", "figcaption", "section", "article",
		"nav", "header", "footer", "main", "aside":
		// Known pass-through or internally handled — don't log.
	default:
		c.logUnknown(n.data)
	}
	return false
}

// renderChildren renders all child nodes of n.
func (c *converter) renderChildren(n *node) {
	for child := n.firstChild; child != nil; child = child.nextSibling {
		c.renderNode(child)
	}
}

// renderToString renders a node's children into a temporary buffer and returns the result.
func (c *converter) renderToString(n *node) string {
	saved := c.buf
	c.buf = strings.Builder{}
	c.renderChildren(n)
	result := c.buf.String()
	c.buf = saved
	return result
}

// --- Confluence macros ---

// renderMacro dispatches ac-structured-macro elements by their ac-name attribute.
func (c *converter) renderMacro(n *node) {
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

	case "anchor", "toc",
		"livesearch", "miro-macro-resizing",
		"profile-picture", "recently-updated":
		// Drop silently — dynamic widgets with no static content.

	case "children":
		page := macroParam(n, "page")
		if page != "" {
			c.buf.WriteString(fmt.Sprintf("\n*(children of [%s](page:%s))*\n", page, page))
		} else {
			c.buf.WriteString("\n*(children)*\n")
		}

	case "content-report-table":
		labels := macroParam(n, "labels")
		if labels != "" {
			c.buf.WriteString(fmt.Sprintf("\n*(content report: %s)*\n", labels))
		} else {
			c.buf.WriteString("\n*(content report)*\n")
		}

	case "decisionreport":
		label := macroParam(n, "label")
		if label != "" {
			c.buf.WriteString(fmt.Sprintf("\n*(decision report: %s)*\n", label))
		} else {
			c.buf.WriteString("\n*(decision report)*\n")
		}

	case "listlabels":
		c.buf.WriteString("*(labels)*")

	case "view-file", "viewpdf":
		name := macroParam(n, "name")
		if name != "" {
			c.buf.WriteString(fmt.Sprintf("[%s](attachment:%s)", name, name))
		}

	case "drawio", "inc-drawio":
		name := macroParam(n, "diagramName")
		if name != "" {
			c.buf.WriteString(fmt.Sprintf("*(diagram: %s)*", name))
		} else {
			c.buf.WriteString("*(diagram)*")
		}

	case "details":
		inner := c.renderMacroBody(n)
		c.buf.WriteString(inner)

	case "detailssummary":
		inner := c.renderMacroBody(n)
		c.buf.WriteString(inner)

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
		} else {
			c.logUnknown("macro:" + name)
		}
	}
}

// renderMacroBody finds the ac-rich-text-body child and renders its contents.
func (c *converter) renderMacroBody(n *node) string {
	body := findChild(n, "ac-rich-text-body")
	if body == nil {
		return ""
	}
	return strings.TrimSpace(c.renderToString(body))
}

// renderTaskList handles ac-task-list elements.
func (c *converter) renderTaskList(n *node) {
	for child := n.firstChild; child != nil; child = child.nextSibling {
		if child.kind == elementNode && child.data == "ac-task" {
			c.renderTask(child)
		}
	}
}

// renderTask handles a single ac-task element.
func (c *converter) renderTask(n *node) {
	status := ""
	body := ""
	for child := n.firstChild; child != nil; child = child.nextSibling {
		if child.kind != elementNode {
			continue
		}
		switch child.data {
		case "ac-task-status":
			status = strings.TrimSpace(collectAllText(child))
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
func (c *converter) renderACLink(n *node) {
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
func (c *converter) renderACImage(n *node) {
	if att := findDescendant(n, "ri-attachment"); att != nil {
		filename := attr(att, "ri-filename")
		c.buf.WriteString(fmt.Sprintf("![%s](attachment:%s)", filename, filename))
		return
	}
	// Fallback: render nothing for unrecognized image sources.
}

// --- Lists ---

// renderList renders an <ol> or <ul> element with proper nesting.
func (c *converter) renderList(n *node, ordered bool) {
	itemNum := 0
	for child := n.firstChild; child != nil; child = child.nextSibling {
		if child.kind != elementNode || child.data != "li" {
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
		for liChild := child.firstChild; liChild != nil; liChild = liChild.nextSibling {
			if liChild.kind == elementNode && (liChild.data == "ul" || liChild.data == "ol") {
				c.buf.WriteString("\n")
				c.listDepth++
				if liChild.data == "ol" {
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
func (c *converter) renderTable(n *node) {
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
func (c *converter) collectRows(n *node, rows *[][]string) {
	for child := n.firstChild; child != nil; child = child.nextSibling {
		if child.kind != elementNode {
			continue
		}
		switch child.data {
		case "thead", "tbody", "tfoot":
			c.collectRows(child, rows)
		case "tr":
			var cols []string
			for cell := child.firstChild; cell != nil; cell = cell.nextSibling {
				if cell.kind == elementNode && (cell.data == "th" || cell.data == "td") {
					text := strings.TrimSpace(c.renderToString(cell))
					// Collapse internal whitespace/newlines for table cells.
					text = strings.Join(strings.Fields(text), " ")
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

func (c *converter) renderDefinitionList(n *node) {
	c.buf.WriteString("\n")
	for child := n.firstChild; child != nil; child = child.nextSibling {
		if child.kind != elementNode {
			continue
		}
		switch child.data {
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
func attr(n *node, key string) string {
	for _, a := range n.attr {
		if a.Name.Local == key {
			return a.Value
		}
	}
	return ""
}

// findChild returns the first child element with the given tag name, or nil.
func findChild(n *node, tag string) *node {
	for child := n.firstChild; child != nil; child = child.nextSibling {
		if child.kind == elementNode && child.data == tag {
			return child
		}
	}
	return nil
}

// findDescendant returns the first descendant element with the given tag, depth-first.
func findDescendant(n *node, tag string) *node {
	for child := n.firstChild; child != nil; child = child.nextSibling {
		if child.kind == elementNode {
			if child.data == tag {
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
func macroParam(macro *node, name string) string {
	for child := macro.firstChild; child != nil; child = child.nextSibling {
		if child.kind == elementNode && child.data == "ac-parameter" && attr(child, "ac-name") == name {
			return strings.TrimSpace(collectAllText(child))
		}
	}
	return ""
}

// macroCDATA extracts the CDATA content from an ac-plain-text-body descendant.
func macroCDATA(macro *node) string {
	body := findDescendant(macro, "ac-plain-text-body")
	if body == nil {
		return ""
	}
	return collectAllText(body)
}

// descendantText finds a descendant element by tag and returns its text content.
func descendantText(n *node, tag string) string {
	child := findDescendant(n, tag)
	if child == nil {
		return ""
	}
	return strings.TrimSpace(collectAllText(child))
}

// collectAllText extracts all text from a node tree.
func collectAllText(n *node) string {
	var buf strings.Builder
	var walk func(*node)
	walk = func(nd *node) {
		if nd.kind == textNode {
			buf.WriteString(nd.data)
		}
		for child := nd.firstChild; child != nil; child = child.nextSibling {
			walk(child)
		}
	}
	walk(n)
	return buf.String()
}

// ReferencedAttachments returns the set of attachment filenames that are
// referenced in the Confluence storage-format HTML (via <ri:attachment>).
func ReferencedAttachments(storageHTML string) map[string]bool {
	s := rewriteNamespaces(storageHTML)

	doc, err := parseXML(s)
	if err != nil {
		return nil
	}

	result := make(map[string]bool)
	var walk func(*node)
	walk = func(n *node) {
		if n.kind == elementNode && n.data == "ri-attachment" {
			if filename := attr(n, "ri-filename"); filename != "" {
				result[filename] = true
			}
		}
		for child := n.firstChild; child != nil; child = child.nextSibling {
			walk(child)
		}
	}
	walk(doc)
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
