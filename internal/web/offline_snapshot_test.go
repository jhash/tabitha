package web

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

func createDigestedSong(t *testing.T, q *db.Queries, title, artist, slug string) db.Song {
	t.Helper()
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: title, Artist: artist})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: slug}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	blocks := []transcription.Block{
		{Kind: transcription.SectionHeader, Text: "CHORUS:"},
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "E"},
				{Text: "I can't get no satisfaction"},
			},
		},
	}
	content, err := transcription.MarshalDocument(blocks)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "manual_edit",
		RawText: transcription.Render(blocks), Content: content,
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	song, err = q.GetSongByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetSongByID() error = %v", err)
	}
	return song
}

func TestBuildOfflineSnapshotProducesAQueryableSQLiteFileWithRenderedSongs(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	createDigestedSong(t, q, "(I Can't Get No) Satisfaction", "Rolling Stones, the", "satisfaction")

	data, version, err := buildOfflineSnapshot(context.Background(), q)
	if err != nil {
		t.Fatalf("buildOfflineSnapshot() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("buildOfflineSnapshot() returned no data")
	}
	if version == "" {
		t.Error("buildOfflineSnapshot() returned an empty version hash")
	}

	path := writeTempSQLiteFile(t, data)
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("opening produced sqlite file: %v", err)
	}
	defer sqlDB.Close()

	var html, title string
	if err := sqlDB.QueryRow(`SELECT title, html FROM songs WHERE slug = ?`, "satisfaction").Scan(&title, &html); err != nil {
		t.Fatalf("querying snapshot row: %v", err)
	}
	if title != "(I Can't Get No) Satisfaction" {
		t.Errorf("title = %q, want the song's title", title)
	}
	if !strings.Contains(html, `class="chord-word"`) {
		t.Error("expected the stored HTML to be the fully rendered song page, including chord-word units")
	}
	if !strings.Contains(html, `class="site-header"`) {
		t.Error("expected the stored HTML to include the normal page chrome, so it's indistinguishable from an online page load")
	}
}

func TestBuildOfflineSnapshotOmitsSongsWithoutADigestedVersion(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	ctx := context.Background()
	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "africa"}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	data, _, err := buildOfflineSnapshot(ctx, q)
	if err != nil {
		t.Fatalf("buildOfflineSnapshot() error = %v", err)
	}

	path := writeTempSQLiteFile(t, data)
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("opening produced sqlite file: %v", err)
	}
	defer sqlDB.Close()

	var count int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM songs WHERE slug = ?`, "africa").Scan(&count); err != nil {
		t.Fatalf("querying snapshot row count: %v", err)
	}
	if count != 0 {
		t.Errorf("undigested song appeared in the offline snapshot, want it omitted")
	}
}

func TestBuildOfflineSnapshotVersionChangesWhenASongChanges(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	createDigestedSong(t, q, "(I Can't Get No) Satisfaction", "Rolling Stones, the", "satisfaction")

	_, before, err := buildOfflineSnapshot(context.Background(), q)
	if err != nil {
		t.Fatalf("buildOfflineSnapshot() error = %v", err)
	}

	createDigestedSong(t, q, "Africa", "Toto", "africa")

	_, after, err := buildOfflineSnapshot(context.Background(), q)
	if err != nil {
		t.Fatalf("buildOfflineSnapshot() error = %v", err)
	}

	if before == after {
		t.Error("version hash unchanged after adding a new digested song")
	}
}

func TestGetOfflineSnapshotCachesWithinTTL(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	createDigestedSong(t, q, "(I Can't Get No) Satisfaction", "Rolling Stones, the", "satisfaction")
	resetSnapshotCache(t)

	_, v1, err := GetOfflineSnapshot(context.Background(), q)
	if err != nil {
		t.Fatalf("GetOfflineSnapshot() error = %v", err)
	}

	createDigestedSong(t, q, "Africa", "Toto", "africa")

	_, v2, err := GetOfflineSnapshot(context.Background(), q)
	if err != nil {
		t.Fatalf("GetOfflineSnapshot() error = %v", err)
	}

	if v1 != v2 {
		t.Error("GetOfflineSnapshot() rebuilt within its TTL, want the cached version reused")
	}
}

// resetSnapshotCache clears the process-wide snapshot cache around a test
// that depends on GetOfflineSnapshot actually rebuilding (or not), without
// copying the cache's mutex (go vet: copylocks).
func resetSnapshotCache(t *testing.T) {
	t.Helper()
	clear := func() {
		snapshotCache.mu.Lock()
		defer snapshotCache.mu.Unlock()
		snapshotCache.data = nil
		snapshotCache.version = ""
		snapshotCache.builtAt = time.Time{}
	}
	clear()
	t.Cleanup(clear)
}

func writeTempSQLiteFile(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "snapshot-*.sqlite")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}
	return f.Name()
}
