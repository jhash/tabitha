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

	// internal/auth, internal/web, and internal/jobs each truncate
	// overlapping tables concurrently in their own `go test ./...` package
	// processes against this same shared test DB. Serialize with a
	// transaction-scoped advisory lock (shared key across all copies of
	// this helper) to avoid a cross-process TRUNCATE deadlock.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("beginning truncate tx: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(72469)"); err != nil {
		t.Fatalf("acquiring test db truncate lock: %v", err)
	}
	if _, err := tx.Exec(ctx, "TRUNCATE songs, transcription_versions, users, sessions, google_oauth_tokens, artists, genres RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncating test db: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("committing truncate tx: %v", err)
	}

	return New(pool)
}
