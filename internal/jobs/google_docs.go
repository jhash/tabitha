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
// newline (a Docs API convention), so this is mostly concatenation — no
// non-text structural element (page breaks, section breaks, tables)
// contributes any text of its own. One exception: a soft line break
// (Shift+Enter, used to group e.g. a chord line with its lyric line
// within one paragraph) comes through as a literal \v (0x0B), which
// browsers don't render as a line break — confirmed against the real
// "Don't Stop Believin'" doc, translated to \n here.
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
	return strings.ReplaceAll(b.String(), "\v", "\n")
}

// blankRunSplitRe matches a run of at least 6 blank lines. Jeff's
// transpose workflow doesn't use a real "insert page break" — he just
// mashes Enter to push the original key onto a new page — so the visible
// gap shows up as one long run of blank lines, not a PageBreak structural
// element. Confirmed against the real Eye of the Tiger doc: ~50
// consecutive blank lines at the actual gap vs. a max of 2 in a row
// anywhere else in either real fixture doc, so 6 has a wide margin against
// false splits on ordinary verse spacing.
var blankRunSplitRe = regexp.MustCompile(`\n{7,}`)

// docSectionsFromText splits flattened doc text on Jeff's transpose-
// workflow page gap. A doc with no such gap is a single section. Always
// returns at least one element (possibly empty), so callers never need a
// length check.
func docSectionsFromText(text string) []string {
	return blankRunSplitRe.Split(text, -1)
}

// docSectionsFromGoogleDoc splits a doc's flattened text on Jeff's
// transpose-workflow page gap (see docSectionsFromText).
func docSectionsFromGoogleDoc(doc *docs.Document) []string {
	return docSectionsFromText(docTextFromGoogleDoc(doc))
}

// originalKeySection returns the section holding the original key's
// transcription. Per Jeff, the original key follows the new key after the
// page break, so it's the last section — confirmed with Jake rather than
// assumed (see docs/jeff-domain-notes.md). Docs with no page break have
// exactly one section, which is trivially "original."
func originalKeySection(sections []string) string {
	return sections[len(sections)-1]
}
