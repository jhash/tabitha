package transcription

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// sectionHeaderRe matches standalone label lines like "CHORUS:", "VERSE 1:",
// "PRE-CHORUS:", "END:". The colon must be the last character — a line like
// "INTRO:  b b b ..." has content after the label and intentionally does not
// match (see TestParseIntroLineWithLabelFallsBackToTextLine).
var sectionHeaderRe = regexp.MustCompile(`^[A-Z][A-Z0-9 '()/-]*:$`)

// chordTokenRe matches anything that can sit in a "chord row": real chord
// symbols (root + accidental + quality + extension + slash bass), bare
// lowercase note letters (bass-walk notation), repeat counts ("x2"), bar
// delimiters ("|"), and arbitrary parenthesized annotations ("(drums)",
// "(E6)"). It's deliberately loose — a heuristic for "what Jeff put in the
// chord row", not a music-theory validator.
var chordTokenRe = regexp.MustCompile(`(?i)^(\([^)]*\)|x[0-9]+|\||[a-g](#|b)?(maj|min|dim|aug|sus2|sus4|sus|add2|add4|add6|add9)?[0-9]{0,2}(/[a-g](#|b)?)?)$`)

// word is a whitespace-delimited token from a line, with its rune (not
// byte) start column — required for correct alignment when the source
// contains multi-byte runes like a Unicode right single quote.
type word struct {
	text     string
	startCol int
}

func tokenizeWords(line []rune) []word {
	var words []word
	i := 0
	for i < len(line) {
		for i < len(line) && line[i] == ' ' {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		for i < len(line) && line[i] != ' ' {
			i++
		}
		words = append(words, word{text: string(line[start:i]), startCol: start})
	}
	return words
}

func isChordLineCandidate(line string) bool {
	words := tokenizeWords([]rune(line))
	if len(words) < 2 {
		return false
	}
	return chordTokenRe.MatchString(words[0].text) && chordTokenRe.MatchString(words[1].text)
}

// splitChordRun consumes leading chord-shaped words and reports the rune
// column where consumption stopped (end of the last consumed word, or 0 if
// none). Everything from that column to end-of-line — including whatever
// whitespace separates it from the last real chord — becomes the block's
// Annotation, captured verbatim so it round-trips without extra padding math.
func splitChordRun(words []word) (chordWords []word, consumedEndCol int) {
	for _, w := range words {
		if !chordTokenRe.MatchString(w.text) {
			break
		}
		chordWords = append(chordWords, w)
		consumedEndCol = w.startCol + utf8.RuneCountInString(w.text)
	}
	return chordWords, consumedEndCol
}

// Parse converts raw transcription plaintext into an ordered Block list.
func Parse(input string) []Block {
	lines := strings.Split(input, "\n")
	var blocks []Block
	i := 0
	for i < len(lines) {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
			blocks = append(blocks, Block{Kind: TextLine, Text: line})
			i++
		case sectionHeaderRe.MatchString(strings.TrimSpace(line)):
			blocks = append(blocks, Block{Kind: SectionHeader, Text: line})
			i++
		case isChordLineCandidate(line):
			if i+1 < len(lines) && canPairAsLyric(lines[i+1]) {
				blocks = append(blocks, parseChordLyricPair(line, lines[i+1]))
				i += 2
			} else {
				blocks = append(blocks, parseChordOnlyLine(line))
				i++
			}
		default:
			blocks = append(blocks, Block{Kind: TextLine, Text: line})
			i++
		}
	}
	return blocks
}

func canPairAsLyric(next string) bool {
	if strings.TrimSpace(next) == "" {
		return false
	}
	if sectionHeaderRe.MatchString(strings.TrimSpace(next)) {
		return false
	}
	return !isChordLineCandidate(next)
}

func parseChordOnlyLine(line string) Block {
	runes := []rune(line)
	words := tokenizeWords(runes)
	chordWords, consumedEnd := splitChordRun(words)

	var tokens []Token
	prevCol := 0
	for _, w := range chordWords {
		if gap := w.startCol - prevCol; gap > 0 {
			tokens = append(tokens, Token{Text: strings.Repeat(" ", gap), Synthetic: true})
		}
		tokens = append(tokens, Token{Chord: w.text})
		prevCol = w.startCol
	}

	b := Block{Kind: ChordOnlyLine, Tokens: tokens}
	if consumedEnd < len(runes) {
		b.Annotation = string(runes[consumedEnd:])
	}
	return b
}

func parseChordLyricPair(chordLine, lyricLine string) Block {
	chordRunes := []rune(chordLine)
	lyricRunes := []rune(lyricLine)
	words := tokenizeWords(chordRunes)
	chordWords, consumedEnd := splitChordRun(words)

	var tokens []Token
	prevCol := 0
	for _, w := range chordWords {
		real, padLen := sliceRealAndPad(lyricRunes, prevCol, w.startCol)
		if real != "" {
			tokens = append(tokens, Token{Text: real})
		}
		if padLen > 0 {
			tokens = append(tokens, Token{Text: strings.Repeat(" ", padLen), Synthetic: true})
		}
		tokens = append(tokens, Token{Chord: w.text})
		prevCol = w.startCol
	}
	if prevCol < len(lyricRunes) {
		tokens = append(tokens, Token{Text: string(lyricRunes[prevCol:])})
	}

	b := Block{Kind: ChordLyricPair, Tokens: tokens}
	if consumedEnd < len(chordRunes) {
		b.Annotation = string(chordRunes[consumedEnd:])
	}
	return b
}

// sliceRealAndPad splits the (end-start)-rune gap into real lyric characters
// and however much synthetic padding is needed to preserve the chord line's
// original column spacing when the lyric line is shorter. This is what makes
// rendering reproduce the original chord line's spacing even when the lyric
// line ran out of characters first, without that padding leaking into the
// reconstructed lyric line (see Token.Synthetic).
func sliceRealAndPad(runes []rune, start, end int) (real string, padLen int) {
	if start >= end {
		return "", 0
	}
	if start >= len(runes) {
		return "", end - start
	}
	realEnd := end
	if realEnd > len(runes) {
		realEnd = len(runes)
	}
	return string(runes[start:realEnd]), end - realEnd
}

// Render reverses Parse, reproducing the original text byte-for-byte.
// Column positions are derived from token order, never stored.
func Render(blocks []Block) string {
	lines := make([]string, 0, len(blocks)*2)
	for _, b := range blocks {
		switch b.Kind {
		case SectionHeader, TextLine:
			lines = append(lines, b.Text)
		case ChordOnlyLine:
			lines = append(lines, renderChordLine(b.Tokens, b.Annotation))
		case ChordLyricPair:
			lines = append(lines, renderChordLine(b.Tokens, b.Annotation))
			lines = append(lines, renderLyricLine(b.Tokens))
		}
	}
	return strings.Join(lines, "\n")
}

func renderChordLine(tokens []Token, annotation string) string {
	var b strings.Builder
	col := 0
	chordLen := 0
	for _, t := range tokens {
		if t.Chord != "" {
			if col > chordLen {
				b.WriteString(strings.Repeat(" ", col-chordLen))
				chordLen = col
			}
			b.WriteString(t.Chord)
			chordLen += utf8.RuneCountInString(t.Chord)
		} else {
			col += utf8.RuneCountInString(t.Text)
		}
	}
	b.WriteString(annotation)
	return b.String()
}

func renderLyricLine(tokens []Token) string {
	var b strings.Builder
	for _, t := range tokens {
		if t.Chord == "" && !t.Synthetic {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}
