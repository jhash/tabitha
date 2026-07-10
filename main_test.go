package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

func testConfig(t *testing.T) config.Config {
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

	// See internal/{auth,web,jobs,db}'s copies of this same pattern: several
	// packages share this one test database, and go test -p 1 is required
	// to run them all in one invocation without racing each other.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("beginning truncate tx: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(72469)"); err != nil {
		t.Fatalf("acquiring test db truncate lock: %v", err)
	}
	if _, err := tx.Exec(ctx, "TRUNCATE users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncating test db: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("committing truncate tx: %v", err)
	}

	return config.Config{DatabaseURL: url}
}

func TestRunPromoteRequiresEmailArgument(t *testing.T) {
	cfg := testConfig(t)
	if err := runPromote(cfg, nil); err == nil {
		t.Fatal("runPromote() with no args succeeded, want a usage error")
	}
}

func TestRunPromotePromotesExistingUser(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()
	q := db.New(pool)
	if _, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"}); err != nil {
		t.Fatalf("seeding user: %v", err)
	}

	if err := runPromote(cfg, []string{"jhash147@gmail.com"}); err != nil {
		t.Fatalf("runPromote() error = %v", err)
	}

	user, err := q.GetUserByEmail(ctx, "jhash147@gmail.com")
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if user.Role != db.UserRoleSuperadmin {
		t.Errorf("Role = %v, want superadmin", user.Role)
	}
}

func TestRunPromoteFailsForUnknownEmail(t *testing.T) {
	cfg := testConfig(t)
	err := runPromote(cfg, []string{"nobody@example.com"})
	if err == nil {
		t.Fatal("runPromote() for an unknown email succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "nobody@example.com") {
		t.Errorf("error = %v, want it to mention the email that wasn't found", err)
	}
}
