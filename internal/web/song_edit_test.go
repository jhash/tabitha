package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

func renderSongEdit(t *testing.T, song db.Song, blocks []transcription.Block, hasVersion bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := songEditContent(song, blocks, hasVersion).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestSongEditContentShowsRawTranscriptionWhenDigested(t *testing.T) {
	blocks := []transcription.Block{
		{Kind: transcription.SectionHeader, Text: "CHORUS:"},
	}
	html := renderSongEdit(t, db.Song{ID: 7, Title: "Africa"}, blocks, true)

	if !strings.Contains(html, "CHORUS:") {
		t.Errorf("expected the raw transcription to render, got: %s", html)
	}
	if !strings.Contains(html, "<pre") {
		t.Error("expected the transcription inside a <pre> element to preserve alignment")
	}
}

func TestSongEditContentShowsPlaceholderWhenNotYetDigested(t *testing.T) {
	html := renderSongEdit(t, db.Song{ID: 7, Title: "Africa"}, nil, false)
	if !strings.Contains(html, "hasn&#39;t been digested") {
		t.Errorf("expected a not-yet-digested message, got: %s", html)
	}
}

func TestSongEditContentLinksBackToTheSongPage(t *testing.T) {
	html := renderSongEdit(t, db.Song{ID: 7, Title: "Africa"}, nil, false)
	if !strings.Contains(html, `href="/songs/7"`) {
		t.Errorf("expected a link back to /songs/7, got: %s", html)
	}
}
