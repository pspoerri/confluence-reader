package convert

import (
	"fmt"
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

	// Code blocks ((?s) for multi-line macros).
	s = regexp.MustCompile(`(?s)<ac:structured-macro[^>]*ac:name="code"[^>]*>.*?<ac:plain-text-body>\s*<!\[CDATA\[(.*?)\]\]>\s*</ac:plain-text-body>\s*</ac:structured-macro>`).
		ReplaceAllString(s, "\n```\n$1\n```\n")

	// Inline code.
	s = regexp.MustCompile(`<code[^>]*>`).ReplaceAllString(s, "`")
	s = strings.ReplaceAll(s, "</code>", "`")

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

	// Tables: basic conversion.
	s = regexp.MustCompile(`<table[^>]*>`).ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, "</table>", "\n")
	s = regexp.MustCompile(`<tr[^>]*>`).ReplaceAllString(s, "| ")
	s = strings.ReplaceAll(s, "</tr>", "\n")
	s = regexp.MustCompile(`<t[hd][^>]*>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`</t[hd]>`).ReplaceAllString(s, " | ")

	// Strip remaining tags.
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")

	// Decode common HTML entities.
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")

	// Collapse excessive newlines.
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")

	return s
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
