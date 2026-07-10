package auth

import (
	"context"
	"os"
	"testing"
	"time"

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

	if _, err := pool.Exec(ctx, "TRUNCATE users, sessions RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncating test db: %v", err)
	}
	return db.New(pool)
}

func TestNewSessionTokenIsUnpredictableAndURLSafe(t *testing.T) {
	a, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	b, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	if a == b {
		t.Error("two tokens were identical — not random")
	}
	if len(a) < 32 {
		t.Errorf("token %q is only %d chars, want a longer unguessable token", a, len(a))
	}
}

func TestCreateSessionThenCurrentUserRoundTrips(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	token, expiresAt, err := CreateSession(ctx, q, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if token == "" {
		t.Fatal("CreateSession() returned an empty token")
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("expiresAt = %v, want a time in the future", expiresAt)
	}

	got, err := CurrentUser(ctx, q, token)
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("CurrentUser() returned user %d, want %d", got.ID, user.ID)
	}
}

func TestCurrentUserFailsForUnknownToken(t *testing.T) {
	q := setupTestQueries(t)
	if _, err := CurrentUser(context.Background(), q, "not-a-real-token"); err == nil {
		t.Error("CurrentUser() succeeded for an unknown token, want an error")
	}
}

func TestCurrentUserFailsAfterLogout(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	token, _, err := CreateSession(ctx, q, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if err := q.DeleteSession(ctx, token); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	if _, err := CurrentUser(ctx, q, token); err == nil {
		t.Error("CurrentUser() succeeded after logout, want an error")
	}
}
