// Package transcription parses and renders Jeffrey's chord-over-lyric chart
// format. Chords are stored as an interleaved token stream (ChordPro-style)
// rather than a separate column-offset table, so edits to lyric text never
// desynchronize a stored chord position — see docs/plans for the reasoning.
package transcription

// BlockKind identifies what kind of line (or line pair) a Block represents.
type BlockKind int

const (
	// SectionHeader is a standalone label line, e.g. "CHORUS:".
	SectionHeader BlockKind = iota
	// ChordLyricPair is a chord line paired with the lyric line beneath it.
	ChordLyricPair
	// ChordOnlyLine is a chord line with no paired lyric (intros, instrumentals).
	ChordOnlyLine
	// TextLine is a catch-all: blank lines, repeat-references like
	// "(CHORUS)", stage directions, or any line that isn't confidently one
	// of the above. Rendered verbatim.
	TextLine
)

// Token is one element of a ChordLyricPair/ChordOnlyLine's content stream.
// Exactly one of Chord or Text is set.
type Token struct {
	Chord string
	Text  string

	// Synthetic marks a Text token as alignment padding we invented (the
	// chord line extends further right than the real lyric text), rather
	// than characters that actually appeared in the source lyric line. Its
	// length still counts toward chord-column math, but it's excluded when
	// reconstructing the lyric line itself.
	Synthetic bool
}

// Block is one unit of a parsed transcription, in document order.
type Block struct {
	Kind BlockKind

	// Text holds the verbatim line content for SectionHeader and TextLine.
	Text string

	// Tokens holds the interleaved chord/text stream for ChordLyricPair and
	// ChordOnlyLine.
	Tokens []Token

	// Annotation holds any trailing content on a chord line that isn't
	// itself chord-shaped (e.g. "3rd x: Girl reaction"), captured verbatim
	// including its leading whitespace so it round-trips exactly.
	Annotation string
}
