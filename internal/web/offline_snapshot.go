package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// offlineSnapshotCacheTTL bounds how often BuildOfflineSnapshot actually
// re-queries and re-renders the whole catalog — cheap enough for the
// catalog's current size, but there's no reason to redo it on every request
// to /offline/snapshot.json when songs change at most a few times an hour.
const offlineSnapshotCacheTTL = 5 * time.Minute

// offlineSnapshotCache holds the most recently built snapshot in memory.
// version is a content hash (see buildOfflineSnapshot) that changes
// whenever any song's slug/updated_at changes, so /offline/meta can tell a
// client "nothing changed" without shipping the whole payload.
type offlineSnapshotCache struct {
	mu      sync.Mutex
	data    []byte
	version string
	builtAt time.Time
}

var snapshotCache offlineSnapshotCache

// OfflineSnapshotMeta is what /offline/meta reports — just enough for a
// client to decide whether it needs to re-download the full snapshot.
type OfflineSnapshotMeta struct {
	Version string `json:"version"`
}

// OfflineSong is one row of the offline snapshot — everything
// static/js/offline-sync.js needs to write directly into an IndexedDB
// object store keyed by slug, and everything static/sw.js needs to serve a
// song page whose URL was never actually visited while online.
type OfflineSong struct {
	Slug      string `json:"slug"`
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	HTML      string `json:"html"`
	UpdatedAt string `json:"updatedAt"`
}

// GetOfflineSnapshot returns the current cached snapshot, building (and
// caching) a fresh one if the cache is empty or stale.
func GetOfflineSnapshot(ctx context.Context, q *db.Queries) ([]byte, string, error) {
	snapshotCache.mu.Lock()
	defer snapshotCache.mu.Unlock()

	if snapshotCache.data != nil && time.Since(snapshotCache.builtAt) < offlineSnapshotCacheTTL {
		return snapshotCache.data, snapshotCache.version, nil
	}

	data, version, err := buildOfflineSnapshot(ctx, q)
	if err != nil {
		return nil, "", err
	}
	snapshotCache.data = data
	snapshotCache.version = version
	snapshotCache.builtAt = time.Now()
	return data, version, nil
}

// buildOfflineSnapshot renders every digested song's full page HTML — the
// same HTML SongShowHandler would serve, produced by the same Page/
// songShowContent code so there's no second, JS-side implementation of the
// chord/lyric layout to drift out of sync — and serializes it as a JSON
// array. The client (static/js/offline-sync.js) writes each row straight
// into an IndexedDB object store keyed by slug; no intermediate database
// format or query engine needed client-side, since a keyed lookup by slug
// is the only thing static/sw.js ever needs to do with this data.
func buildOfflineSnapshot(ctx context.Context, q *db.Queries) ([]byte, string, error) {
	rows, err := q.ListSongsForOfflineSnapshot(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("listing songs for offline snapshot: %w", err)
	}

	songs := make([]OfflineSong, 0, len(rows))
	hash := sha256.New()
	for _, row := range rows {
		blocks, err := transcription.UnmarshalDocument(row.TranscriptionVersion.Content)
		if err != nil {
			// A malformed stored document shouldn't take down the whole
			// snapshot — just leave that one song out of it.
			continue
		}
		html, err := renderOfflineSongPage(row.Song, blocks, deref(row.TranscriptionVersion.Key))
		if err != nil {
			continue
		}
		updatedAt := row.Song.UpdatedAt.Time.UTC().Format(time.RFC3339)
		songs = append(songs, OfflineSong{
			Slug:      row.Song.Slug,
			ID:        row.Song.ID,
			Title:     row.Song.Title,
			Artist:    row.Song.Artist,
			HTML:      html,
			UpdatedAt: updatedAt,
		})
		fmt.Fprintf(hash, "%s:%s\n", row.Song.Slug, updatedAt)
	}

	data, err := json.Marshal(songs)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling offline snapshot: %w", err)
	}
	return data, hex.EncodeToString(hash.Sum(nil))[:16], nil
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
