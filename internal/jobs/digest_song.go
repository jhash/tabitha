package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/riverqueue/river"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// DigestSongArgs fetches a single song's Google Doc and writes a new
// transcription version from it.
type DigestSongArgs struct {
	SongID int64 `json:"song_id"`
}

func (DigestSongArgs) Kind() string { return "digest_song" }

type DigestSongWorker struct {
	river.WorkerDefaults[DigestSongArgs]
	Queries       *db.Queries
	Config        config.Config
	EncryptionKey []byte
}

// ErrNoOAuthToken means no superadmin has logged in via Google yet (or the
// stored token can no longer be refreshed) — see design doc Phase 2.
var ErrNoOAuthToken = errors.New("digest_song: no usable Google OAuth token on file; log in at /auth/google first")

// Work handles Jeff's transpose workflow: a doc may hold more than one
// key's transcription separated by a page break (see
// docs/jeff-domain-notes.md). Only the original key's section — the last
// one — is kept; the transposed copy on top is discarded rather than
// stored as a second version, per Jake's call.
func (w *DigestSongWorker) Work(ctx context.Context, job *river.Job[DigestSongArgs]) error {
	song, err := w.Queries.GetSongByID(ctx, job.Args.SongID)
	if err != nil {
		return fmt.Errorf("digest_song: loading song %d: %w", job.Args.SongID, err)
	}

	token, err := auth.ValidGoogleToken(ctx, w.Queries, w.Config, w.EncryptionKey, google.Endpoint)
	if err != nil {
		return river.JobCancel(fmt.Errorf("%w (%v)", ErrNoOAuthToken, err))
	}

	docID := song.GoogleDocID
	if docID == "" {
		docID, err = w.findGoogleDocID(ctx, token, song.Title)
		if err != nil {
			return fmt.Errorf("digest_song: finding google doc id for %q: %w", song.Title, err)
		}
		if err := w.Queries.SetSongGoogleDocID(ctx, db.SetSongGoogleDocIDParams{ID: song.ID, GoogleDocID: docID}); err != nil {
			return fmt.Errorf("digest_song: storing google doc id: %w", err)
		}
	}

	rawText, err := w.fetchDocText(ctx, token, docID)
	if err != nil {
		return fmt.Errorf("digest_song: fetching doc content: %w", err)
	}

	blocks := transcription.Parse(rawText)
	content, err := transcription.MarshalDocument(blocks)
	if err != nil {
		return fmt.Errorf("digest_song: marshaling parsed content: %w", err)
	}

	version, err := w.Queries.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID:  song.ID,
		Kind:    "primary",
		Source:  "google_doc_scrape",
		RawText: rawText,
		Content: content,
	})
	if err != nil {
		return fmt.Errorf("digest_song: storing transcription version: %w", err)
	}

	if err := w.Queries.ClearCurrentVersionsForSong(ctx, song.ID); err != nil {
		return fmt.Errorf("digest_song: clearing prior current version: %w", err)
	}
	if err := w.Queries.MarkVersionCurrent(ctx, version.ID); err != nil {
		return fmt.Errorf("digest_song: marking version current: %w", err)
	}
	if err := w.Queries.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		return fmt.Errorf("digest_song: updating song's current version: %w", err)
	}
	return nil
}

func (w *DigestSongWorker) findGoogleDocID(ctx context.Context, token *oauth2.Token, title string) (string, error) {
	svc, err := sheets.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
	if err != nil {
		return "", fmt.Errorf("building sheets client: %w", err)
	}
	spreadsheet, err := svc.Spreadsheets.Get(tocSpreadsheetID).IncludeGridData(true).Do()
	if err != nil {
		return "", fmt.Errorf("fetching toc spreadsheet: %w", err)
	}
	hyperlink, ok := findHyperlinkForTitle(spreadsheet, title)
	if !ok {
		return "", fmt.Errorf("no row found matching title %q", title)
	}
	return extractDocIDFromHyperlink(hyperlink)
}

func (w *DigestSongWorker) fetchDocText(ctx context.Context, token *oauth2.Token, docID string) (string, error) {
	svc, err := docs.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
	if err != nil {
		return "", fmt.Errorf("building docs client: %w", err)
	}
	doc, err := svc.Documents.Get(docID).Do()
	if err != nil {
		return "", fmt.Errorf("fetching doc %s: %w", docID, err)
	}
	return originalKeySection(docSectionsFromGoogleDoc(doc)), nil
}
