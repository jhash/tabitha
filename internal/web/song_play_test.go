package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

func renderSongPlay(t *testing.T, song db.Song, blocks []transcription.Block, key string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := songPlayContent(song, blocks, key).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestSongPlayRendersTitleBylineAndKey(t *testing.T) {
	song := db.Song{Title: "Africa", Artist: "Toto"}
	html := renderSongPlay(t, song, nil, "A")

	if !strings.Contains(html, "<h1>Africa</h1>") {
		t.Errorf("expected title to render, got: %s", html)
	}
	if !strings.Contains(html, "As performed by Toto") {
		t.Errorf("expected byline to render, got: %s", html)
	}
	if !strings.Contains(html, "<b>A</b>") {
		t.Errorf("expected key to render, got: %s", html)
	}
}

func TestSongPlayRendersSameTranscriptionAsShowPage(t *testing.T) {
	song := db.Song{Title: "Satisfaction"}
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
	html := renderSongPlay(t, song, blocks, "")

	if !strings.Contains(html, "CHORUS:") {
		t.Error("expected section header to render")
	}
	if !strings.Contains(html, `<span class="chord">E</span>`) {
		t.Errorf("expected the E chord to render in its own span, got: %s", html)
	}
}

func TestSongPlayIncludesScrollerAndNavControls(t *testing.T) {
	song := db.Song{ID: 7, Slug: "africa", Title: "Africa"}
	html := renderSongPlay(t, song, nil, "")

	if !strings.Contains(html, `id="play-scroller"`) {
		t.Error("expected the paginated scroller element")
	}
	if !strings.Contains(html, `data-show-href="/songs/africa"`) {
		t.Errorf("expected data-show-href pointing back at the show page, got: %s", html)
	}
	if !strings.Contains(html, `class="play-close"`) || !strings.Contains(html, `class="play-prev"`) || !strings.Contains(html, `class="play-next"`) {
		t.Errorf("expected close/prev/next controls, got: %s", html)
	}
}
