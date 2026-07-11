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

func TestSplitIntoChordWordsAttachesChordToFollowingWord(t *testing.T) {
	tokens := []transcription.Token{
		{Chord: "e"},
		{Text: "I can't get no satisfaction"},
	}
	got := splitIntoChordWords(tokens)
	want := []chordWord{
		{Chord: "e", Word: "I"},
		{Word: "can't"},
		{Word: "get"},
		{Word: "no"},
		{Word: "satisfaction"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d words, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("word[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSplitIntoChordWordsHandlesConsecutiveChordsWithNoLyric(t *testing.T) {
	tokens := []transcription.Token{
		{Chord: "e"},
		{Chord: "a7"},
	}
	got := splitIntoChordWords(tokens)
	want := []chordWord{{Chord: "e"}, {Chord: "a7"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSplitIntoChordWordsSkipsSyntheticPadding(t *testing.T) {
	tokens := []transcription.Token{
		{Text: "    ", Synthetic: true},
		{Chord: "a7"},
	}
	got := splitIntoChordWords(tokens)
	want := []chordWord{{Chord: "a7"}}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSplitIntoChordWordsHandlesWordsWithoutAChord(t *testing.T) {
	tokens := []transcription.Token{{Text: "hello world"}}
	got := splitIntoChordWords(tokens)
	want := []chordWord{{Word: "hello"}, {Word: "world"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestRenderTranscriptionHTMLBoldsAndUppercasesChordsPerWord(t *testing.T) {
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
	if !strings.Contains(html, `<span class="chord">E</span>`) {
		t.Errorf("expected chord E rendered in its own span, got: %s", html)
	}
	if !strings.Contains(html, `<span class="lyric">I</span>`) {
		t.Errorf("expected the chord's word (\"I\") to render alongside it, got: %s", html)
	}
	if !strings.Contains(html, `<span class="lyric">can&#39;t</span>`) {
		t.Errorf("expected subsequent words to render as their own wrappable unit, got: %s", html)
	}
}

func TestRenderTranscriptionHTMLEachChordWordIsIndependentlyWrappable(t *testing.T) {
	// The whole point of this redesign: each chord+word pair is its own
	// flex item (chord-word), not a single monospace-padded line — that's
	// what lets the browser wrap chord charts on narrow screens while
	// keeping each chord glued to the word it belongs above.
	blocks := []transcription.Block{
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "e"},
				{Text: "one two"},
				{Chord: "a"},
				{Text: " three"},
			},
		},
	}
	html := renderTranscriptionHTMLString(t, blocks)
	if strings.Count(html, `class="chord-word"`) != 3 {
		t.Errorf("expected 3 chord-word units (one/two/three), got: %s", html)
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

func TestRenderTranscriptionHTMLRendersChordOnlyLineChords(t *testing.T) {
	blocks := []transcription.Block{
		{
			Kind: transcription.ChordOnlyLine,
			Tokens: []transcription.Token{
				{Text: "    ", Synthetic: true},
				{Chord: "a7"},
			},
		},
	}
	html := renderTranscriptionHTMLString(t, blocks)
	if !strings.Contains(html, `<span class="chord">A7</span>`) {
		t.Errorf("expected a bolded, upper-cased A7 chord, got: %s", html)
	}
}

func TestRenderTranscriptionHTMLRendersAnnotationAsTrailingWrappableText(t *testing.T) {
	blocks := []transcription.Block{
		{
			Kind:       transcription.ChordLyricPair,
			Tokens:     []transcription.Token{{Chord: "e"}, {Text: "hi"}},
			Annotation: "3rd x: Girl reaction",
		},
	}
	html := renderTranscriptionHTMLString(t, blocks)
	if !strings.Contains(html, `<span class="annotation">3rd x: Girl reaction</span>`) {
		t.Errorf("expected the annotation to render as its own trailing span, got: %s", html)
	}
}
