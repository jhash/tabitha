package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres:///tabitha_test?sslmode=disable"
	}
	return url
}

func tableExists(t *testing.T, url, table string) bool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	defer pool.Close()

	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
		table,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("checking table %s: %v", table, err)
	}
	return exists
}

func TestMigrateUpCreatesAllTablesThenDownDropsThem(t *testing.T) {
	url := testDatabaseURL(t)

	// Start from a clean slate in case a previous run left state behind.
	_ = MigrateDown(url)

	if err := MigrateUp(url); err != nil {
		t.Fatalf("MigrateUp() error = %v", err)
	}
	t.Cleanup(func() { _ = MigrateDown(url) })

	for _, table := range []string{"users", "songs", "transcription_versions", "sessions", "google_oauth_tokens"} {
		if !tableExists(t, url, table) {
			t.Errorf("table %q does not exist after MigrateUp()", table)
		}
	}

	if err := MigrateDown(url); err != nil {
		t.Fatalf("MigrateDown() error = %v", err)
	}
	for _, table := range []string{"users", "songs", "transcription_versions", "sessions", "google_oauth_tokens"} {
		if tableExists(t, url, table) {
			t.Errorf("table %q still exists after MigrateDown()", table)
		}
	}
}
