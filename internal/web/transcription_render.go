package web

import (
	"strings"
	"unicode/utf8"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/transcription"
)

// renderTranscriptionHTML mirrors transcription.Render's column-alignment
// logic, but emits chords as bolded, upper-cased <b> nodes instead of plain
// text — matching how chords appear in Jeff's Google Docs. transcription.Render
// itself must stay untouched: it round-trips raw text byte-for-byte and is
// relied on elsewhere for that guarantee.
func renderTranscriptionHTML(blocks []transcription.Block) g.Node {
	var nodes []g.Node
	for i, b := range blocks {
		if i > 0 {
			nodes = append(nodes, g.Text("\n"))
		}
		switch b.Kind {
		case transcription.SectionHeader, transcription.TextLine:
			nodes = append(nodes, g.Text(b.Text))
		case transcription.ChordOnlyLine:
			nodes = append(nodes, chordLineNodes(b.Tokens, b.Annotation)...)
		case transcription.ChordLyricPair:
			nodes = append(nodes, chordLineNodes(b.Tokens, b.Annotation)...)
			nodes = append(nodes, g.Text("\n"))
			nodes = append(nodes, g.Text(lyricLineText(b.Tokens)))
		}
	}
	return Pre(Class("transcription"), g.Group(nodes))
}

func chordLineNodes(tokens []transcription.Token, annotation string) []g.Node {
	var nodes []g.Node
	col := 0
	chordLen := 0
	for _, t := range tokens {
		if t.Chord != "" {
			if col > chordLen {
				nodes = append(nodes, g.Text(strings.Repeat(" ", col-chordLen)))
				chordLen = col
			}
			nodes = append(nodes, B(g.Text(strings.ToUpper(t.Chord))))
			chordLen += utf8.RuneCountInString(t.Chord)
		} else {
			col += utf8.RuneCountInString(t.Text)
		}
	}
	if annotation != "" {
		nodes = append(nodes, g.Text(annotation))
	}
	return nodes
}

func lyricLineText(tokens []transcription.Token) string {
	var b strings.Builder
	for _, t := range tokens {
		if t.Chord == "" && !t.Synthetic {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}
