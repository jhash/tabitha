package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/transcription"
)

func renderTranscriptionHTMLString(t *testing.T, blocks []transcription.Block) string {
	t.Helper()
	var buf bytes.Buffer
	if err := renderTranscriptionHTML(blocks).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestRenderTranscriptionHTMLBoldsChords(t *testing.T) {
	blocks := []transcription.Block{
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "e"},
				{Text: "I can't get no satisfaction"},
			},
		},
	}
	html := renderTranscriptionHTMLString(t, blocks)
	if !strings.Contains(html, "<b>E</b>") {
		t.Errorf("expected chord to render bolded and upper-cased as <b>E</b>, got: %s", html)
	}
	if !strings.Contains(html, "I can&#39;t get no satisfaction") {
		t.Errorf("expected lyric line to still render, got: %s", html)
	}
}

func TestRenderTranscriptionHTMLPreservesSectionHeadersAndTextLines(t *testing.T) {
	blocks := []transcription.Block{
		{Kind: transcription.SectionHeader, Text: "CHORUS:"},
		{Kind: transcription.TextLine, Text: "(repeat)"},
	}
	html := renderTranscriptionHTMLString(t, blocks)
	if !strings.Contains(html, "CHORUS:") || !strings.Contains(html, "(repeat)") {
		t.Errorf("expected section header and text line to render, got: %s", html)
	}
}

func TestRenderTranscriptionHTMLPreservesChordOnlyLineAlignment(t *testing.T) {
	blocks := []transcription.Block{
		{
			Kind: transcription.ChordOnlyLine,
			Tokens: []transcription.Token{
				{Text: "    "},
				{Chord: "a7"},
			},
		},
	}
	html := renderTranscriptionHTMLString(t, blocks)
	if !strings.Contains(html, "    <b>A7</b>") {
		t.Errorf("expected 4 spaces of padding before bolded chord, got: %s", html)
	}
}
