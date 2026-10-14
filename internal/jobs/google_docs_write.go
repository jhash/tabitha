package jobs

import (
	"fmt"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
)

// createGoogleDoc creates a brand-new Google Doc titled title, containing
// text as its entire body, and returns its Drive file ID. Used only for
// docs tabitha creates itself (see auth.GoogleDriveFileScope) — never for
// modifying a doc that already exists.
func createGoogleDoc(driveSvc *drive.Service, docsSvc *docs.Service, title, text string) (string, error) {
	file, err := driveSvc.Files.Create(&drive.File{
		Name:     title,
		MimeType: "application/vnd.google-apps.document",
	}).Fields("id").Do()
	if err != nil {
		return "", fmt.Errorf("creating drive file: %w", err)
	}

	if text == "" {
		return file.Id, nil
	}
	if _, err := docsSvc.Documents.BatchUpdate(file.Id, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{insertTextRequest(1, text)},
	}).Do(); err != nil {
		return "", fmt.Errorf("inserting content into new doc %s: %w", file.Id, err)
	}
	return file.Id, nil
}

// replaceGoogleDocContent overwrites docID's entire body with text. Only
// ever called on a docID tabitha itself stored on the song (i.e. a doc
// createGoogleDoc made), never on an arbitrary externally-supplied ID.
func replaceGoogleDocContent(docsSvc *docs.Service, docID, text string) error {
	doc, err := docsSvc.Documents.Get(docID).Fields("body(content(endIndex))").Do()
	if err != nil {
		return fmt.Errorf("fetching doc %s: %w", docID, err)
	}

	var requests []*docs.Request
	if endIndex := bodyEndIndex(doc); endIndex > 1 {
		requests = append(requests, &docs.Request{
			DeleteContentRange: &docs.DeleteContentRangeRequest{
				Range: &docs.Range{StartIndex: 1, EndIndex: endIndex - 1},
			},
		})
	}
	if text != "" {
		requests = append(requests, insertTextRequest(1, text))
	}
	if len(requests) == 0 {
		return nil
	}

	if _, err := docsSvc.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Do(); err != nil {
		return fmt.Errorf("replacing content in doc %s: %w", docID, err)
	}
	return nil
}

// bodyEndIndex returns a doc's body's final structural element's
// endIndex — the Docs API convention for "one past the last character,
// including the document's own trailing implicit newline, which can
// never be deleted." A doc with only that implicit newline has
// endIndex == 1, i.e. already empty.
func bodyEndIndex(doc *docs.Document) int64 {
	if doc.Body == nil || len(doc.Body.Content) == 0 {
		return 1
	}
	return doc.Body.Content[len(doc.Body.Content)-1].EndIndex
}

func insertTextRequest(index int64, text string) *docs.Request {
	return &docs.Request{
		InsertText: &docs.InsertTextRequest{
			Location: &docs.Location{Index: index},
			Text:     text,
		},
	}
}
