package web

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jhash/tabitha/internal/db"
)

func TestBuildSongsQueryUsesRelevanceOrderWhenSearchPresentAndNoExplicitSort(t *testing.T) {
	query, args := buildSongsQuery(SongQueryParams{Search: "tiger"})
	if !strings.Contains(query, "similarity(title") {
		t.Errorf("query = %q, want it to rank by similarity when searching", query)
	}
	if len(args) != 3 {
		t.Fatalf("args = %v, want 3 (status, added_by, search term)", args)
	}
	if args[2] != "tiger" {
		t.Errorf("args[2] = %v, want the search term", args[2])
	}
}

func TestBuildSongsQueryExplicitSortWinsOverSearchRelevance(t *testing.T) {
	query, _ := buildSongsQuery(SongQueryParams{Search: "tiger", Sort: "artist", Order: "desc"})
	if strings.Contains(query, "similarity(title") {
		t.Errorf("query = %q, want explicit sort to override relevance ranking", query)
	}
	if !strings.Contains(query, "lower(artist) DESC") {
		t.Errorf("query = %q, want ORDER BY lower(artist) DESC", query)
	}
}

func TestBuildSongsQueryRejectsUnknownSortAndOrder(t *testing.T) {
	query, _ := buildSongsQuery(SongQueryParams{Sort: "'; DROP TABLE songs; --", Order: "sideways"})
	if !strings.Contains(query, "lower(title) ASC") {
		t.Errorf("query = %q, want fallback to lower(title) ASC for invalid sort/order", query)
	}
}

func TestListSongsQuerySearchRanksTitleAboveArtistAboveGenre(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	titleMatch, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Rocket Man", Artist: "Elton John"})
	if err != nil {
		t.Fatalf("seeding title match: %v", err)
	}
	artistMatch, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Rocketeers"})
	if err != nil {
		t.Fatalf("seeding artist match: %v", err)
	}
	genreMatch, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Yesterday", Artist: "The Beatles"})
	if err != nil {
		t.Fatalf("seeding genre match: %v", err)
	}
	genre, err := q.FindOrCreateGenre(ctx, "Rocket Rock")
	if err != nil {
		t.Fatalf("FindOrCreateGenre() error = %v", err)
	}
	if err := q.LinkSongGenre(ctx, db.LinkSongGenreParams{SongID: genreMatch.ID, GenreID: genre.ID}); err != nil {
		t.Fatalf("LinkSongGenre() error = %v", err)
	}

	rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{Search: "rocket"})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}

	idOrder := []int64{rows[0].ID, rows[1].ID, rows[2].ID}
	want := []int64{titleMatch.ID, artistMatch.ID, genreMatch.ID}
	if !equalInt64s(idOrder, want) {
		t.Errorf("order = %v, want title match, then artist match, then genre match: %v", idOrder, want)
	}
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuildSongsQueryFiltersOutUndigestedWhenHideUndigestedSet(t *testing.T) {
	query, args := buildSongsQuery(SongQueryParams{HideUndigested: true})
	if !strings.Contains(query, "current_version_id IS NOT NULL") {
		t.Errorf("query = %q, want it to filter on current_version_id when HideUndigested is set", query)
	}
	if len(args) != 2 {
		t.Fatalf("args = %v, want 2 (status, added_by) — HideUndigested shouldn't add a placeholder", args)
	}
}

func TestListSongsQueryHideUndigestedExcludesSongsWithoutAVersion(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	undigested, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Undigested"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	digested, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Digested"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: digested.ID, Kind: "primary", Source: "google_doc_scrape", RawText: "x", Content: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: digested.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{HideUndigested: true})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Title != "Digested" {
		t.Errorf("rows = %+v, want just Digested", rows)
	}
	_ = undigested
}

func TestListSongsQueryReportsHasVersion(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	undigested, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Undigested"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	digested, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Digested"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: digested.ID, Kind: "primary", Source: "google_doc_scrape", RawText: "x", Content: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: digested.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{Sort: "title"})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Title == "Undigested" && r.HasVersion {
			t.Errorf("Undigested song HasVersion = true, want false")
		}
		if r.Title == "Digested" && !r.HasVersion {
			t.Errorf("Digested song HasVersion = false, want true")
		}
	}
	_ = undigested
}

func TestListSongsQueryFiltersByStatus(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	if _, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Done Song", Status: "Done"}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if _, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "QC Song", Status: "Quality Check"}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{Status: "Done"})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Title != "Done Song" {
		t.Errorf("rows = %+v, want just Done Song", rows)
	}
}

func TestListSongsQueryFiltersByAddedBy(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	jeff, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jeff@tabitha.local", Name: "Jeff"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	jake, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	jeffSong, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Jeff's Song"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	// UpsertSongFromTOC's trigger defaults added_by to Jeff already, but
	// set both explicitly so the test doesn't depend on that default.
	if _, err := q.DB().Exec(ctx, "UPDATE songs SET added_by_user_id = $2 WHERE id = $1", jeffSong.ID, jeff.ID); err != nil {
		t.Fatalf("setting added_by: %v", err)
	}
	jakeSong, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Jake's Song"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if _, err := q.DB().Exec(ctx, "UPDATE songs SET added_by_user_id = $2 WHERE id = $1", jakeSong.ID, jake.ID); err != nil {
		t.Fatalf("setting added_by: %v", err)
	}

	rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{AddedBy: "Jake"})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Title != "Jake's Song" {
		t.Errorf("rows = %+v, want just Jake's Song", rows)
	}
}

func TestListSongsQuerySortToggleReversesOrder(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	if _, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa"}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if _, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Yesterday"}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	asc, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{Sort: "title", Order: "asc"})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	desc, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{Sort: "title", Order: "desc"})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}

	if len(asc) != 2 || len(desc) != 2 {
		t.Fatalf("got %d asc, %d desc rows, want 2 each", len(asc), len(desc))
	}
	if asc[0].Title != "Africa" || asc[1].Title != "Yesterday" {
		t.Errorf("asc order = %v, want [Africa, Yesterday]", []string{asc[0].Title, asc[1].Title})
	}
	if desc[0].Title != "Yesterday" || desc[1].Title != "Africa" {
		t.Errorf("desc order = %v, want [Yesterday, Africa]", []string{desc[0].Title, desc[1].Title})
	}
}

func TestListSongsQueryReportsDocTimestampsWhenSet(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	withDocTimes, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "With Doc Times"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	docCreated := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	docModified := time.Date(2021, 3, 4, 0, 0, 0, 0, time.UTC)
	if err := q.SetSongDocTimestamps(ctx, db.SetSongDocTimestampsParams{
		ID:            withDocTimes.ID,
		DocCreatedAt:  pgtype.Timestamptz{Time: docCreated, Valid: true},
		DocModifiedAt: pgtype.Timestamptz{Time: docModified, Valid: true},
	}); err != nil {
		t.Fatalf("SetSongDocTimestamps() error = %v", err)
	}

	without, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Without Doc Times"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{Sort: "title"})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	var withRow, withoutRow SongRow
	for _, r := range rows {
		if r.ID == withDocTimes.ID {
			withRow = r
		}
		if r.ID == without.ID {
			withoutRow = r
		}
	}
	if withRow.DocCreatedAt == nil || !withRow.DocCreatedAt.Equal(docCreated) {
		t.Errorf("withRow.DocCreatedAt = %v, want %v", withRow.DocCreatedAt, docCreated)
	}
	if withRow.DocModifiedAt == nil || !withRow.DocModifiedAt.Equal(docModified) {
		t.Errorf("withRow.DocModifiedAt = %v, want %v", withRow.DocModifiedAt, docModified)
	}
	if withoutRow.DocCreatedAt != nil || withoutRow.DocModifiedAt != nil {
		t.Errorf("withoutRow doc timestamps = %v/%v, want both nil", withoutRow.DocCreatedAt, withoutRow.DocModifiedAt)
	}
}
