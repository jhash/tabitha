package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/scrape"
)

// The Ultimate Guitar/e-chords fetch happy path is covered directly
// against a real fixture in internal/scrape's own tests. These cover
// scrape_song's plumbing around that: which errors cancel the job
// outright vs. leave it retryable.

func TestScrapeSongWorkerCancelsWhenSongHasNoSourceURL(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "No URL Song", Artist: "Nobody"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	worker := &ScrapeSongWorker{Queries: q}
	job := &river.Job[ScrapeSongArgs]{JobRow: &rivertype.JobRow{}, Args: ScrapeSongArgs{SongID: song.ID}}

	err = worker.Work(ctx, job)
	if !errors.Is(err, ErrNoSourceURL) {
		t.Errorf("Work() error = %v, want it to wrap ErrNoSourceURL", err)
	}
	var cancelErr *rivertype.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Error("Work() error should be a JobCancel — no URL isn't going to fix itself on retry")
	}
}

func TestScrapeSongWorkerCancelsForUnsupportedHost(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{
		Title: "Unsupported Host Song", Artist: "Nobody", SourceUrl: "https://example.com/whatever",
	})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	worker := &ScrapeSongWorker{Queries: q}
	job := &river.Job[ScrapeSongArgs]{JobRow: &rivertype.JobRow{}, Args: ScrapeSongArgs{SongID: song.ID}}

	err = worker.Work(ctx, job)
	if !errors.Is(err, scrape.ErrUnsupportedSite) {
		t.Errorf("Work() error = %v, want it to wrap scrape.ErrUnsupportedSite", err)
	}
	var cancelErr *rivertype.JobCancelError
	if !errors.As(err, &cancelErr) {
		t.Error("Work() error should be a JobCancel — an unsupported host isn't going to fix itself on retry")
	}
}

func TestScrapeSongWorkerFailsWhenSongNotFound(t *testing.T) {
	q := setupTestQueries(t)
	worker := &ScrapeSongWorker{Queries: q}

	job := &river.Job[ScrapeSongArgs]{JobRow: &rivertype.JobRow{}, Args: ScrapeSongArgs{SongID: 999999}}
	if err := worker.Work(context.Background(), job); err == nil {
		t.Fatal("Work() error = nil, want an error for an unknown song ID")
	}
}
