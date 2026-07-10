package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// setupTestDB returns a Queries backed by the real local Postgres test
// database, migrated and with all tables truncated for test isolation.
// Exercising sqlc-generated CRUD against real Postgres (constraints,
// upsert conflict targets, enum handling) is the only meaningful way to
// verify it — there's nothing useful to mock here.
func setupTestDB(t *testing.T) *Queries {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres:///tabitha_test?sslmode=disable"
	}

	if err := MigrateUp(url); err != nil {
		t.Fatalf("migrating test db: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, "TRUNCATE songs, transcription_versions, users, sessions, google_oauth_tokens RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncating test db: %v", err)
	}

	return New(pool)
}
