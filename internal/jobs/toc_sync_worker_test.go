package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/jhash/tabitha/internal/db"
)

func setupTestQueries(t *testing.T) *db.Queries {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres:///tabitha_test?sslmode=disable"
	}

	if err := db.MigrateUp(url); err != nil {
		t.Fatalf("migrating test db: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(pool.Close)

	// internal/auth and internal/web run their own TRUNCATEs over
	// overlapping tables concurrently (separate `go test ./...` package
	// processes, same shared test DB). Serialize with a transaction-scoped
	// advisory lock (shared key across all three copies of this helper) to
	// avoid a cross-process deadlock.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("beginning truncate tx: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(72469)"); err != nil {
		t.Fatalf("acquiring test db truncate lock: %v", err)
	}
	if _, err := tx.Exec(ctx, "TRUNCATE songs, transcription_versions, users, artists, genres RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncating test db: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("committing truncate tx: %v", err)
	}

	return db.New(pool)
}

func TestTocSyncWorkerUpsertsRowsFromHTTPResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(tocFixtureCSV))
	}))
	defer server.Close()

	q := setupTestQueries(t)
	worker := &TocSyncWorker{Queries: q, HTTPClient: server.Client(), SheetURL: server.URL}

	job := &river.Job[TocSyncArgs]{JobRow: &rivertype.JobRow{}, Args: TocSyncArgs{}}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("Work() error = %v", err)
	}

	songs, err := q.ListSongsByTitle(context.Background())
	if err != nil {
		t.Fatalf("ListSongsByTitle() error = %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("got %d songs, want 2", len(songs))
	}
}

func TestTocSyncWorkerAssignsSlugsToNewSongs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(tocFixtureCSV))
	}))
	defer server.Close()

	q := setupTestQueries(t)
	worker := &TocSyncWorker{Queries: q, HTTPClient: server.Client(), SheetURL: server.URL}

	job := &river.Job[TocSyncArgs]{JobRow: &rivertype.JobRow{}, Args: TocSyncArgs{}}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("Work() error = %v", err)
	}

	song, err := q.GetSongByTitle(context.Background(), "(I Can't Get No) Satisfaction")
	if err != nil {
		t.Fatalf("GetSongByTitle() error = %v", err)
	}
	if song.Slug != "i-cant-get-no-satisfaction" {
		t.Errorf("Slug = %q, want %q", song.Slug, "i-cant-get-no-satisfaction")
	}
}

func TestTocSyncWorkerAppendsArtistSlugOnTitleCollision(t *testing.T) {
	const csv = `TITLE,ARTIST,GENRE,FILM/SHOW/ALBUM,DECADE,BOB,STATUS,SCRAPE LINK,Notes,TRANSPOSE
Yesterday,The Beatles,,,1960s,,Done,,,
Yesterday,Boyz II Men,,,1990s,,Done,,,
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(csv))
	}))
	defer server.Close()

	q := setupTestQueries(t)
	worker := &TocSyncWorker{Queries: q, HTTPClient: server.Client(), SheetURL: server.URL}

	job := &river.Job[TocSyncArgs]{JobRow: &rivertype.JobRow{}, Args: TocSyncArgs{}}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("Work() error = %v", err)
	}

	beatles, err := q.GetSongByTitle(context.Background(), "Yesterday")
	if err != nil {
		t.Fatalf("GetSongByTitle() error = %v", err)
	}
	_ = beatles

	all, err := q.ListAllSongSlugs(context.Background())
	if err != nil {
		t.Fatalf("ListAllSongSlugs() error = %v", err)
	}
	slugs := make(map[string]bool)
	for _, s := range all {
		slugs[s.Slug] = true
	}
	if !slugs["yesterday"] {
		t.Errorf("expected one song to keep the plain %q slug, got %v", "yesterday", slugs)
	}
	if !slugs["yesterday-boyz-ii-men"] && !slugs["yesterday-the-beatles"] {
		t.Errorf("expected the colliding song to get an artist-suffixed slug, got %v", slugs)
	}
}

func TestTocSyncWorkerLinksArtistAndGenre(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(tocFixtureCSV))
	}))
	defer server.Close()

	q := setupTestQueries(t)
	worker := &TocSyncWorker{Queries: q, HTTPClient: server.Client(), SheetURL: server.URL}

	job := &river.Job[TocSyncArgs]{JobRow: &rivertype.JobRow{}, Args: TocSyncArgs{}}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatalf("Work() error = %v", err)
	}

	ctx := context.Background()
	song, err := q.GetSongByTitle(ctx, "(I Can't Get No) Satisfaction")
	if err != nil {
		t.Fatalf("GetSongByTitle() error = %v", err)
	}
	if song.ArtistID == nil {
		t.Fatal("ArtistID is nil, want it linked to an artists row")
	}

	genres, err := q.ListGenresForSong(ctx, song.ID)
	if err != nil {
		t.Fatalf("ListGenresForSong() error = %v", err)
	}
	if len(genres) != 1 || genres[0].Name != "Classic Rock" {
		t.Errorf("genres = %+v, want a single \"Classic Rock\" genre", genres)
	}
}

func TestTocSyncWorkerReplacesGenreLinkOnResync(t *testing.T) {
	// A song's genre in the sheet can change between syncs — the old
	// link shouldn't linger alongside the new one.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(tocFixtureCSV))
	}))
	defer server.Close()

	q := setupTestQueries(t)
	worker := &TocSyncWorker{Queries: q, HTTPClient: server.Client(), SheetURL: server.URL}
	ctx := context.Background()

	job := &river.Job[TocSyncArgs]{JobRow: &rivertype.JobRow{}, Args: TocSyncArgs{}}
	if err := worker.Work(ctx, job); err != nil {
		t.Fatalf("Work() error = %v", err)
	}

	updatedCSV := strings.Replace(tocFixtureCSV, "Classic Rock", "Rock", 1)
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(updatedCSV))
	}))
	defer server2.Close()
	worker.HTTPClient = server2.Client()
	worker.SheetURL = server2.URL

	if err := worker.Work(ctx, job); err != nil {
		t.Fatalf("second Work() error = %v", err)
	}

	song, err := q.GetSongByTitle(ctx, "(I Can't Get No) Satisfaction")
	if err != nil {
		t.Fatalf("GetSongByTitle() error = %v", err)
	}
	genres, err := q.ListGenresForSong(ctx, song.ID)
	if err != nil {
		t.Fatalf("ListGenresForSong() error = %v", err)
	}
	if len(genres) != 1 || genres[0].Name != "Rock" {
		t.Errorf("genres = %+v, want just the updated \"Rock\" genre", genres)
	}
}

