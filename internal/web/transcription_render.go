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

	// Bold/Italic/Underline come from whichever Token's Text started this
	// word (see splitIntoChordWords) — if a word is stitched together from
	// multiple Tokens with different marks (a rare edge case: a mid-word
	// chord that's also a mark boundary), only the first token's marks are
	// kept, rather than tracking per-character formatting.
	Bold      bool
	Italic    bool
	Underline bool
}

// splitIntoChordWords walks a ChordLyricPair/ChordOnlyLine's token stream
// and regroups it at word granularity: a chord token attaches to the next
// real word encountered (matching how the original space-aligned chart
// read — a chord sits above the word that follows it), consecutive chords
// with no lyric between them each become their own chordless-word entry,
// and synthetic alignment padding (meaningless once we're not
// reconstructing fixed columns) is dropped entirely.
//
// Text is processed rune-by-rune across token boundaries (rather than
// per-token via strings.Fields) so a chord landing mid-word — which
// stores the split location as two adjacent Text tokens with no
// whitespace between them, e.g. "yo" / "u" for "you" — doesn't fragment
// that word into two separately-wrapping chord-word units. Only actual
// whitespace in the underlying text ends a word.
func splitIntoChordWords(tokens []transcription.Token) []chordWord {
	var words []chordWord
	var buf strings.Builder
	pendingChord := ""
	havePending := false
	var bold, italic, underline bool

	flush := func() {
		if buf.Len() > 0 || havePending {
			words = append(words, chordWord{
				Chord: pendingChord, Word: buf.String(),
				Bold: bold, Italic: italic, Underline: underline,
			})
		}
		buf.Reset()
		pendingChord = ""
		havePending = false
		bold, italic, underline = false, false, false
	}

	for _, t := range tokens {
		if t.Chord != "" {
			if havePending {
				flush()
			}
			pendingChord = t.Chord
			havePending = true
			continue
		}
		if t.Synthetic {
			continue
		}
		for _, r := range t.Text {
			if r == ' ' || r == '\t' || r == '\n' {
				flush()
			} else {
				if buf.Len() == 0 {
					bold, italic, underline = t.Bold, t.Italic, t.Underline
				}
				buf.WriteRune(r)
			}
		}
	}
	flush()
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
			nodes[i] = Div(Class("text-line"), textLineContent(b))
		case transcription.ChordOnlyLine, transcription.ChordLyricPair:
			nodes[i] = chordLineNode(b)
		}
	}
	return Div(Class("transcription"), g.Group(nodes))
}

// textLineContent renders a TextLine's Bold/Italic/Underline marks when the
// editor has set Tokens (see blocks.go) — falling back to the plain Text
// string for lines with no formatting, or stored before this feature.
func textLineContent(b transcription.Block) g.Node {
	if len(b.Tokens) == 0 {
		return g.Text(b.Text)
	}
	nodes := make([]g.Node, 0, len(b.Tokens))
	for _, t := range b.Tokens {
		var node g.Node = g.Text(t.Text)
		if t.Underline {
			node = U(node)
		}
		if t.Italic {
			node = Em(node)
		}
		if t.Bold {
			node = Strong(node)
		}
		nodes = append(nodes, node)
	}
	return g.Group(nodes)
}

func chordLineNode(b transcription.Block) g.Node {
	words := splitIntoChordWords(b.Tokens)
	children := make([]g.Node, 0, len(words)+1)
	for _, w := range words {
		children = append(children, Span(Class("chord-word"),
			Span(Class("chord"), g.Text(w.Chord)),
			Span(Class("lyric"), lyricWordNode(w)),
		))
	}
	if b.Annotation != "" {
		children = append(children, Span(Class("annotation"), g.Text(b.Annotation)))
	}
	return Div(Class("chord-line"), g.Group(children))
}

// lyricWordNode wraps a chordWord's text in <strong>/<em>/<u> per its
// marks (see splitIntoChordWords) — nested in that order when more than
// one applies.
func lyricWordNode(w chordWord) g.Node {
	var node g.Node = g.Text(w.Word)
	if w.Underline {
		node = U(node)
	}
	if w.Italic {
		node = Em(node)
	}
	if w.Bold {
		node = Strong(node)
	}
	return node
}

// transposeControlsNode renders the on-the-fly chord-transposition
// control: a semitone +/- stepper plus a live key readout, wired up by
// static/js/transpose.js against this page's .chord spans. It's a pure
// client-side string rewrite over already-rendered DOM — no server
// round-trip — so it works the same online or from the offline cache
// (see static/js/offline-db.js). key is the song's stored/detected key
// (may be ""), used only to seed the starting spelling; shown as a plain
// semitone offset instead when unknown.
func transposeControlsNode(key string) g.Node {
	return Div(Class("transpose-controls"), g.Attr("data-key", key),
		Button(Class("transpose-down"), Type("button"), g.Attr("aria-label", "Transpose down a semitone"), g.Text("−")),
		Span(Class("transpose-key"), g.Text(transposeKeyPlaceholder(key))),
		Button(Class("transpose-up"), Type("button"), g.Attr("aria-label", "Transpose up a semitone"), g.Text("+")),
	)
}

func transposeKeyPlaceholder(key string) string {
	if key == "" {
		return "0"
	}
	return key
}

// transposeScript loads static/js/transpose.js as a classic (non-module)
// script — deliberately not type="module", since it's embedded directly
// in boosted page content and needs to re-run on every htmx-boosted
// navigation between songs (see the file's own doc comment for why a
// module wouldn't).
func transposeScript() g.Node {
	return Script(Src(versionedHref("/static/js/transpose.js", assets.TransposeJS)))
}
