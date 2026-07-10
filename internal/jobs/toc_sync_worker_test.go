package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
	if _, err := tx.Exec(ctx, "TRUNCATE songs, transcription_versions, users RESTART IDENTITY CASCADE"); err != nil {
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

