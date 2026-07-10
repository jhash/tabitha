package web

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

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

	// internal/auth's test package truncates overlapping tables (users, and
	// anything CASCADE reaches from it) concurrently in its own process
	// when `go test ./...` runs packages in parallel. Two concurrent
	// TRUNCATEs over overlapping table sets can lock-order-invert and
	// deadlock, so serialize with a transaction-scoped advisory lock
	// (shared key with internal/auth's copy of this helper) instead of
	// truncating directly on the pool.
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

func TestListSongsQueryEachSortColumnReturnsAllSeededSongs(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	for _, s := range []db.UpsertSongFromTOCParams{
		{Title: "Africa", Artist: "Toto", Status: "Done"},
		{Title: "Yesterday", Artist: "The Beatles", Status: "Quality Check"},
	} {
		if _, err := q.UpsertSongFromTOC(ctx, s); err != nil {
			t.Fatalf("seeding song: %v", err)
		}
	}

	for _, sort := range sortColumns {
		rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{Sort: sort})
		if err != nil {
			t.Fatalf("ListSongsQuery(sort=%q) error = %v", sort, err)
		}
		if len(rows) != 2 {
			t.Errorf("ListSongsQuery(sort=%q) returned %d rows, want 2", sort, len(rows))
		}
	}
}

func TestListSongsQueryFallsBackToTitleForUnknownSort(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	if _, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Zzz", Artist: "A"}); err != nil {
		t.Fatalf("seeding song: %v", err)
	}

	rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{Sort: "'; DROP TABLE songs; --"})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Title != "Zzz" {
		t.Errorf("rows = %+v, want the seeded song still returned (fallback to title sort)", rows)
	}
}
