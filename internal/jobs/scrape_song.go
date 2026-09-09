package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/riverqueue/river"

	"github.com/jhash/tabitha/internal/cloudflare"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/scrape"
	"github.com/jhash/tabitha/internal/transcription"
)

// ScrapeSongArgs fetches a song's chord chart from an external site
// (Ultimate Guitar, e-chords) and writes a new transcription version
// from it — the scrape-based counterpart to DigestSongArgs, which does
// the same thing from a Google Doc.
type ScrapeSongArgs struct {
	SongID int64 `json:"song_id"`
	// URL overrides the song's own source_url when set. Lets a caller
	// (e.g. the admin "scrape" trigger) scrape a song that doesn't have
	// source_url populated yet, or re-scrape from a different URL
	// without first editing the song.
	URL string `json:"url,omitempty"`
}

func (ScrapeSongArgs) Kind() string { return "scrape_song" }

type ScrapeSongWorker struct {
	river.WorkerDefaults[ScrapeSongArgs]
	Queries *db.Queries
	Config  config.Config

	// Cloudflare purges the scraped song's page after a successful
	// scrape, same as DigestSongWorker. Nil-safe.
	Cloudflare *cloudflare.Client
}

// ErrNoSourceURL means neither the job args nor the song row has a URL
// to scrape — retrying won't fix this, so Work cancels the job.
var ErrNoSourceURL = errors.New("scrape_song: no URL given and song has no source_url")

func (w *ScrapeSongWorker) Work(ctx context.Context, job *river.Job[ScrapeSongArgs]) error {
	song, err := w.Queries.GetSongByID(ctx, job.Args.SongID)
	if err != nil {
		return fmt.Errorf("scrape_song: loading song %d: %w", job.Args.SongID, err)
	}

	rawURL := job.Args.URL
	if rawURL == "" {
		rawURL = song.SourceUrl
	}
	if rawURL == "" {
		return river.JobCancel(fmt.Errorf("%w (song %d)", ErrNoSourceURL, song.ID))
	}

	provider, err := scrape.ForURL(rawURL)
	if err != nil {
		return river.JobCancel(fmt.Errorf("scrape_song: %w", err))
	}

	scraped, err := provider.Fetch(ctx, rawURL)
	if err != nil {
		if errors.Is(err, scrape.ErrBlockedByCloudflare) {
			// Retrying immediately won't help — the block isn't
			// transient in the way a 5xx or timeout is.
			return river.JobCancel(err)
		}
		return fmt.Errorf("scrape_song: fetching %s: %w", rawURL, err)
	}

	content, err := transcription.MarshalDocument(scraped.Blocks)
	if err != nil {
		return fmt.Errorf("scrape_song: marshaling parsed content: %w", err)
	}

	var key *string
	if scraped.Key != "" {
		key = &scraped.Key
	}

	version, err := w.Queries.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID:  song.ID,
		Kind:    "primary",
		Source:  provider.Name(),
		RawText: scraped.RawText,
		Content: content,
		Key:     key,
	})
	if err != nil {
		return fmt.Errorf("scrape_song: storing transcription version: %w", err)
	}
	if err := w.Queries.ClearCurrentVersionsForSong(ctx, song.ID); err != nil {
		return fmt.Errorf("scrape_song: clearing prior current version: %w", err)
	}
	if err := w.Queries.MarkVersionCurrent(ctx, version.ID); err != nil {
		return fmt.Errorf("scrape_song: marking version current: %w", err)
	}
	if err := w.Queries.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		return fmt.Errorf("scrape_song: updating song's current version: %w", err)
	}
	if song.SourceUrl == "" {
		if err := w.Queries.SetSongSourceURL(ctx, db.SetSongSourceURLParams{ID: song.ID, SourceUrl: rawURL}); err != nil {
			return fmt.Errorf("scrape_song: storing source url: %w", err)
		}
	}

	if w.Cloudflare != nil && w.Cloudflare.Configured() {
		if err := w.Cloudflare.PurgeURLs(ctx, []string{
			w.Config.AppURL + "/",
			w.Config.AppURL + "/" + songPath(song),
		}); err != nil {
			log.Printf("scrape_song: cloudflare purge failed for song %d: %v", song.ID, err)
		}
	}
	return nil
}
