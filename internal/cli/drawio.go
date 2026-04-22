package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pspoerri/confluence-reader/internal/api"
	"github.com/pspoerri/confluence-reader/internal/drawio"
	"github.com/pspoerri/confluence-reader/internal/progress"
)

// drawioMissingOnce ensures we only emit the "drawio CLI not installed"
// notice once per mirror run, no matter how many .drawio attachments we see.
var drawioMissingOnce sync.Once

// renderDrawioPlaceholders shells out to the draw.io desktop CLI to convert
// each downloaded .drawio attachment to a sibling cropped PDF, then rewrites
// the *(diagram: name)* placeholder in the markdown to link the PDF with a
// fallback link to the .drawio source.
//
// When the draw.io CLI isn't installed we leave the placeholder in place
// (printing a one-time notice) so the markdown still references the source
// .drawio file. Per-file render failures are warned about and skipped.
func renderDrawioPlaceholders(md, dir string, renameMap map[string]string, atts []api.Attachment, bar *progress.Bar) (string, []string) {
	var written []string

	cliAvailable := drawio.Available()
	if !cliAvailable {
		// Only emit the install hint once; if there are no drawio files in
		// the entire mirror we never bother the user at all.
		for _, att := range atts {
			if isDrawio(att) {
				drawioMissingOnce.Do(func() {
					bar.Log("info: draw.io CLI not found — leaving *(diagram: ...)* placeholders. Install the draw.io desktop app to render diagrams to PDF.")
				})
				break
			}
		}
		return md, written
	}

	for _, att := range atts {
		if !isDrawio(att) {
			continue
		}
		newName := renameMap[att.Title]
		if newName == "" {
			continue
		}
		drawioPath := filepath.Join(dir, sanitizeFilename(newName))
		pdfName := pdfFilename(newName)
		pdfPath := filepath.Join(dir, sanitizeFilename(pdfName))

		if err := drawio.RenderToPDF(drawioPath, pdfPath); err != nil {
			bar.Log("warning: drawio render failed for %s (%v) — leaving source link", att.Title, err)
			continue
		}
		written = append(written, pdfPath)

		md = replaceDrawioPlaceholder(md, att.Title, newName, pdfName)
	}
	return md, written
}

// replaceDrawioPlaceholder rewrites the placeholder text emitted by the
// markdown converter for a drawio macro. The macro's diagramName parameter
// may include or omit the .drawio suffix, so we try both forms. The rendered
// PDF is referenced as a plain markdown link (not an image embed) since
// markdown viewers can't render PDF inline; the source .drawio is offered as
// a secondary link for editing.
func replaceDrawioPlaceholder(md, attTitle, drawioFilename, pdfFilename string) string {
	bare := strings.TrimSuffix(attTitle, ".drawio")
	embed := fmt.Sprintf("[%s (PDF)](%s) — [source](%s)", bare, pdfFilename, drawioFilename)
	for _, placeholder := range []string{
		fmt.Sprintf("*(diagram: %s)*", attTitle),
		fmt.Sprintf("*(diagram: %s)*", bare),
	} {
		md = strings.ReplaceAll(md, placeholder, embed)
	}
	return md
}

// isDrawio reports whether an attachment is a draw.io diagram.
func isDrawio(att api.Attachment) bool {
	if strings.HasSuffix(strings.ToLower(att.Title), ".drawio") {
		return true
	}
	return strings.Contains(strings.ToLower(att.MediaType), "drawio")
}

// pdfFilename derives the PDF sibling name for a drawio file. Preserves the
// renamed prefix (page-slug + file-id) so the PDF sorts next to its source.
func pdfFilename(drawioName string) string {
	base := strings.TrimSuffix(drawioName, ".drawio")
	if base == drawioName {
		return drawioName + ".pdf"
	}
	return base + ".pdf"
}
