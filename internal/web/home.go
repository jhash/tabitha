package web

import (
	"context"
	"fmt"
	"net/http"
	"time"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/db"
)

// SongRow is the view-model for one row of the home page table — a
// flattened, plain-Go-typed projection of whichever ListSongsBy* row shape
// the active sort produced (sqlc generates a distinct struct per query since
// it can't parametrize ORDER BY; this is the seam that papers over that).
type SongRow struct {
	ID           int64
	Title        string
	Artist       string
	Status       string
	AddedByName  string
	AddedByEmail string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var sortColumns = []string{"title", "artist", "updated", "added", "status", "added_by"}

func isValidSort(sort string) bool {
	for _, c := range sortColumns {
		if c == sort {
			return true
		}
	}
	return false
}

// listSongsSorted runs the ListSongsBy* query matching sort (defaulting to
// title for anything unrecognized) and flattens the result to []SongRow.
func listSongsSorted(ctx context.Context, q *db.Queries, sort string) ([]SongRow, string, error) {
	if !isValidSort(sort) {
		sort = "title"
	}

	switch sort {
	case "artist":
		rows, err := q.ListSongsByArtist(ctx)
		return mapSongRows(rows, func(r db.ListSongsByArtistRow) SongRow {
			return SongRow{r.ID, r.Title, r.Artist, r.Status, deref(r.AddedByName), deref(r.AddedByEmail), r.CreatedAt.Time, r.UpdatedAt.Time}
		}), sort, err
	case "updated":
		rows, err := q.ListSongsByLastUpdated(ctx)
		return mapSongRows(rows, func(r db.ListSongsByLastUpdatedRow) SongRow {
			return SongRow{r.ID, r.Title, r.Artist, r.Status, deref(r.AddedByName), deref(r.AddedByEmail), r.CreatedAt.Time, r.UpdatedAt.Time}
		}), sort, err
	case "added":
		rows, err := q.ListSongsByRecentlyAdded(ctx)
		return mapSongRows(rows, func(r db.ListSongsByRecentlyAddedRow) SongRow {
			return SongRow{r.ID, r.Title, r.Artist, r.Status, deref(r.AddedByName), deref(r.AddedByEmail), r.CreatedAt.Time, r.UpdatedAt.Time}
		}), sort, err
	case "status":
		rows, err := q.ListSongsByStatus(ctx)
		return mapSongRows(rows, func(r db.ListSongsByStatusRow) SongRow {
			return SongRow{r.ID, r.Title, r.Artist, r.Status, deref(r.AddedByName), deref(r.AddedByEmail), r.CreatedAt.Time, r.UpdatedAt.Time}
		}), sort, err
	case "added_by":
		rows, err := q.ListSongsByAddedBy(ctx)
		return mapSongRows(rows, func(r db.ListSongsByAddedByRow) SongRow {
			return SongRow{r.ID, r.Title, r.Artist, r.Status, deref(r.AddedByName), deref(r.AddedByEmail), r.CreatedAt.Time, r.UpdatedAt.Time}
		}), sort, err
	default: // "title"
		rows, err := q.ListSongsByTitle(ctx)
		return mapSongRows(rows, func(r db.ListSongsByTitleRow) SongRow {
			return SongRow{r.ID, r.Title, r.Artist, r.Status, deref(r.AddedByName), deref(r.AddedByEmail), r.CreatedAt.Time, r.UpdatedAt.Time}
		}), sort, err
	}
}

func mapSongRows[T any](rows []T, fn func(T) SongRow) []SongRow {
	out := make([]SongRow, len(rows))
	for i, r := range rows {
		out[i] = fn(r)
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// HomeHandler renders the public table of contents: every song, sortable by
// title, artist, status, last updated, most recently added, or added-by.
func HomeHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		songs, sort, err := listSongsSorted(r.Context(), q, r.URL.Query().Get("sort"))
		if err != nil {
			http.Error(w, "failed to load songs", http.StatusInternalServerError)
			return
		}

		page := Page("Songs", "Jeff's music transcription catalog", nil, homeTable(songs, sort))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func homeTable(songs []SongRow, sort string) g.Node {
	return Table(
		THead(
			Tr(
				sortHeader("Title", "title", sort),
				sortHeader("Artist", "artist", sort),
				sortHeader("Status", "status", sort),
				sortHeader("Last Updated", "updated", sort),
				sortHeader("Added", "added", sort),
				sortHeader("Added By", "added_by", sort),
			),
		),
		TBody(
			g.Map(songs, func(s SongRow) g.Node {
				return Tr(
					Td(A(Href(fmt.Sprintf("/songs/%d", s.ID)), g.Text(s.Title))),
					Td(g.Text(s.Artist)),
					Td(g.Text(s.Status)),
					Td(g.Text(formatDate(s.UpdatedAt))),
					Td(g.Text(formatDate(s.CreatedAt))),
					Td(g.Text(addedByLabel(s))),
				)
			}),
		),
	)
}

func addedByLabel(s SongRow) string {
	if s.AddedByName != "" {
		return s.AddedByName
	}
	return s.AddedByEmail
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func sortHeader(label, column, activeSort string) g.Node {
	return Th(A(Href("/?sort="+column), g.Attr("hx-get", "/?sort="+column), g.Attr("hx-push-url", "true"), g.Attr("hx-target", "body"),
		g.Text(label),
		g.If(column == activeSort, g.Text(" ▾")),
	))
}
