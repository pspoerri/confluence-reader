package convert

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Slugify converts a string into a URL/filesystem-safe slug.
// "Getting Started!" → "getting-started"
func Slugify(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else if r == ' ' || r == '-' || r == '_' || r == '.' {
			b.WriteByte('-')
		}
	}
	slug := b.String()

	// Collapse multiple hyphens.
	slug = regexp.MustCompile(`-{2,}`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if slug == "" {
		return "file"
	}
	return slug
}

// RenameAttachment produces a new filename following the convention:
// <page-title-slug>-<filename-stem-slug>-<fileID>.<ext>
func RenameAttachment(pageTitle, originalFilename, fileID string) string {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	stem := strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))

	pageSlug := Slugify(pageTitle)
	stemSlug := Slugify(stem)

	// Avoid duplication if the stem already starts with the page slug.
	if strings.HasPrefix(stemSlug, pageSlug) {
		return stemSlug + "-" + fileID + ext
	}

	return pageSlug + "-" + stemSlug + "-" + fileID + ext
}
