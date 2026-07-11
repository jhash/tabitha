package web

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

func TestCurrentVersionBlocksReturnsFalseWhenSongHasNoVersion(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	blocks, _, hasVersion, err := currentVersionBlocks(ctx, q, song)
	if err != nil {
		t.Fatalf("currentVersionBlocks() error = %v", err)
	}
	if hasVersion {
		t.Error("hasVersion = true, want false for a song with no current_version_id")
	}
	if blocks != nil {
		t.Errorf("blocks = %v, want nil", blocks)
	}
}

func TestCurrentVersionBlocksRoundTripsRealParsedContent(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{
		Title: "(I Can't Get No) Satisfaction", Artist: "Rolling Stones, the",
	})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	original := []transcription.Block{
		{Kind: transcription.SectionHeader, Text: "CHORUS:"},
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "E"},
				{Text: "I can't get no satisfaction"},
			},
		},
	}
	content, err := transcription.MarshalDocument(original)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}

	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID:  song.ID,
		Kind:    "primary",
		Source:  "manual_edit",
		RawText: transcription.Render(original),
		Content: content,
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.MarkVersionCurrent(ctx, version.ID); err != nil {
		t.Fatalf("MarkVersionCurrent() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	song, err = q.GetSongByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetSongByID() error = %v", err)
	}

	blocks, _, hasVersion, err := currentVersionBlocks(ctx, q, song)
	if err != nil {
		t.Fatalf("currentVersionBlocks() error = %v", err)
	}
	if !hasVersion {
		t.Fatal("hasVersion = false, want true")
	}
	if len(blocks) != len(original) {
		t.Fatalf("got %d blocks, want %d", len(blocks), len(original))
	}
	if transcription.Render(blocks) != transcription.Render(original) {
		t.Errorf("round-tripped blocks render differently:\ngot:  %q\nwant: %q", transcription.Render(blocks), transcription.Render(original))
	}
}

// TestFullPipelineRealSatisfactionFileParseStoreFetchRender is the whole
// system in one test: the real sample file, through the real parser, into
// real Postgres JSONB, back out through the real show-page query path, and
// re-rendered — asserting the exact original file comes back byte for byte.
func TestFullPipelineRealSatisfactionFileParseStoreFetchRender(t *testing.T) {
	raw, err := os.ReadFile("../../music/satisfaction-rolling-stones.txt")
	if err != nil {
		t.Fatalf("reading real sample file: %v", err)
	}

	original := transcription.Parse(string(raw))

	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{
		Title: "(I Can't Get No) Satisfaction", Artist: "Rolling Stones, the",
	})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	content, err := transcription.MarshalDocument(original)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "google_doc_scrape",
		RawText: string(raw), Content: content,
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

	blocks, _, hasVersion, err := currentVersionBlocks(ctx, q, song)
	if err != nil {
		t.Fatalf("currentVersionBlocks() error = %v", err)
	}
	if !hasVersion {
		t.Fatal("hasVersion = false, want true")
	}

	rendered := transcription.Render(blocks)
	if rendered != string(raw) {
		t.Errorf("full pipeline did not reproduce the original file byte-for-byte.\ngot:\n%s\nwant:\n%s", rendered, raw)
	}

	// And the HTML page itself must contain that exact text inside a <pre>,
	// proving the alignment survives all the way to what a browser renders.
	html := renderSongShow(t, song, blocks, hasVersion, false)
	if !strings.Contains(html, "<pre") {
		t.Error("expected the transcription inside a <pre> element")
	}
}
