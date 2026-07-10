package jobs

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/riverqueue/river"

	"github.com/jhash/tabitha/internal/db"
)

// tocSheetURL is Jeffrey's table-of-contents Google Sheet, exported as CSV.
// This works unauthenticated for cell values (confirmed directly) — only
// per-row Google Doc hyperlinks require the Sheets API + a stored OAuth
// token, which TocSyncWorker doesn't yet have (see design doc Phase 2).
const tocSheetURL = "https://docs.google.com/spreadsheets/d/1uqJfZ7TyH-Ii_dJGvby6MH-uVyH0xFkG7RQ3bxARezQ/export?format=csv&gid=0"

// TocSyncArgs triggers a table-of-contents sync: fetch the sheet, upsert
// every row into songs. Takes no parameters — there's only one sheet.
type TocSyncArgs struct{}

func (TocSyncArgs) Kind() string { return "toc_sync" }

type TocSyncWorker struct {
	river.WorkerDefaults[TocSyncArgs]
	Queries    *db.Queries
	HTTPClient *http.Client

	// SheetURL overrides tocSheetURL. Left empty in production; tests point
	// it at an httptest.Server instead of the real live sheet.
	SheetURL string
}

func (w *TocSyncWorker) Work(ctx context.Context, job *river.Job[TocSyncArgs]) error {
	client := w.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	url := w.SheetURL
	if url == "" {
		url = tocSheetURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("toc_sync: building request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("toc_sync: fetching sheet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("toc_sync: sheet fetch returned status %d", resp.StatusCode)
	}

	rows, err := parseTOCCSV(resp.Body)
	if err != nil {
		return fmt.Errorf("toc_sync: parsing sheet: %w", err)
	}

	for _, row := range rows {
		if _, err := w.Queries.UpsertSongFromTOC(ctx, row); err != nil {
			return fmt.Errorf("toc_sync: upserting %q by %q: %w", row.Title, row.Artist, err)
		}
	}

	return nil
}

// tocColumns are the sheet's real headers, confirmed directly against the
// live document. Matched case-sensitively, trimmed of surrounding space.
var tocColumns = []string{
	"TITLE", "ARTIST", "GENRE", "FILM/SHOW/ALBUM", "DECADE", "BOB",
	"STATUS", "SCRAPE LINK", "Notes", "TRANSPOSE",
}

// parseTOCCSV reads the table-of-contents export and maps columns by header
// name (rather than position) so a reordered sheet doesn't silently corrupt
// data. Rows with no title are skipped.
func parseTOCCSV(r io.Reader) ([]db.UpsertSongFromTOCParams, error) {
	reader := csv.NewReader(r)

	header, err := reader.Read()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading header row: %w", err)
	}

	colIndex := make(map[string]int, len(header))
	for i, name := range header {
		colIndex[strings.TrimSpace(name)] = i
	}

	get := func(record []string, column string) string {
		i, ok := colIndex[column]
		if !ok || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}

	var rows []db.UpsertSongFromTOCParams
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}

		title := get(record, "TITLE")
		if title == "" {
			continue
		}

		rows = append(rows, db.UpsertSongFromTOCParams{
			Title:         title,
			Artist:        get(record, "ARTIST"),
			Genre:         get(record, "GENRE"),
			FilmShowAlbum: get(record, "FILM/SHOW/ALBUM"),
			Decade:        get(record, "DECADE"),
			BobTag:        get(record, "BOB"),
			Status:        get(record, "STATUS"),
			SourceUrl:     get(record, "SCRAPE LINK"),
			Notes:         get(record, "Notes"),
			TransposeHint: get(record, "TRANSPOSE"),
		})
	}

	return rows, nil
}
