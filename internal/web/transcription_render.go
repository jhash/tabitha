package web

import (
	"strings"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/transcription"
)

// chordWord is one wrappable chord-chart unit: a chord (possibly empty,
// for a lyric word with no chord change) glued to the single lyric word it
// sits above. Rendering each of these as its own flex item — rather than
// reconstructing fixed monospace columns — is what lets a chord chart
// reflow on a narrow screen without losing which word each chord belongs
// to (see TestRenderTranscriptionHTMLEachChordWordIsIndependentlyWrappable).
type chordWord struct {
	Chord string
	Word  string
}

// splitIntoChordWords walks a ChordLyricPair/ChordOnlyLine's token stream
// and regroups it at word granularity: a chord token attaches to the next
// real word encountered (matching how the original space-aligned chart
// read — a chord sits above the word that follows it), consecutive chords
// with no lyric between them each become their own chordless-word entry,
// and synthetic alignment padding (meaningless once we're not
// reconstructing fixed columns) is dropped entirely.
func splitIntoChordWords(tokens []transcription.Token) []chordWord {
	var words []chordWord
	pendingChord := ""
	havePending := false
	for _, t := range tokens {
		if t.Chord != "" {
			if havePending {
				words = append(words, chordWord{Chord: pendingChord})
			}
			pendingChord = t.Chord
			havePending = true
			continue
		}
		if t.Synthetic {
			continue
		}
		for _, w := range strings.Fields(t.Text) {
			if havePending {
				words = append(words, chordWord{Chord: pendingChord, Word: w})
				pendingChord = ""
				havePending = false
			} else {
				words = append(words, chordWord{Word: w})
			}
		}
	}
	if havePending {
		words = append(words, chordWord{Chord: pendingChord})
	}
	return words
}

// renderTranscriptionHTML renders a parsed transcription for display: each
// chord-word wraps independently (see chordWord) instead of relying on a
// monospace <pre> grid, so a chord chart reflows readably on narrow
// screens instead of requiring horizontal scrolling. transcription.Render
// itself must stay untouched — it round-trips raw text byte-for-byte and
// is relied on elsewhere for that guarantee; this is a separate,
// display-only rendering path over the same token stream.
func renderTranscriptionHTML(blocks []transcription.Block) g.Node {
	nodes := make([]g.Node, len(blocks))
	for i, b := range blocks {
		switch b.Kind {
		case transcription.SectionHeader:
			nodes[i] = Div(Class("section-header"), g.Text(b.Text))
		case transcription.TextLine:
			nodes[i] = Div(Class("text-line"), g.Text(b.Text))
		case transcription.ChordOnlyLine, transcription.ChordLyricPair:
			nodes[i] = chordLineNode(b)
		}
	}
	return Div(Class("transcription"), g.Group(nodes))
}

func chordLineNode(b transcription.Block) g.Node {
	words := splitIntoChordWords(b.Tokens)
	children := make([]g.Node, 0, len(words)+1)
	for _, w := range words {
		children = append(children, Span(Class("chord-word"),
			Span(Class("chord"), g.Text(strings.ToUpper(w.Chord))),
			Span(Class("lyric"), g.Text(w.Word)),
		))
	}
	if b.Annotation != "" {
		children = append(children, Span(Class("annotation"), g.Text(b.Annotation)))
	}
	return Div(Class("chord-line"), g.Group(children))
}
