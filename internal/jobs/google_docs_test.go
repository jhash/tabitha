package jobs

import (
	"strings"
	"testing"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/sheets/v4"
)

func TestExtractDocIDFromHyperlinkParsesStandardDocURL(t *testing.T) {
	got, err := extractDocIDFromHyperlink("https://docs.google.com/document/d/1AbC-XyZ_123/edit")
	if err != nil {
		t.Fatalf("extractDocIDFromHyperlink() error = %v", err)
	}
	if got != "1AbC-XyZ_123" {
		t.Errorf("got %q, want %q", got, "1AbC-XyZ_123")
	}
}

func TestExtractDocIDFromHyperlinkHandlesTrailingQueryAndFragment(t *testing.T) {
	got, err := extractDocIDFromHyperlink("https://docs.google.com/document/d/1AbC-XyZ_123/edit?usp=sharing#heading=h.abc")
	if err != nil {
		t.Fatalf("extractDocIDFromHyperlink() error = %v", err)
	}
	if got != "1AbC-XyZ_123" {
		t.Errorf("got %q, want %q", got, "1AbC-XyZ_123")
	}
}

func TestExtractDocIDFromHyperlinkRejectsNonDocURL(t *testing.T) {
	if _, err := extractDocIDFromHyperlink("https://example.com/not-a-doc"); err == nil {
		t.Error("expected an error for a URL that isn't a Google Doc link")
	}
}

func TestFindHyperlinkForTitleMatchesCaseInsensitively(t *testing.T) {
	spreadsheet := &sheets.Spreadsheet{
		Sheets: []*sheets.Sheet{
			{
				Data: []*sheets.GridData{
					{
						RowData: []*sheets.RowData{
							{Values: []*sheets.CellData{{FormattedValue: "Title"}, {FormattedValue: "Artist"}}},
							{Values: []*sheets.CellData{
								{FormattedValue: "Great Balls of Fire", Hyperlink: "https://docs.google.com/document/d/doc-123/edit"},
								{FormattedValue: "Jerry Lee Lewis"},
							}},
						},
					},
				},
			},
		},
	}

	got, ok := findHyperlinkForTitle(spreadsheet, "great balls of fire")
	if !ok {
		t.Fatal("expected a match")
	}
	if got != "https://docs.google.com/document/d/doc-123/edit" {
		t.Errorf("got %q", got)
	}
}

func TestFindHyperlinkForTitleReturnsFalseWhenNoMatch(t *testing.T) {
	spreadsheet := &sheets.Spreadsheet{
		Sheets: []*sheets.Sheet{
			{
				Data: []*sheets.GridData{
					{
						RowData: []*sheets.RowData{
							{Values: []*sheets.CellData{{FormattedValue: "Title"}}},
							{Values: []*sheets.CellData{{FormattedValue: "Africa"}}},
						},
					},
				},
			},
		},
	}

	if _, ok := findHyperlinkForTitle(spreadsheet, "Great Balls of Fire"); ok {
		t.Error("expected no match")
	}
}

func TestFindHyperlinkForTitleFindsTitleColumnByHeaderName(t *testing.T) {
	// Title isn't necessarily column A — same convention as the CSV parser,
	// which maps columns by header name rather than fixed position.
	spreadsheet := &sheets.Spreadsheet{
		Sheets: []*sheets.Sheet{
			{
				Data: []*sheets.GridData{
					{
						RowData: []*sheets.RowData{
							{Values: []*sheets.CellData{{FormattedValue: "Artist"}, {FormattedValue: "Title"}}},
							{Values: []*sheets.CellData{
								{FormattedValue: "Jerry Lee Lewis"},
								{FormattedValue: "Great Balls of Fire", Hyperlink: "https://docs.google.com/document/d/doc-123/edit"},
							}},
						},
					},
				},
			},
		},
	}

	got, ok := findHyperlinkForTitle(spreadsheet, "Great Balls of Fire")
	if !ok {
		t.Fatal("expected a match")
	}
	if got != "https://docs.google.com/document/d/doc-123/edit" {
		t.Errorf("got %q", got)
	}
}

func TestDocTextFromGoogleDocConcatenatesParagraphTextRuns(t *testing.T) {
	doc := &docs.Document{
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{Paragraph: &docs.Paragraph{Elements: []*docs.ParagraphElement{
					{TextRun: &docs.TextRun{Content: "Great Balls of Fire\n"}},
				}}},
				{Paragraph: &docs.Paragraph{Elements: []*docs.ParagraphElement{
					{TextRun: &docs.TextRun{Content: "As performed by: Jerry Lee Lewis\n"}},
				}}},
			},
		},
	}

	got := docTextFromGoogleDoc(doc)
	want := "Great Balls of Fire\nAs performed by: Jerry Lee Lewis\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDocTextFromGoogleDocSkipsNonTextStructuralElements(t *testing.T) {
	// A page break contributes no text of its own — Jeff's transpose
	// workflow uses one to separate two keys in the same doc, but
	// splitting on it is digestion-level work, not this function's job.
	doc := &docs.Document{
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{Paragraph: &docs.Paragraph{Elements: []*docs.ParagraphElement{
					{TextRun: &docs.TextRun{Content: "before\n"}},
				}}},
				{Paragraph: &docs.Paragraph{Elements: []*docs.ParagraphElement{
					{PageBreak: &docs.PageBreak{}},
				}}},
				{Paragraph: &docs.Paragraph{Elements: []*docs.ParagraphElement{
					{TextRun: &docs.TextRun{Content: "after\n"}},
				}}},
			},
		},
	}

	got := docTextFromGoogleDoc(doc)
	want := "before\nafter\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDocTextFromGoogleDocHandlesNilBody(t *testing.T) {
	if got := docTextFromGoogleDoc(&docs.Document{}); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestDocSectionsFromGoogleDocSplitsOnLongBlankRun(t *testing.T) {
	// Jeff doesn't use a real "insert page break" — he mashes Enter to push
	// the original key to a new page. Confirmed against the real Eye of the
	// Tiger doc: ~50 consecutive blank lines at the real gap, vs. a max of
	// 2 in a row anywhere else in either real fixture doc. So a long blank
	// run (not any single blank line, which separates verses normally) is
	// the actual delimiter.
	text := "new key\n" + strings.Repeat("\n", 6) + "original key\n"
	got := docSectionsFromText(text)
	// The discarded (new-key) section's exact trailing whitespace doesn't
	// matter — only the kept section (last, see originalKeySection) needs
	// to round-trip cleanly through the parser.
	want := []string{"new key", "original key\n"}
	if len(got) != len(want) {
		t.Fatalf("got %d sections %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDocSectionsFromTextIgnoresNormalBlankLines(t *testing.T) {
	text := "verse one\n\nverse two\n\nverse three\n"
	got := docSectionsFromText(text)
	if len(got) != 1 || got[0] != text {
		t.Errorf("got %q, want one section %q", got, text)
	}
}

func TestDocSectionsFromTextHandlesEmptyString(t *testing.T) {
	got := docSectionsFromText("")
	if len(got) != 1 || got[0] != "" {
		t.Errorf("got %q, want one empty section", got)
	}
}

func TestOriginalKeySectionReturnsLastSection(t *testing.T) {
	if got := originalKeySection([]string{"new key\n", "original key\n"}); got != "original key\n" {
		t.Errorf("got %q, want %q", got, "original key\n")
	}
}

func TestOriginalKeySectionHandlesSingleSection(t *testing.T) {
	if got := originalKeySection([]string{"only key\n"}); got != "only key\n" {
		t.Errorf("got %q, want %q", got, "only key\n")
	}
}
