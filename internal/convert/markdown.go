package convert

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/pspoerri/confluence-reader/internal/api"
)

// ToMarkdown converts Confluence storage format HTML into Markdown.
// Attachment references are rewritten to [filename](attachment:filename).
func ToMarkdown(storageHTML string, attachments []api.Attachment) string {
	s := storageHTML

	// Build attachment lookup by filename.
	attachMap := make(map[string]api.Attachment, len(attachments))
	for _, a := range attachments {
		attachMap[a.Title] = a
	}

	// Replace <ac:image> tags with markdown image references.
	s = replaceACImages(s, attachMap)

	// Replace <ac:link> attachment references.
	s = replaceACLinks(s, attachMap)

	// Convert standard HTML to markdown.
	s = convertHTMLToMarkdown(s)

	// Append attachment list at the end if any exist.
	if len(attachments) > 0 {
		s += "\n\n---\n\n## Attachments\n\n"
		for _, a := range attachments {
			s += fmt.Sprintf("- [%s](attachment:%s) (%s, %s)\n", a.Title, a.Title, a.MediaType, formatSize(a.FileSize))
		}
	}

	return strings.TrimSpace(s)
}

// replaceACImages converts <ac:image> tags to markdown image syntax.
func replaceACImages(s string, attachments map[string]api.Attachment) string {
	// Match <ac:image ...><ri:attachment ri:filename="..." />...</ac:image>
	// (?s) enables dotall mode so . matches newlines (macros often span lines).
	re := regexp.MustCompile(`(?s)<ac:image[^>]*>.*?<ri:attachment\s+ri:filename="([^"]+)"\s*/?>.*?</ac:image>`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		filename := sub[1]
		return fmt.Sprintf("![%s](attachment:%s)", filename, filename)
	})
}

// replaceACLinks converts <ac:link> attachment references to markdown link syntax.
func replaceACLinks(s string, attachments map[string]api.Attachment) string {
	re := regexp.MustCompile(`(?s)<ac:link[^>]*>.*?<ri:attachment\s+ri:filename="([^"]+)"\s*/?>.*?(?:<ac:plain-text-link-body>\s*<!\[CDATA\[([^\]]*)\]\]>\s*</ac:plain-text-link-body>)?.*?</ac:link>`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		filename := sub[1]
		label := filename
		if len(sub) >= 3 && sub[2] != "" {
			label = sub[2]
		}
		return fmt.Sprintf("[%s](attachment:%s)", label, filename)
	})
}

// convertHTMLToMarkdown does a best-effort conversion of common HTML to markdown.
func convertHTMLToMarkdown(s string) string {
	// Extract code blocks first to protect their content from later transformations.
	// Use placeholders that cannot appear in normal content.
	var codeBlocks []string
	placeholder := func(i int) string { return fmt.Sprintf("\x00CODEBLOCK_%d\x00", i) }

	// Confluence code macro with optional language parameter.
	codeRe := regexp.MustCompile(`(?s)<ac:structured-macro[^>]*ac:name="code"[^>]*>(.*?)</ac:structured-macro>`)
	langRe := regexp.MustCompile(`(?s)<ac:parameter\s+ac:name="language"[^>]*>\s*(.*?)\s*</ac:parameter>`)
	bodyRe := regexp.MustCompile(`(?s)<ac:plain-text-body>\s*<!\[CDATA\[(.*?)\]\]>\s*</ac:plain-text-body>`)
	s = codeRe.ReplaceAllStringFunc(s, func(match string) string {
		lang := ""
		if m := langRe.FindStringSubmatch(match); len(m) >= 2 {
			lang = strings.TrimSpace(m[1])
		}
		code := ""
		if m := bodyRe.FindStringSubmatch(match); len(m) >= 2 {
			code = m[1]
		}
		block := fmt.Sprintf("\n```%s\n%s\n```\n", lang, code)
		idx := len(codeBlocks)
		codeBlocks = append(codeBlocks, block)
		return placeholder(idx)
	})

	// Plain <pre> blocks (sometimes used without the ac:structured-macro wrapper).
	preRe := regexp.MustCompile(`(?s)<pre[^>]*>(.*?)</pre>`)
	s = preRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := preRe.FindStringSubmatch(match)[1]
		// Strip HTML tags inside <pre> but preserve the text.
		inner = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(inner, "")
		inner = html.UnescapeString(inner)
		block := "\n```\n" + inner + "\n```\n"
		idx := len(codeBlocks)
		codeBlocks = append(codeBlocks, block)
		return placeholder(idx)
	})

	// Inline code — protect from entity decoding and tag stripping.
	inlineCodeRe := regexp.MustCompile(`(?s)<code[^>]*>(.*?)</code>`)
	s = inlineCodeRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := inlineCodeRe.FindStringSubmatch(match)[1]
		inner = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(inner, "")
		inner = html.UnescapeString(inner)
		block := "`" + inner + "`"
		idx := len(codeBlocks)
		codeBlocks = append(codeBlocks, block)
		return placeholder(idx)
	})

	// Confluence info/warning/note/tip/success/error/decision panels.
	// Convert to markdown blockquotes with a label prefix.
	// The inner content (ac:rich-text-body) is kept as HTML so subsequent
	// conversion steps can process it normally.
	panelNames := map[string]string{
		"info":     "Info",
		"note":     "Note",
		"warning":  "Warning",
		"tip":      "Tip",
		"success":  "Success",
		"error":    "Error",
		"decision": "Decision",
		"expand":   "Details",
	}
	for macro, label := range panelNames {
		panelRe := regexp.MustCompile(`(?s)<ac:structured-macro[^>]*ac:name="` + macro + `"[^>]*>(?:.*?<ac:parameter\s+ac:name="title"[^>]*>(.*?)</ac:parameter>)?.*?<ac:rich-text-body>(.*?)</ac:rich-text-body>\s*</ac:structured-macro>`)
		s = panelRe.ReplaceAllStringFunc(s, func(match string) string {
			sub := panelRe.FindStringSubmatch(match)
			title := ""
			if len(sub) >= 2 {
				title = strings.TrimSpace(sub[1])
			}
			body := ""
			if len(sub) >= 3 {
				body = sub[2]
			}
			header := label
			if title != "" {
				header = label + ": " + title
			}
			return fmt.Sprintf("\n> **%s:**\n> %s\n", header, body)
		})
	}

	// Headings.
	for i := 6; i >= 1; i-- {
		prefix := strings.Repeat("#", i)
		openTag := fmt.Sprintf("<h%d[^>]*>", i)
		closeTag := fmt.Sprintf("</h%d>", i)
		s = regexp.MustCompile(openTag).ReplaceAllString(s, prefix+" ")
		s = strings.ReplaceAll(s, closeTag, "\n\n")
	}

	// Bold.
	s = regexp.MustCompile(`<strong\b[^>]*>`).ReplaceAllString(s, "**")
	s = strings.ReplaceAll(s, "</strong>", "**")
	s = regexp.MustCompile(`<b\b[^>]*>`).ReplaceAllString(s, "**")
	s = strings.ReplaceAll(s, "</b>", "**")

	// Italic.
	s = regexp.MustCompile(`<em\b[^>]*>`).ReplaceAllString(s, "*")
	s = strings.ReplaceAll(s, "</em>", "*")
	s = regexp.MustCompile(`<i\b[^>]*>`).ReplaceAllString(s, "*")
	s = strings.ReplaceAll(s, "</i>", "*")

	// Links.
	s = regexp.MustCompile(`<a[^>]+href="([^"]*)"[^>]*>(.*?)</a>`).ReplaceAllString(s, "[$2]($1)")

	// Ordered list items: replace <li> inside <ol> with numbered markers.
	olRe := regexp.MustCompile(`(?s)<ol[^>]*>(.*?)</ol>`)
	s = olRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := olRe.FindStringSubmatch(match)[1]
		n := 0
		liRe := regexp.MustCompile(`<li[^>]*>`)
		inner = liRe.ReplaceAllStringFunc(inner, func(string) string {
			n++
			return fmt.Sprintf("%d. ", n)
		})
		return inner
	})
	s = regexp.MustCompile(`</?ol[^>]*>`).ReplaceAllString(s, "")

	// Unordered list items.
	s = regexp.MustCompile(`<li[^>]*>`).ReplaceAllString(s, "- ")
	s = strings.ReplaceAll(s, "</li>", "\n")

	// Paragraphs and line breaks.
	s = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "</p>", "\n\n")
	s = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(s, "\n")

	// Horizontal rules.
	s = regexp.MustCompile(`<hr\s*/?>`).ReplaceAllString(s, "\n---\n")

	// Tables: convert to proper markdown tables with header separator.
	s = convertTables(s)

	// Strip remaining tags.
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")

	// Decode HTML entities (handles all named and numeric entities).
	s = html.UnescapeString(s)

	// Restore code blocks (after entity decoding and tag stripping).
	for i, block := range codeBlocks {
		s = strings.Replace(s, placeholder(i), block, 1)
	}

	// Collapse excessive newlines.
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")

	return s
}

// convertTables converts HTML tables into proper markdown tables.
func convertTables(s string) string {
	tableRe := regexp.MustCompile(`(?s)<table[^>]*>(.*?)</table>`)
	trRe := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	thRe := regexp.MustCompile(`(?s)<th[^>]*>(.*?)</th>`)
	tdRe := regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	tagRe := regexp.MustCompile(`<[^>]+>`)

	return tableRe.ReplaceAllStringFunc(s, func(table string) string {
		inner := tableRe.FindStringSubmatch(table)[1]
		rows := trRe.FindAllStringSubmatch(inner, -1)
		if len(rows) == 0 {
			return table
		}

		var lines []string
		for i, row := range rows {
			rowHTML := row[1]

			// Detect if row contains <th> cells.
			isHeader := thRe.MatchString(rowHTML)
			cellRe := tdRe
			if isHeader {
				cellRe = thRe
			}

			cells := cellRe.FindAllStringSubmatch(rowHTML, -1)
			var cols []string
			for _, cell := range cells {
				// Strip inner HTML tags and clean up whitespace.
				text := tagRe.ReplaceAllString(cell[1], "")
				text = strings.TrimSpace(text)
				// Collapse internal newlines to spaces for table cells.
				text = regexp.MustCompile(`\s*\n\s*`).ReplaceAllString(text, " ")
				cols = append(cols, text)
			}

			line := "| " + strings.Join(cols, " | ") + " |"
			lines = append(lines, line)

			// Insert separator after the first row (header row).
			if i == 0 {
				seps := make([]string, len(cols))
				for j := range seps {
					seps[j] = "---"
				}
				lines = append(lines, "| "+strings.Join(seps, " | ")+" |")
			}
		}

		return "\n" + strings.Join(lines, "\n") + "\n"
	})
}

// ReferencedAttachments returns the set of attachment filenames that are
// referenced in the Confluence storage-format HTML (via <ri:attachment>).
func ReferencedAttachments(storageHTML string) map[string]bool {
	re := regexp.MustCompile(`<ri:attachment\s+ri:filename="([^"]+)"`)
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
