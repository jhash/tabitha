package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

func renderSongShow(t *testing.T, song db.Song, blocks []transcription.Block, hasVersion, viewerIsSuperadmin bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := songShowContent(song, blocks, hasVersion, viewerIsSuperadmin).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestSongShowRendersTitleAndArtist(t *testing.T) {
	song := db.Song{Title: "(I Can't Get No) Satisfaction", Artist: "Rolling Stones, the"}
	html := renderSongShow(t, song, nil, false, false)
	if !strings.Contains(html, "(I Can&#39;t Get No) Satisfaction") {
		t.Error("expected title to render (HTML-escaped)")
	}
	if !strings.Contains(html, "Rolling Stones, the") {
		t.Error("expected artist to render")
	}
}

func TestSongShowWithoutVersionShowsNotYetDigestedMessage(t *testing.T) {
	song := db.Song{Title: "Africa", Artist: "Toto"}
	html := renderSongShow(t, song, nil, false, false)
	if !strings.Contains(html, "hasn&#39;t been digested") {
		t.Errorf("expected a not-yet-digested message, got: %s", html)
	}
	if strings.Contains(html, "<pre") {
		t.Error("expected no <pre> transcription block when there's no version yet")
	}
}

func TestSongShowRendersTranscriptionPreservingAlignment(t *testing.T) {
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
	html := renderSongShow(t, db.Song{Title: "Satisfaction"}, blocks, true, false)

	if !strings.Contains(html, "CHORUS:") {
		t.Error("expected section header to render")
	}
	// Byte-for-byte alignment only survives in a whitespace-preserving
	// element — assert it's actually inside a <pre>, not stripped into a
	// normal paragraph where the browser would collapse the spacing.
	if !strings.Contains(html, "<pre") {
		t.Error("expected transcription to render inside a <pre> element to preserve chord alignment")
	}
	if !strings.Contains(html, "E") || !strings.Contains(html, "I can&#39;t get no satisfaction") {
		t.Errorf("expected chord and lyric text to render, got: %s", html)
	}
}

func TestSongShowHidesEditLinkFromRegularViewers(t *testing.T) {
	song := db.Song{ID: 42, Title: "Africa", Artist: "Toto"}
	html := renderSongShow(t, song, nil, false, false)
	if strings.Contains(html, "/songs/42/edit") {
		t.Error("expected no edit link for a non-superadmin viewer")
	}
}

func TestSongShowShowsEditLinkToSuperadmins(t *testing.T) {
	song := db.Song{ID: 42, Title: "Africa", Artist: "Toto"}
	html := renderSongShow(t, song, nil, false, true)
	if !strings.Contains(html, "/songs/42/edit") {
		t.Errorf("expected an edit link pointing at /songs/42/edit for a superadmin viewer, got: %s", html)
	}
}
