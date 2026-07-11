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
	Chord string `json:"chord,omitempty"`
	Text  string `json:"text,omitempty"`

	// Synthetic marks a Text token as alignment padding we invented (the
	// chord line extends further right than the real lyric text), rather
	// than characters that actually appeared in the source lyric line. Its
	// length still counts toward chord-column math, but it's excluded when
	// reconstructing the lyric line itself.
	Synthetic bool `json:"synthetic,omitempty"`

	// Bold/Italic/Underline are set only via the editor (never derived from
	// parsing raw text — Render/the parser round-trip plain ASCII and stay
	// untouched by these) and only apply to Text tokens.
	Bold      bool `json:"bold,omitempty"`
	Italic    bool `json:"italic,omitempty"`
	Underline bool `json:"underline,omitempty"`
}

// Block is one unit of a parsed transcription, in document order.
type Block struct {
	Kind BlockKind `json:"kind"`

	// Text holds the verbatim line content for SectionHeader and TextLine.
	// For TextLine, this stays populated as a plain-string convenience for
	// existing consumers (byline dedup, "Key:" detection) even when Tokens
	// is also set below — it's always the same content, just without marks.
	Text string `json:"text,omitempty"`

	// Tokens holds the interleaved chord/text stream for ChordLyricPair and
	// ChordOnlyLine. For TextLine it optionally carries the same content as
	// Text, but as Bold/Italic/Underline-aware tokens (chordless) — set
	// only by the editor, when the line has marks worth preserving; absent
	// for text lines with no formatting or that predate this feature.
	Tokens []Token `json:"tokens,omitempty"`

	// Annotation holds any trailing content on a chord line that isn't
	// itself chord-shaped (e.g. "3rd x: Girl reaction"), captured verbatim
	// including its leading whitespace so it round-trips exactly.
	Annotation string `json:"annotation,omitempty"`
}
