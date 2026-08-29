package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/time/rate"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// PublishSongDocArgs publishes a song's current transcription version to
// a Google Doc: creates one if the song has none yet (storing the new
// google_doc_id), or overwrites the existing one's content otherwise.
// This is the "up to a new Google Doc" half of two-way sync — the "down"
// half is DigestSongArgs, which already works on any doc ID a song has,
// regardless of how that doc came to exist. Publish -> edit the doc by
// hand -> digest pulls the edit back -> re-publish pushes further edits
// up: a full round trip through infrastructure that already existed for
// the read side.
//
// Deliberately never touches a doc tabitha didn't create: the OAuth
// token this uses only has drive.file access (see
// auth.GoogleDriveFileScope), which Google enforces server-side, so
// there's no path from this job to Jeff's actual TOC-linked docs even if
// a song's google_doc_id were somehow set to one of those by mistake.
type PublishSongDocArgs struct {
	SongID int64 `json:"song_id"`
}

func (PublishSongDocArgs) Kind() string { return "publish_song_doc" }

type PublishSongDocWorker struct {
	river.WorkerDefaults[PublishSongDocArgs]
	Queries       *db.Queries
	Config        config.Config
	EncryptionKey []byte

	// RateLimiter is shared with DigestSongWorker's — both hit the same
	// per-user Docs/Drive API quota.
	RateLimiter *rate.Limiter
}

// ErrNoTranscriptionVersion means the song has no current transcription
// version to publish yet — scrape or digest it first.
var ErrNoTranscriptionVersion = errors.New("publish_song_doc: song has no current transcription version")

func (w *PublishSongDocWorker) wait(ctx context.Context) error {
	if w.RateLimiter == nil {
		return nil
	}
	return w.RateLimiter.Wait(ctx)
}

func (w *PublishSongDocWorker) Work(ctx context.Context, job *river.Job[PublishSongDocArgs]) error {
	song, err := w.Queries.GetSongByID(ctx, job.Args.SongID)
	if err != nil {
		return fmt.Errorf("publish_song_doc: loading song %d: %w", job.Args.SongID, err)
	}
	if song.CurrentVersionID == nil {
		return river.JobCancel(fmt.Errorf("%w (song %d)", ErrNoTranscriptionVersion, song.ID))
	}
	version, err := w.Queries.GetTranscriptionVersion(ctx, *song.CurrentVersionID)
	if err != nil {
		return fmt.Errorf("publish_song_doc: loading transcription version %d: %w", *song.CurrentVersionID, err)
	}

	token, err := auth.ValidGoogleToken(ctx, w.Queries, w.Config, w.EncryptionKey, google.Endpoint)
	if err != nil {
		return river.JobCancel(fmt.Errorf("%w (%v)", ErrNoOAuthToken, err))
	}

	if err := w.wait(ctx); err != nil {
		return fmt.Errorf("publish_song_doc: waiting for rate limiter: %w", err)
	}
	docsSvc, err := docs.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
	if err != nil {
		return fmt.Errorf("publish_song_doc: building docs client: %w", err)
	}
	driveSvc, err := drive.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
	if err != nil {
		return fmt.Errorf("publish_song_doc: building drive client: %w", err)
	}

	docID := song.GoogleDocID
	if docID == "" {
		docID, err = createGoogleDoc(driveSvc, docsSvc, song.Title, version.RawText)
		if err != nil {
			return snoozeOnRateLimit(fmt.Errorf("publish_song_doc: creating doc: %w", err))
		}
		if err := w.Queries.SetSongGoogleDocID(ctx, db.SetSongGoogleDocIDParams{ID: song.ID, GoogleDocID: docID}); err != nil {
			return fmt.Errorf("publish_song_doc: storing google doc id: %w", err)
		}
		log.Printf("publish_song_doc: created new doc %s for song %d", docID, song.ID)
	} else {
		if err := replaceGoogleDocContent(docsSvc, docID, version.RawText); err != nil {
			return snoozeOnRateLimit(fmt.Errorf("publish_song_doc: updating doc: %w", err))
		}
	}

	if err := w.wait(ctx); err != nil {
		return fmt.Errorf("publish_song_doc: waiting for rate limiter: %w", err)
	}
	file, err := driveSvc.Files.Get(docID).Fields("createdTime", "modifiedTime").Do()
	if err != nil {
		return snoozeOnRateLimit(fmt.Errorf("publish_song_doc: fetching doc timestamps: %w", err))
	}
	created, err := parseDriveTime(file.CreatedTime)
	if err != nil {
		return fmt.Errorf("publish_song_doc: parsing createdTime: %w", err)
	}
	modified, err := parseDriveTime(file.ModifiedTime)
	if err != nil {
		return fmt.Errorf("publish_song_doc: parsing modifiedTime: %w", err)
	}
	if err := w.Queries.SetSongDocTimestamps(ctx, db.SetSongDocTimestampsParams{
		ID:            song.ID,
		DocCreatedAt:  pgtype.Timestamptz{Time: created, Valid: true},
		DocModifiedAt: pgtype.Timestamptz{Time: modified, Valid: true},
	}); err != nil {
		return fmt.Errorf("publish_song_doc: storing doc timestamps: %w", err)
	}
	return nil
}
