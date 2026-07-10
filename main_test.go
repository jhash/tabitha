package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
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

func TestRunReparseRederivesContentFromRawText(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()
	q := db.New(pool)

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Reparse Test Song", Artist: "Someone"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	// Stale content as if digested before the lowercase-suffix header fix
	// landed: "VERSE 1a:" misclassified as an opaque TextLine.
	rawText := "VERSE 1a:\n"
	staleBlocks := []transcription.Block{{Kind: transcription.TextLine, Text: "VERSE 1a:"}}
	staleContent, err := transcription.MarshalDocument(staleBlocks)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}

	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "google_doc_scrape", RawText: rawText, Content: staleContent,
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	if err := runReparse(cfg); err != nil {
		t.Fatalf("runReparse() error = %v", err)
	}

	updated, err := q.GetTranscriptionVersion(ctx, version.ID)
	if err != nil {
		t.Fatalf("GetTranscriptionVersion() error = %v", err)
	}
	blocks, err := transcription.UnmarshalDocument(updated.Content)
	if err != nil {
		t.Fatalf("UnmarshalDocument() error = %v", err)
	}
	if len(blocks) == 0 || blocks[0].Kind != transcription.SectionHeader {
		t.Errorf("got %+v, want first block to be SectionHeader (reparsed with the current parser)", blocks)
	}
	if updated.RawText != rawText {
		t.Errorf("RawText changed = %q, want unchanged %q", updated.RawText, rawText)
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

func testServerPort(t *testing.T, handler http.Handler) string {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}
	return u.Port()
}

func TestRunHealthcheckSucceedsWhenServerHealthy(t *testing.T) {
	port := testServerPort(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if err := runHealthcheck(config.Config{Port: port}); err != nil {
		t.Errorf("runHealthcheck() error = %v", err)
	}
}

func TestRunHealthcheckFailsWhenServerReturnsError(t *testing.T) {
	port := testServerPort(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	if err := runHealthcheck(config.Config{Port: port}); err == nil {
		t.Error("runHealthcheck() succeeded, want an error for a 503 response")
	}
}

func TestRunHealthcheckFailsWhenServerUnreachable(t *testing.T) {
	if err := runHealthcheck(config.Config{Port: "1"}); err == nil {
		t.Error("runHealthcheck() succeeded against an unreachable port, want an error")
	}
}
