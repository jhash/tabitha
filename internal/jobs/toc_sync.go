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

// tocSpreadsheetID is Jeffrey's table-of-contents Google Sheet.
const tocSpreadsheetID = "1uqJfZ7TyH-Ii_dJGvby6MH-uVyH0xFkG7RQ3bxARezQ"

// tocSheetURL is the sheet's CSV export. This works unauthenticated for
// cell values (confirmed directly) — only per-row Google Doc hyperlinks
// require the Sheets API + a stored OAuth token (see google_docs.go, used
// by DigestSongWorker).
const tocSheetURL = "https://docs.google.com/spreadsheets/d/" + tocSpreadsheetID + "/export?format=csv&gid=0"

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
		song, err := w.Queries.UpsertSongFromTOC(ctx, row)
		if err != nil {
			return fmt.Errorf("toc_sync: upserting %q by %q: %w", row.Title, row.Artist, err)
		}
		if err := w.linkArtistAndGenre(ctx, song, row); err != nil {
			return fmt.Errorf("toc_sync: linking artist/genre for %q: %w", row.Title, err)
		}
	}

	return nil
}

// linkArtistAndGenre normalizes the sheet's free-text ARTIST/GENRE columns
// into the artists/genres tables, best-effort. songs.artist/songs.genre
// stay the source of truth for display and the TOC dedup key — these are
// derived links, re-derived on every sync so a changed genre in the sheet
// doesn't leave a stale link behind.
func (w *TocSyncWorker) linkArtistAndGenre(ctx context.Context, song db.Song, row db.UpsertSongFromTOCParams) error {
	if row.Artist != "" {
		artist, err := w.Queries.FindOrCreateArtist(ctx, row.Artist)
		if err != nil {
			return fmt.Errorf("finding/creating artist %q: %w", row.Artist, err)
		}
		if err := w.Queries.SetSongArtistID(ctx, db.SetSongArtistIDParams{ID: song.ID, ArtistID: &artist.ID}); err != nil {
			return fmt.Errorf("setting artist_id: %w", err)
		}
	}

	if err := w.Queries.ClearSongGenres(ctx, song.ID); err != nil {
		return fmt.Errorf("clearing prior genre links: %w", err)
	}
	if row.Genre != "" {
		genre, err := w.Queries.FindOrCreateGenre(ctx, row.Genre)
		if err != nil {
			return fmt.Errorf("finding/creating genre %q: %w", row.Genre, err)
		}
		if err := w.Queries.LinkSongGenre(ctx, db.LinkSongGenreParams{SongID: song.ID, GenreID: genre.ID}); err != nil {
			return fmt.Errorf("linking genre: %w", err)
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
