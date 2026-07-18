package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

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

func TestBuildOfflineManifestListsDigestedSongSlugs(t *testing.T) {
	q := setupTestQueries(t)
	createDigestedSong(t, q, "(I Can't Get No) Satisfaction", "Rolling Stones, the", "satisfaction")

	data, err := buildOfflineManifest(context.Background(), q)
	if err != nil {
		t.Fatalf("buildOfflineManifest() error = %v", err)
	}

	var manifest OfflineManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshaling manifest: %v", err)
	}
	if manifest.Version == "" {
		t.Error("manifest version is empty")
	}
	if len(manifest.Songs) != 1 || manifest.Songs[0].Slug != "satisfaction" {
		t.Errorf("songs = %+v, want one song with slug %q", manifest.Songs, "satisfaction")
	}
	if manifest.Songs[0].ContentHash == "" {
		t.Error("expected a non-empty contentHash")
	}
}

func TestBuildOfflineManifestOmitsSongsWithoutADigestedVersion(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()
	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "africa"}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	data, err := buildOfflineManifest(ctx, q)
	if err != nil {
		t.Fatalf("buildOfflineManifest() error = %v", err)
	}

	var manifest OfflineManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshaling manifest: %v", err)
	}
	for _, s := range manifest.Songs {
		if s.Slug == "africa" {
			t.Error("undigested song appeared in the offline manifest, want it omitted")
		}
	}
}

func TestBuildOfflineManifestVersionChangesWhenASongChanges(t *testing.T) {
	q := setupTestQueries(t)
	createDigestedSong(t, q, "(I Can't Get No) Satisfaction", "Rolling Stones, the", "satisfaction")

	before, err := buildOfflineManifest(context.Background(), q)
	if err != nil {
		t.Fatalf("buildOfflineManifest() error = %v", err)
	}
	var beforeManifest OfflineManifest
	_ = json.Unmarshal(before, &beforeManifest)

	createDigestedSong(t, q, "Africa", "Toto", "africa")

	after, err := buildOfflineManifest(context.Background(), q)
	if err != nil {
		t.Fatalf("buildOfflineManifest() error = %v", err)
	}
	var afterManifest OfflineManifest
	_ = json.Unmarshal(after, &afterManifest)

	if beforeManifest.Version == afterManifest.Version {
		t.Error("manifest version unchanged after adding a new digested song")
	}
}

// TestGetOfflineManifestReflectsAnEditImmediately guards against
// re-introducing a time-based cache: a caller re-fetching the manifest
// right after an edit must see the change, with no staleness window.
func TestGetOfflineManifestReflectsAnEditImmediately(t *testing.T) {
	q := setupTestQueries(t)
	song := createDigestedSong(t, q, "(I Can't Get No) Satisfaction", "Rolling Stones, the", "satisfaction")

	before, err := GetOfflineManifest(context.Background(), q)
	if err != nil {
		t.Fatalf("GetOfflineManifest() error = %v", err)
	}

	editedBlocks := []transcription.Block{
		{Kind: transcription.SectionHeader, Text: "VERSE:"},
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "A"},
				{Text: "a completely different line"},
			},
		},
	}
	content, err := transcription.MarshalDocument(editedBlocks)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	ctx := context.Background()
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "manual_edit",
		RawText: transcription.Render(editedBlocks), Content: content,
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	after, err := GetOfflineManifest(context.Background(), q)
	if err != nil {
		t.Fatalf("GetOfflineManifest() error = %v", err)
	}

	if string(before) == string(after) {
		t.Error("GetOfflineManifest() unchanged immediately after an edit, want it to reflect the edit with no caching delay")
	}
}

func TestRenderOfflineSongRendersTheFullPage(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	createDigestedSong(t, q, "(I Can't Get No) Satisfaction", "Rolling Stones, the", "satisfaction")

	song, err := RenderOfflineSong(context.Background(), q, "satisfaction")
	if err != nil {
		t.Fatalf("RenderOfflineSong() error = %v", err)
	}
	if song == nil {
		t.Fatal("RenderOfflineSong() = nil, want a rendered song")
	}
	if song.Title != "(I Can't Get No) Satisfaction" {
		t.Errorf("title = %q, want the song's title", song.Title)
	}
	if !strings.Contains(song.HTML, `class="chord-word"`) {
		t.Error("expected the rendered HTML to include chord-word units")
	}
	if !strings.Contains(song.HTML, `class="site-header"`) {
		t.Error("expected the rendered HTML to include the normal page chrome, so it's indistinguishable from an online page load")
	}
	if song.ContentHash == "" {
		t.Error("expected a non-empty contentHash")
	}
}

// TestRenderOfflineSongContentHashMatchesManifest is the whole point of
// computing the hash in SQL rather than in Go on each side separately: a
// client's stored hash (from RenderOfflineSong) and the manifest's hash
// for the same slug (from buildOfflineManifest) must be byte-for-byte
// equal, or the download-queue diff would never settle.
func TestRenderOfflineSongContentHashMatchesManifest(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	createDigestedSong(t, q, "(I Can't Get No) Satisfaction", "Rolling Stones, the", "satisfaction")
	ctx := context.Background()

	manifestData, err := buildOfflineManifest(ctx, q)
	if err != nil {
		t.Fatalf("buildOfflineManifest() error = %v", err)
	}
	var manifest OfflineManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshaling manifest: %v", err)
	}

	song, err := RenderOfflineSong(ctx, q, "satisfaction")
	if err != nil {
		t.Fatalf("RenderOfflineSong() error = %v", err)
	}

	if len(manifest.Songs) != 1 || manifest.Songs[0].ContentHash != song.ContentHash {
		t.Errorf("manifest contentHash = %+v, RenderOfflineSong contentHash = %q, want them equal", manifest.Songs, song.ContentHash)
	}
}

func TestRenderOfflineSongReturnsNilForUndigestedSlug(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()
	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "africa"}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	got, err := RenderOfflineSong(ctx, q, "africa")
	if err != nil {
		t.Fatalf("RenderOfflineSong() error = %v", err)
	}
	if got != nil {
		t.Errorf("RenderOfflineSong() = %+v, want nil for an undigested song", got)
	}
}

func TestRenderOfflineSongReturnsNilForUnknownSlug(t *testing.T) {
	q := setupTestQueries(t)
	got, err := RenderOfflineSong(context.Background(), q, "does-not-exist")
	if err != nil {
		t.Fatalf("RenderOfflineSong() error = %v", err)
	}
	if got != nil {
		t.Errorf("RenderOfflineSong() = %+v, want nil for an unknown slug", got)
	}
}
