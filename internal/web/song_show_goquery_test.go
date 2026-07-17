package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// TestSongShowRouteRendersChordWordsInOrderWithChordAboveItsOwnLyric
// walks the actual served chord chart structurally: each .chord-word span
// pairs a .chord with the .lyric word it belongs to, in source order.
// song_show_test.go's unit tests already check individual spans render
// (via substring match on the render function's output directly) — this
// instead proves the full HTTP response, through the real router, keeps
// chord/lyric pairing intact and in the right sequence, which a substring
// check can't (it can't tell "E" is paired with "I" rather than some
// other word elsewhere on the page).
func TestSongShowRouteRendersChordWordsInOrderWithChordAboveItsOwnLyric(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Satisfaction", Artist: "Rolling Stones"})
	if err != nil {
		t.Fatalf("seeding song: %v", err)
	}
	blocks := []transcription.Block{
		{Kind: transcription.SectionHeader, Text: "CHORUS:"},
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "E"},
				{Text: "I can't "},
				{Chord: "A"},
				{Text: "satisfaction"},
			},
		},
	}
	content, err := transcription.MarshalDocument(blocks)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "manual_edit", RawText: "x", Content: content,
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	doc := getDoc(t, r, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/songs/%d", song.ID), nil))

	if got := doc.Find(".section-header").Text(); got != "CHORUS:" {
		t.Errorf(".section-header text = %q, want %q", got, "CHORUS:")
	}

	type pair struct{ chord, lyric string }
	want := []pair{
		{"E", "I"},
		{"", "can't"},
		{"A", "satisfaction"},
	}

	words := doc.Find(".chord-line .chord-word")
	if words.Length() != len(want) {
		t.Fatalf("got %d .chord-word spans, want %d", words.Length(), len(want))
	}
	words.Each(func(i int, s *goquery.Selection) {
		chord := s.Find(".chord").Text()
		lyric := s.Find(".lyric").Text()
		if chord != want[i].chord || lyric != want[i].lyric {
			t.Errorf("chord-word[%d] = {chord: %q, lyric: %q}, want %+v", i, chord, lyric, want[i])
		}
	})
}

// TestSongShowRouteBoldMarkRendersAsStrongInsideItsOwnLyricWord verifies a
// bold mark lands on the right <strong> element nested inside the right
// .lyric span — proving the mark travels with its specific word through
// the full render+router path, not just that "<strong>" appears somewhere
// in the body.
func TestSongShowRouteBoldMarkRendersAsStrongInsideItsOwnLyricWord(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Downtown", Artist: "Petula Clark"})
	if err != nil {
		t.Fatalf("seeding song: %v", err)
	}
	blocks := []transcription.Block{
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "C"},
				{Text: "plain", Bold: false},
				{Text: " "},
				{Chord: "G"},
				{Text: "bold", Bold: true},
			},
		},
	}
	content, err := transcription.MarshalDocument(blocks)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "manual_edit", RawText: "x", Content: content,
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	doc := getDoc(t, r, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/songs/%d", song.ID), nil))

	words := doc.Find(".chord-line .chord-word")
	if words.Length() != 2 {
		t.Fatalf("got %d .chord-word spans, want 2", words.Length())
	}
	if n := words.Eq(0).Find(".lyric strong").Length(); n != 0 {
		t.Errorf("\"plain\" word has %d <strong> descendants, want 0", n)
	}
	if got := words.Eq(1).Find(".lyric strong").Text(); got != "bold" {
		t.Errorf("\"bold\" word's <strong> text = %q, want %q", got, "bold")
	}
}
