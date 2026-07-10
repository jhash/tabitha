package jobs

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/sheets/v4"
)

// docURLIDRe extracts the document ID from a standard Google Docs URL,
// e.g. https://docs.google.com/document/d/<ID>/edit?usp=sharing#heading=h.x
var docURLIDRe = regexp.MustCompile(`^https://docs\.google\.com/document/d/([^/]+)/`)

// extractDocIDFromHyperlink pulls the document ID out of a title cell's
// hyperlink (the TOC sheet stores a real "insert link" hyperlink on the
// title, not a formula — confirmed directly against the sheet).
func extractDocIDFromHyperlink(rawURL string) (string, error) {
	m := docURLIDRe.FindStringSubmatch(rawURL)
	if m == nil {
		return "", fmt.Errorf("google_docs: %q doesn't look like a Google Doc URL", rawURL)
	}
	return m[1], nil
}

// findHyperlinkForTitle scans a spreadsheet's grid data for a row whose
// Title-column cell matches title (case-insensitively), returning that
// cell's hyperlink. The title column is found by header name, same
// convention as the CSV parser in toc_sync.go, since it isn't necessarily
// column A.
func findHyperlinkForTitle(spreadsheet *sheets.Spreadsheet, title string) (string, bool) {
	want := strings.ToLower(strings.TrimSpace(title))

	for _, sheet := range spreadsheet.Sheets {
		for _, grid := range sheet.Data {
			titleCol := -1
			for _, row := range grid.RowData {
				if titleCol == -1 {
					for i, cell := range row.Values {
						if strings.EqualFold(strings.TrimSpace(cell.FormattedValue), "Title") {
							titleCol = i
							break
						}
					}
					continue
				}
				if titleCol >= len(row.Values) {
					continue
				}
				cell := row.Values[titleCol]
				if strings.ToLower(strings.TrimSpace(cell.FormattedValue)) == want {
					return cell.Hyperlink, true
				}
			}
		}
	}
	return "", false
}

// docTextFromGoogleDoc flattens a fetched Google Doc into plain text.
// Paragraph TextRun content already includes the paragraph's trailing
// newline (a Docs API convention), so this is just concatenation — no
// non-text structural element (page breaks, section breaks, tables)
// contributes any text of its own. Detecting Jeff's transpose-workflow
// page break to split one doc into multiple transcription_versions rows
// is future digestion-level work (see docs/jeff-domain-notes.md), not
// this function's job.
func docTextFromGoogleDoc(doc *docs.Document) string {
	if doc.Body == nil {
		return ""
	}
	var b strings.Builder
	for _, se := range doc.Body.Content {
		if se.Paragraph == nil {
			continue
		}
		for _, el := range se.Paragraph.Elements {
			if el.TextRun != nil {
				b.WriteString(el.TextRun.Content)
			}
		}
	}
	return b.String()
}
