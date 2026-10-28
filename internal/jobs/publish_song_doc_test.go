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

// The real Docs/Drive API happy path needs real Google credentials and
// isn't exercised here, same as digest_song's — verified manually via
// /admin/tools against a real doc instead. These cover the error paths
// that don't require actual network access.

func TestPublishSongDocWorkerCancelsWhenNoCurrentVersion(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Undigested Song", Artist: "Nobody"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	worker := &PublishSongDocWorker{Queries: q}
	job := &river.Job[PublishSongDocArgs]{JobRow: &rivertype.JobRow{}, Args: PublishSongDocArgs{SongID: song.ID}}

	err = worker.Work(ctx, job)
	if !errors.Is(err, ErrNoTranscriptionVersion) {
		t.Errorf("Work() error = %v, want it to wrap ErrNoTranscriptionVersion", err)
	}
	var cancelErr *rivertype.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Error("Work() error should be a JobCancel — no transcription version isn't going to fix itself on retry")
	}
}

func TestPublishSongDocWorkerCancelsWhenNoOAuthToken(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Digested Song", Artist: "Somebody"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "manual_edit", RawText: "hello", Content: []byte(`{"blocks":[]}`),
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	worker := &PublishSongDocWorker{Queries: q, Config: config.Config{GoogleKey: "key", GoogleSecret: "secret"}, EncryptionKey: testEncryptionKey()}
	job := &river.Job[PublishSongDocArgs]{JobRow: &rivertype.JobRow{}, Args: PublishSongDocArgs{SongID: song.ID}}

	err = worker.Work(ctx, job)
	if !errors.Is(err, ErrNoOAuthToken) {
		t.Errorf("Work() error = %v, want it to wrap ErrNoOAuthToken", err)
	}
	var cancelErr *rivertype.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Error("Work() error should be a JobCancel — no OAuth token isn't going to fix itself on retry")
	}
}

func TestPublishSongDocWorkerFailsWhenSongNotFound(t *testing.T) {
	q := setupTestQueries(t)
	worker := &PublishSongDocWorker{Queries: q}

	job := &river.Job[PublishSongDocArgs]{JobRow: &rivertype.JobRow{}, Args: PublishSongDocArgs{SongID: 999999}}
	if err := worker.Work(context.Background(), job); err == nil {
		t.Fatal("Work() error = nil, want an error for an unknown song ID")
	}
}
