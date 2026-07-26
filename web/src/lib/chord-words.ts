import type { TranscriptionToken } from './schemas'

/**
 * Direct port of internal/web/transcription_render.go's
 * `splitIntoChordWords`. Behaviour is intentionally identical — the Go
 * tests in transcription_render_test.go describe this function, and
 * porting the tests alongside it is what proves the migration didn't
 * quietly change how a chart reads.
 */

/**
 * One wrappable chord-chart unit: a chord (possibly empty, for a lyric
 * word with no chord change) glued to the single lyric word it sits
 * above. Rendering each as its own flex item — rather than rebuilding
 * fixed monospace columns — is what lets a chart reflow on a narrow
 * screen without losing which word each chord belongs to.
 */
export interface ChordWord {
  chord: string
  word: string
  /**
   * Taken from whichever token started the word. A word stitched from
   * several tokens with differing marks (rare: a mid-word chord that's
   * also a mark boundary) keeps only the first token's — same
   * simplification the Go version makes, rather than tracking
   * per-character formatting.
   */
  bold: boolean
  italic: boolean
  underline: boolean
}

/**
 * Regroups a chord line's token stream at word granularity:
 *
 *   - a chord attaches to the next real word (matching how the original
 *     space-aligned chart read — a chord sits above the word following it)
 *   - consecutive chords with no lyric between them each become their own
 *     chordless entry
 *   - synthetic alignment padding is dropped, being meaningless once
 *     we're not reconstructing fixed columns
 *
 * Text is walked code point by code point ACROSS token boundaries rather
 * than split per-token, because a chord landing mid-word stores the split
 * as two adjacent text tokens with no whitespace between them ("yo" +
 * "u"). Splitting per token would fragment "you" into two separately
 * wrapping units. Only real whitespace ends a word.
 */
export function splitIntoChordWords(
  tokens: ReadonlyArray<TranscriptionToken>,
): Array<ChordWord> {
  const words: Array<ChordWord> = []

  let buf = ''
  let pendingChord = ''
  let havePending = false
  let bold = false
  let italic = false
  let underline = false

  const flush = () => {
    // `havePending` is what keeps a trailing chord with no lyric under it
    // (an instrumental line's last chord) from being dropped.
    if (buf.length > 0 || havePending) {
      words.push({ chord: pendingChord, word: buf, bold, italic, underline })
    }
    buf = ''
    pendingChord = ''
    havePending = false
    bold = false
    italic = false
    underline = false
  }

  for (const token of tokens) {
    if (token.chord) {
      if (havePending) flush()
      pendingChord = token.chord
      havePending = true
      continue
    }
    if (token.synthetic) continue

    // for..of iterates code points, matching Go's `for _, r := range s`.
    // Indexing would split surrogate pairs mid-character.
    for (const ch of token.text ?? '') {
      if (ch === ' ' || ch === '\t' || ch === '\n') {
        flush()
      } else {
        if (buf.length === 0) {
          bold = token.bold ?? false
          italic = token.italic ?? false
          underline = token.underline ?? false
        }
        buf += ch
      }
    }
  }
  flush()

  return words
}
