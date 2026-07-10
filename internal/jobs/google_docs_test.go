package jobs

import (
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
