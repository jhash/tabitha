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
	return renderSongShowWithKey(t, song, blocks, "", hasVersion, viewerIsSuperadmin)
}

func renderSongShowWithKey(t *testing.T, song db.Song, blocks []transcription.Block, key string, hasVersion, viewerIsSuperadmin bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := songShowContent(song, blocks, key, hasVersion, viewerIsSuperadmin).Render(&buf); err != nil {
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

func TestSongShowOmitsDuplicateTitleAndBylineFromTranscription(t *testing.T) {
	// The digested doc's own first two lines repeat exactly what the
	// page's own H1/byline already show — Downtown / As performed by:
	// Petula Clark, rendered twice, was the bug report.
	blocks := []transcription.Block{
		{Kind: transcription.TextLine, Text: "Downtown"},
		{Kind: transcription.TextLine, Text: "As performed by: Petula Clark"},
		{Kind: transcription.TextLine, Text: "Key:  E, Original E"},
		{Kind: transcription.SectionHeader, Text: "VERSE 1:"},
	}
	song := db.Song{Title: "Downtown", Artist: "Petula Clark"}
	html := renderSongShowWithKey(t, song, blocks, "E", true, false)

	if strings.Count(html, "Downtown") != 1 {
		t.Errorf("want \"Downtown\" to appear exactly once (H1 only), got %d times: %s", strings.Count(html, "Downtown"), html)
	}
	if strings.Count(html, "Petula Clark") != 1 {
		t.Errorf("want \"Petula Clark\" to appear exactly once (byline only), got %d times: %s", strings.Count(html, "Petula Clark"), html)
	}
	// The Key line is sourced from the database (passed in as key) and
	// shown once near the byline — the doc's own raw "Key: ..." line is
	// now redundant and should be omitted from the <pre> block.
	if strings.Count(html, "Key:") != 1 {
		t.Errorf("want \"Key:\" to appear exactly once (from the database, not the raw doc line), got %d times: %s", strings.Count(html, "Key:"), html)
	}
	if !strings.Contains(html, "VERSE 1:") {
		t.Errorf("expected the rest of the transcription to still render, got: %s", html)
	}
}

func TestSongShowDisplaysKeyFromDatabaseNotRawText(t *testing.T) {
	blocks := []transcription.Block{
		{Kind: transcription.TextLine, Text: "Song"},
		{Kind: transcription.TextLine, Text: "As performed by: Someone"},
		{Kind: transcription.TextLine, Text: "Key:  Bb, Original A"},
		{Kind: transcription.SectionHeader, Text: "VERSE 1:"},
	}
	song := db.Song{Title: "Song", Artist: "Someone"}
	// The database's stored key (the original key, per parseKeyLine) is
	// "A" here — different from the doc's raw "Key: Bb, Original A" line,
	// to prove the displayed key comes from the key parameter/database,
	// not from re-parsing the raw text at render time.
	html := renderSongShowWithKey(t, song, blocks, "A", true, false)

	if !strings.Contains(html, "Key: A") {
		t.Errorf("expected the database-sourced key \"A\" to render, got: %s", html)
	}
}

func TestSongShowOmitsDuplicateBylineWithExtraInternalWhitespace(t *testing.T) {
	// Real bug: "(I Love You) For Sentimental Reasons" still showed the
	// byline twice because the doc's own line has two spaces after the
	// colon ("As performed by:  Nat King Cole") — an exact-match compare
	// missed it.
	blocks := []transcription.Block{
		{Kind: transcription.TextLine, Text: "(I Love You) For Sentimental Reasons"},
		{Kind: transcription.TextLine, Text: "As performed by:  Nat King Cole"},
		{Kind: transcription.SectionHeader, Text: "VERSE 1:"},
	}
	song := db.Song{Title: "(I Love You) For Sentimental Reasons", Artist: "Nat King Cole"}
	html := renderSongShow(t, song, blocks, true, false)

	if strings.Count(html, "Nat King Cole") != 1 {
		t.Errorf("want \"Nat King Cole\" to appear exactly once, got %d times: %s", strings.Count(html, "Nat King Cole"), html)
	}
}

func TestSongShowKeepsTranscriptionWhenFirstLinesDontMatchTitleArtist(t *testing.T) {
	// Don't trim anything if the doc doesn't actually start with the
	// title/artist lines — some digested docs might not follow the
	// convention exactly, and blind trimming would eat real content.
	blocks := []transcription.Block{
		{Kind: transcription.SectionHeader, Text: "INTRO:"},
		{Kind: transcription.TextLine, Text: "some other content"},
	}
	song := db.Song{Title: "Downtown", Artist: "Petula Clark"}
	html := renderSongShow(t, song, blocks, true, false)

	if !strings.Contains(html, "INTRO:") || !strings.Contains(html, "some other content") {
		t.Errorf("expected transcription content to render unmodified, got: %s", html)
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
