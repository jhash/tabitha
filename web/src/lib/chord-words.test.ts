import { describe, expect, it } from 'vitest'

import { splitIntoChordWords } from './chord-words'
import type { ChordWord } from './chord-words'
import type { TranscriptionToken } from './schemas'

/**
 * Parity suite for the port of internal/web/transcription_render.go.
 *
 * Each case mirrors a test in transcription_render_test.go by name, so a
 * behavioural drift between the Go renderer and this one fails here
 * rather than showing up as a subtly misaligned chart in production.
 * Both implementations must stay live during the migration, so this is
 * the contract between them.
 */

const plain = (word: string, chord = ''): ChordWord => ({
  chord,
  word,
  bold: false,
  italic: false,
  underline: false,
})

describe('splitIntoChordWords', () => {
  // TestSplitIntoChordWordsAttachesChordToFollowingWord
  it('attaches a chord to the word that follows it', () => {
    const tokens: Array<TranscriptionToken> = [
      { chord: 'e' },
      { text: "I can't get no satisfaction" },
    ]
    expect(splitIntoChordWords(tokens)).toEqual([
      plain('I', 'e'),
      plain("can't"),
      plain('get'),
      plain('no'),
      plain('satisfaction'),
    ])
  })

  // TestSplitIntoChordWordsHandlesConsecutiveChordsWithNoLyric
  it('gives each of two consecutive chords its own chordless entry', () => {
    expect(splitIntoChordWords([{ chord: 'e' }, { chord: 'a7' }])).toEqual([
      plain('', 'e'),
      plain('', 'a7'),
    ])
  })

  // TestSplitIntoChordWordsSkipsSyntheticPadding
  it('drops synthetic alignment padding', () => {
    const tokens: Array<TranscriptionToken> = [
      { text: '    ', synthetic: true },
      { chord: 'a7' },
    ]
    expect(splitIntoChordWords(tokens)).toEqual([plain('', 'a7')])
  })

  // TestSplitIntoChordWordsReunitesAWordSplitByAMidWordChord
  it('reunites a word split by a mid-word chord', () => {
    // Real production data. A chord landing mid-word stores the split as
    // two adjacent text tokens with no whitespace ("yo" / "u") — treating
    // each as its own word fragments "you" into two separately-wrapping
    // units with a visible gap. Note the chord binds to the REUNITED
    // word, not to the fragment after it.
    const tokens: Array<TranscriptionToken> = [
      { chord: 'Db' }, { text: 'Look into my eyes, yo' },
      { chord: 'Ab' }, { text: 'u  will see what you mean to m' },
      { chord: 'Gb' }, { text: 'e' },
    ]
    expect(splitIntoChordWords(tokens)).toEqual([
      plain('Look', 'Db'),
      plain('into'),
      plain('my'),
      plain('eyes,'),
      plain('you', 'Ab'),
      plain('will'),
      plain('see'),
      plain('what'),
      plain('you'),
      plain('mean'),
      plain('to'),
      plain('me', 'Gb'),
    ])
  })

  // TestSplitIntoChordWordsCarriesMarksFromTheTokenThatStartsAWord
  it('carries marks from whichever token started the word', () => {
    const tokens: Array<TranscriptionToken> = [
      { chord: 'C' },
      { text: 'shout', bold: true, italic: true },
      { text: ' plain' },
    ]
    expect(splitIntoChordWords(tokens)).toEqual([
      { chord: 'C', word: 'shout', bold: true, italic: true, underline: false },
      plain('plain'),
    ])
  })

  // TestSplitIntoChordWordsHandlesWordsWithoutAChord
  it('emits chordless words for lyrics with no chord change', () => {
    expect(splitIntoChordWords([{ text: 'just words here' }])).toEqual([
      plain('just'),
      plain('words'),
      plain('here'),
    ])
  })

  it('keeps a trailing chord that has no lyric under it', () => {
    // An instrumental line's last chord must survive the final flush —
    // this is what `havePending` guards in both implementations.
    expect(splitIntoChordWords([{ text: 'end ' }, { chord: 'G7' }])).toEqual([
      plain('end'),
      plain('', 'G7'),
    ])
  })

  it('treats tabs and newlines as word boundaries, like Go', () => {
    expect(splitIntoChordWords([{ text: 'a\tb\nc' }])).toEqual([
      plain('a'),
      plain('b'),
      plain('c'),
    ])
  })

  it('does not split a multi-byte character', () => {
    // Go ranges over runes; JS must iterate code points, or an emoji or
    // accented character indexed by UTF-16 unit would be torn in half.
    expect(splitIntoChordWords([{ text: 'café 🎸' }])).toEqual([
      plain('café'),
      plain('🎸'),
    ])
  })

  it('returns nothing for an empty token stream', () => {
    expect(splitIntoChordWords([])).toEqual([])
  })
})
