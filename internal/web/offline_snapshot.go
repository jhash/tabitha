package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// offlineSnapshotCacheTTL bounds how often BuildOfflineSnapshot actually
// re-queries and re-renders the whole catalog — cheap enough for the
// catalog's current size, but there's no reason to redo it on every request
// to /offline/snapshot.sqlite when songs change at most a few times an
// hour.
const offlineSnapshotCacheTTL = 5 * time.Minute

// offlineSnapshotCache holds the most recently built snapshot in memory.
// version is a content hash (see hashSnapshotRows) that changes whenever any
// song's slug/updated_at changes, so /offline/meta can tell a client
// "nothing changed" without shipping the whole file.
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
// chord/lyric layout to drift out of sync — and writes it into a fresh
// SQLite file. The client copies this file into IndexedDB in the
// background (see static/js/offline-sync.js); the service worker later
// opens it with sql.js to serve a song page whose URL was never actually
// visited while online (see static/sw.js).
func buildOfflineSnapshot(ctx context.Context, q *db.Queries) ([]byte, string, error) {
	rows, err := q.ListSongsForOfflineSnapshot(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("listing songs for offline snapshot: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "tabitha-offline-*.sqlite")
	if err != nil {
		return nil, "", fmt.Errorf("creating offline snapshot temp file: %w", err)
	}
	path := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(path)

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, "", fmt.Errorf("opening offline snapshot sqlite db: %w", err)
	}
	defer sqlDB.Close()

	if _, err := sqlDB.ExecContext(ctx, `
		CREATE TABLE songs (
			slug TEXT PRIMARY KEY,
			id INTEGER NOT NULL,
			title TEXT NOT NULL,
			artist TEXT NOT NULL,
			html TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`); err != nil {
		return nil, "", fmt.Errorf("creating offline snapshot schema: %w", err)
	}

	stmt, err := sqlDB.PrepareContext(ctx, `INSERT INTO songs (slug, id, title, artist, html, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, "", fmt.Errorf("preparing offline snapshot insert: %w", err)
	}
	defer stmt.Close()

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
		if _, err := stmt.ExecContext(ctx, row.Song.Slug, row.Song.ID, row.Song.Title, row.Song.Artist, html, updatedAt); err != nil {
			return nil, "", fmt.Errorf("inserting offline snapshot row for %q: %w", row.Song.Slug, err)
		}
		fmt.Fprintf(hash, "%s:%s\n", row.Song.Slug, updatedAt)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("reading offline snapshot file: %w", err)
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
