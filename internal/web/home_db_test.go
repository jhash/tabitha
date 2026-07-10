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

	if _, err := pool.Exec(ctx, "TRUNCATE songs, transcription_versions, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncating test db: %v", err)
	}
	return db.New(pool)
}

func TestListSongsSortedEachSortColumnReturnsAllSeededSongs(t *testing.T) {
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
		rows, resolvedSort, err := listSongsSorted(ctx, q, sort)
		if err != nil {
			t.Fatalf("listSongsSorted(%q) error = %v", sort, err)
		}
		if resolvedSort != sort {
			t.Errorf("listSongsSorted(%q) resolved sort = %q", sort, resolvedSort)
		}
		if len(rows) != 2 {
			t.Errorf("listSongsSorted(%q) returned %d rows, want 2", sort, len(rows))
		}
	}
}

func TestListSongsSortedFallsBackToTitleForUnknownSort(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	if _, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Zzz", Artist: "A"}); err != nil {
		t.Fatalf("seeding song: %v", err)
	}

	_, resolvedSort, err := listSongsSorted(ctx, q, "'; DROP TABLE songs; --")
	if err != nil {
		t.Fatalf("listSongsSorted() error = %v", err)
	}
	if resolvedSort != "title" {
		t.Errorf("resolvedSort = %q, want fallback to title", resolvedSort)
	}
}
