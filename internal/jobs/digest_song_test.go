package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// The Sheets/Docs API happy path needs real Google credentials and isn't
// exercised here — verified manually via /admin/tools against a real doc
// instead (see todos.md). These tests cover the error paths that don't
// require actual network access.

func TestDigestSongWorkerFailsWithClearErrorWhenNoOAuthToken(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Great Balls of Fire", Artist: "Jerry Lee Lewis"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	worker := &DigestSongWorker{Queries: q, Config: config.Config{GoogleKey: "key", GoogleSecret: "secret"}, EncryptionKey: testEncryptionKey()}
	job := &river.Job[DigestSongArgs]{JobRow: &rivertype.JobRow{}, Args: DigestSongArgs{SongID: song.ID}}

	err = worker.Work(ctx, job)
	if err == nil {
		t.Fatal("Work() error = nil, want an error with no Google OAuth token on file")
	}
	if !errors.Is(err, ErrNoOAuthToken) {
		t.Errorf("Work() error = %v, want it to wrap ErrNoOAuthToken", err)
	}
	var cancelErr *rivertype.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Error("Work() error should be a JobCancel — no OAuth token isn't going to fix itself on retry")
	}
}

func TestDigestSongWorkerFailsWhenSongNotFound(t *testing.T) {
	q := setupTestQueries(t)
	worker := &DigestSongWorker{Queries: q, EncryptionKey: testEncryptionKey()}

	job := &river.Job[DigestSongArgs]{JobRow: &rivertype.JobRow{}, Args: DigestSongArgs{SongID: 999999}}
	if err := worker.Work(context.Background(), job); err == nil {
		t.Fatal("Work() error = nil, want an error for an unknown song ID")
	}
}

func testEncryptionKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}
