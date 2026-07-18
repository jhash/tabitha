package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// OfflineManifest is what /offline/manifest.json reports: every digested
// song's slug and content hash, nothing else — enough for
// static/js/offline-sync.js to diff against what it already has in
// IndexedDB and queue up just the slugs that are missing or stale, without
// ever shipping the whole catalog's HTML in one request. Computed fresh on
// every request rather than cached — the underlying query is cheap (no
// rendering, just a join and an md5()), and caching it would mean an edit
// takes up to however-long-the-cache-lives to actually reach the offline
// download queue.
type OfflineManifest struct {
	Version string                `json:"version"`
	Songs   []OfflineManifestSong `json:"songs"`
}

type OfflineManifestSong struct {
	Slug        string `json:"slug"`
	ContentHash string `json:"contentHash"`
}

// OfflineSong is what /offline/songs/{slug} returns for one song — everything
// static/js/offline-sync.js needs to write into its IndexedDB object store,
// and everything static/sw.js needs to serve that song's page offline.
// ContentHash uses the exact same SQL expression as the manifest's (see
// queries/songs.sql), so a stored song's hash and the manifest's hash for
// that slug are always byte-for-byte comparable.
type OfflineSong struct {
	Slug        string `json:"slug"`
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	HTML        string `json:"html"`
	ContentHash string `json:"contentHash"`
}

// GetOfflineManifest builds the current catalog manifest. Exported as a
// stable entry point for OfflineManifestHandler even though — unlike an
// earlier version of this — it does no caching of its own; see
// OfflineManifest's doc comment for why.
func GetOfflineManifest(ctx context.Context, q *db.Queries) ([]byte, error) {
	return buildOfflineManifest(ctx, q)
}

func buildOfflineManifest(ctx context.Context, q *db.Queries) ([]byte, error) {
	rows, err := q.ListSongSlugsForOfflineManifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing songs for offline manifest: %w", err)
	}

	songs := make([]OfflineManifestSong, 0, len(rows))
	hash := sha256.New()
	for _, row := range rows {
		songs = append(songs, OfflineManifestSong{Slug: row.Slug, ContentHash: row.ContentHash})
		fmt.Fprintf(hash, "%s:%s\n", row.Slug, row.ContentHash)
	}

	data, err := json.Marshal(OfflineManifest{
		Version: hex.EncodeToString(hash.Sum(nil))[:16],
		Songs:   songs,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling offline manifest: %w", err)
	}
	return data, nil
}

// RenderOfflineSong renders one digested song's full page HTML — the same
// HTML SongShowHandler would serve, produced by the same Page/
// songShowContent code so there's no second, JS-side implementation of the
// chord/lyric layout to drift out of sync. Returns nil, nil if slug isn't a
// digested song (nothing to serve offline, not an error — the manifest and
// the actual catalog can be momentarily out of sync with each other).
func RenderOfflineSong(ctx context.Context, q *db.Queries, slug string) (*OfflineSong, error) {
	row, err := q.GetSongForOfflineSnapshotBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	blocks, err := transcription.UnmarshalDocument(row.TranscriptionVersion.Content)
	if err != nil {
		return nil, err
	}
	html, err := renderOfflineSongPage(row.Song, blocks, deref(row.TranscriptionVersion.Key))
	if err != nil {
		return nil, err
	}

	return &OfflineSong{
		Slug:        row.Song.Slug,
		ID:          row.Song.ID,
		Title:       row.Song.Title,
		Artist:      row.Song.Artist,
		HTML:        html,
		ContentHash: row.ContentHash,
	}, nil
}

// renderOfflineSongPage renders a song exactly as SongShowHandler would —
// same Page chrome, same songShowContent — so the offline copy is
// indistinguishable from the page a visitor would have gotten online.
func renderOfflineSongPage(song db.Song, blocks []transcription.Block, key string) (string, error) {
	description := fmt.Sprintf("%s, as performed by %s", song.Title, song.Artist)
	page := Page(song.Title, description, nil, false, songShowContent(song, blocks, key, true, false))
	var buf bytes.Buffer
	if err := page.Render(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
