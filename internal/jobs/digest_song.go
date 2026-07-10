package jobs

import (
	"context"
	"errors"

	"github.com/riverqueue/river"

	"github.com/jhash/tabitha/internal/db"
)

// DigestSongArgs fetches a single song's Google Doc and writes a new
// transcription version from it.
type DigestSongArgs struct {
	SongID int64 `json:"song_id"`
}

func (DigestSongArgs) Kind() string { return "digest_song" }

type DigestSongWorker struct {
	river.WorkerDefaults[DigestSongArgs]
	Queries *db.Queries
}

// ErrNoOAuthToken is returned until a superadmin has logged in via Google
// and a readonly Drive/Docs token is on file — see design doc Phase 2.
var ErrNoOAuthToken = errors.New("digest_song: no Google OAuth token on file yet; log in at /auth/google first")

func (w *DigestSongWorker) Work(ctx context.Context, job *river.Job[DigestSongArgs]) error {
	// Stubbed until Task 23: real digestion needs the Sheets API (for the
	// song's google_doc_id, not visible in the plain CSV export) and the
	// Docs API to fetch content, both via the stored OAuth token.
	return river.JobCancel(ErrNoOAuthToken)
}
