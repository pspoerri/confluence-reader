package cli

import (
	"fmt"
	"strings"
)

// Frontmatter is the YAML metadata block written at the top of each mirrored
// page's index.md. Fields with zero values are omitted (matching the legacy
// ad-hoc emitter), so output stays compact when the API didn't return them.
type Frontmatter struct {
	Title      string
	PageID     string
	Version    int
	CreatedAt  string
	AuthorID   string
	ModifiedAt string
	ModifiedBy string
	Source     string
}

// Encode renders the frontmatter as a YAML 1.2 block delimited by `---`.
// All string values are double-quoted with control characters and embedded
// quotes properly escaped, so titles containing `"`, `\`, newlines, or tabs
// round-trip through any YAML parser.
func (f Frontmatter) Encode() string {
	var b strings.Builder
	b.WriteString("---\n")
	writeYAMLString(&b, "title", f.Title)
	writeYAMLString(&b, "page_id", f.PageID)
	writeYAMLInt(&b, "version", f.Version)
	if f.CreatedAt != "" {
		writeYAMLString(&b, "created_at", f.CreatedAt)
	}
	writeYAMLString(&b, "author_id", f.AuthorID)
	if f.ModifiedAt != "" {
		writeYAMLString(&b, "modified_at", f.ModifiedAt)
	}
	writeYAMLString(&b, "modified_by", f.ModifiedBy)
	if f.Source != "" {
		writeYAMLString(&b, "source", f.Source)
	}
	b.WriteString("---\n")
	return b.String()
}

func writeYAMLString(b *strings.Builder, key, val string) {
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(yamlQuote(val))
	b.WriteByte('\n')
}

func writeYAMLInt(b *strings.Builder, key string, val int) {
	fmt.Fprintf(b, "%s: %d\n", key, val)
}

// yamlQuote wraps s in double quotes, escaping characters that have
// special meaning in YAML 1.2 double-quoted scalars.
func yamlQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
