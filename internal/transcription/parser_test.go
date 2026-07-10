package transcription

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBlankLineIsTextLine(t *testing.T) {
	// strings.Split("\n", "\n") yields ["", ""] — a leading blank line plus
	// the trailing-newline marker — so assert the round-trip invariant
	// rather than an exact block count, which is an implementation detail.
	blocks := Parse("\n")
	if blocks[0].Kind != TextLine || blocks[0].Text != "" {
		t.Fatalf("got %+v, want first block to be an empty TextLine", blocks)
	}
	if got := Render(blocks); got != "\n" {
		t.Errorf("Render() = %q, want %q", got, "\n")
	}
}

func TestParseSectionHeader(t *testing.T) {
	// No trailing \n here deliberately — this test is about classifying a
	// single line, not end-of-file newline handling (covered elsewhere).
	blocks := Parse("CHORUS:")
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if blocks[0].Kind != SectionHeader || blocks[0].Text != "CHORUS:" {
		t.Errorf("got %+v, want SectionHeader %q", blocks[0], "CHORUS:")
	}
}

func TestParseSectionHeaderWithNumberAndHyphen(t *testing.T) {
	for _, header := range []string{"VERSE 1:", "PRE-CHORUS:", "END:"} {
		blocks := Parse(header + "\n")
		if blocks[0].Kind != SectionHeader {
			t.Errorf("Parse(%q)[0].Kind = %v, want SectionHeader", header, blocks[0].Kind)
		}
	}
}

func TestParseChordLyricPairInterleavesTokens(t *testing.T) {
	// No trailing \n — isolates the pairing/tokenizing behavior from
	// end-of-file newline handling (covered by the full-file round-trip test).
	input := "E                 A7                E                  A7\n" +
		"  I can't get no     satisfaction,     I can't get no      satisfaction."
	blocks := Parse(input)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	b := blocks[0]
	if b.Kind != ChordLyricPair {
		t.Fatalf("Kind = %v, want ChordLyricPair", b.Kind)
	}

	var chords []string
	for _, tok := range b.Tokens {
		if tok.Chord != "" {
			chords = append(chords, tok.Chord)
		}
	}
	want := []string{"E", "A7", "E", "A7"}
	if len(chords) != len(want) {
		t.Fatalf("chords = %v, want %v", chords, want)
	}
	for i, c := range want {
		if chords[i] != c {
			t.Errorf("chords[%d] = %q, want %q", i, chords[i], c)
		}
	}

	// First token must be the chord "E" (anchored at column 0), not text —
	// that's the whole point of interleaving vs. a separate offset table.
	if b.Tokens[0].Chord != "E" {
		t.Errorf("Tokens[0] = %+v, want chord E first", b.Tokens[0])
	}
}

func TestParseChordOnlyLineWhenNextLineBlank(t *testing.T) {
	input := "E   A   E   A\n\n"
	blocks := Parse(input)
	if blocks[0].Kind != ChordOnlyLine {
		t.Fatalf("Kind = %v, want ChordOnlyLine", blocks[0].Kind)
	}
}

func TestParseTrailingAnnotationOnChordLine(t *testing.T) {
	// Real line from the catalog: a chord line with a trailing repeat-count
	// annotation that isn't itself chord-shaped ("3rd x: Girl reaction").
	input := "         E         B7        E         A            3rd x: Girl reaction\n" +
		"'Cause I try and I try and I try and I try.\n"
	blocks := Parse(input)
	b := blocks[0]
	if b.Kind != ChordLyricPair {
		t.Fatalf("Kind = %v, want ChordLyricPair", b.Kind)
	}
	if b.Annotation == "" {
		t.Fatal("Annotation is empty, want trailing '3rd x: Girl reaction' text preserved")
	}
	if got, want := b.Annotation, "3rd x: Girl reaction"; !endsWith(got, want) {
		t.Errorf("Annotation = %q, want to end with %q", got, want)
	}
}

func TestParseRepeatReferenceIsTextLine(t *testing.T) {
	blocks := Parse("(CHORUS)\n")
	if blocks[0].Kind != TextLine || blocks[0].Text != "(CHORUS)" {
		t.Errorf("got %+v, want TextLine (CHORUS)", blocks[0])
	}
}

func TestParseIntroLineWithLabelFallsBackToTextLine(t *testing.T) {
	// "INTRO:  b b b c# ..." mixes a section-style label with chord content
	// on one line. MVP doesn't special-case this hybrid shape — it's stored
	// verbatim as a TextLine rather than guessed at. Revisit once the full
	// catalog shows how common this pattern actually is.
	input := "INTRO:  b b b c# d d d c# c# b     E  (E6)  E7/D   E6/D   E/D  x2\n"
	blocks := Parse(input)
	if blocks[0].Kind != TextLine {
		t.Fatalf("Kind = %v, want TextLine (fallback for label+chords hybrid line)", blocks[0].Kind)
	}
	if blocks[0].Text != "INTRO:  b b b c# d d d c# c# b     E  (E6)  E7/D   E6/D   E/D  x2" {
		t.Errorf("Text = %q, want verbatim original line", blocks[0].Text)
	}
}

func TestParseBarTableFormatUsesChordLyricPairModel(t *testing.T) {
	input := "|   E             /D | x3       |         E        /D | x3              E\n" +
		"| I can't get no,    | x3       | no satisfaction,    | x3      no satisfaction\n"
	blocks := Parse(input)
	if blocks[0].Kind != ChordLyricPair {
		t.Fatalf("Kind = %v, want ChordLyricPair (bar format needs no special casing)", blocks[0].Kind)
	}
}

func TestParseHandlesUnicodeApostropheColumnsCorrectly(t *testing.T) {
	// The real catalog uses a Unicode right single quote (U+2019) in
	// contractions. It must count as ONE column, not three UTF-8 bytes,
	// or every chord after it on the line would misalign.
	input := "E        A\nWhen I’m here    now\n"
	blocks := Parse(input)
	b := blocks[0]
	if len(b.Tokens) == 0 {
		t.Fatal("expected tokens")
	}
	// Re-render must reproduce the exact original spacing despite the
	// multi-byte rune. Split/Join symmetry means the trailing newline
	// round-trips too, so we compare against the full original input.
	if got := Render(blocks); got != input {
		t.Errorf("Render() = %q, want %q", got, input)
	}
}

func TestParseFullSatisfactionFileRoundTrips(t *testing.T) {
	raw := readFixture(t, "satisfaction.txt")
	blocks := Parse(raw)
	rendered := Render(blocks)
	if rendered != raw {
		t.Errorf("round-trip mismatch.\n--- got ---\n%s\n--- want ---\n%s", rendered, raw)
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return string(data)
}
