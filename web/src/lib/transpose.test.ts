import { describe, expect, it } from 'vitest'

import {
  displayChord,
  transposeChordToken,
  transposeLabel,
  wrapSemitones,
} from './transpose'

/**
 * Parity suite for the port of static/js/transpose.js.
 */

describe('wrapSemitones', () => {
  it('reduces to the shortest path in (-6, 6]', () => {
    expect(wrapSemitones(0)).toBe(0)
    expect(wrapSemitones(6)).toBe(6)
    // 7 up is 5 down — same pitch classes, shorter to say.
    expect(wrapSemitones(7)).toBe(-5)
    expect(wrapSemitones(-7)).toBe(5)
  })

  it('treats a whole octave as no transposition', () => {
    // An old "?t=12" link must normalise to the original, so the
    // exact-original-spelling path in displayChord still triggers.
    expect(wrapSemitones(12)).toBe(0)
    expect(wrapSemitones(-12)).toBe(0)
    expect(wrapSemitones(13)).toBe(1)
  })
})

describe('transposeChordToken', () => {
  it('shifts a plain triad', () => {
    expect(transposeChordToken('C', 2, false)).toBe('D')
  })

  it('keeps quality and extension text attached to the new root', () => {
    expect(transposeChordToken('Am7', 3, false)).toBe('Cm7')
    expect(transposeChordToken('Gsus4', 2, false)).toBe('Asus4')
  })

  it('transposes a slash bass note too', () => {
    expect(transposeChordToken('C/E', 2, false)).toBe('D/F#')
  })

  it('honours the flat/sharp preference', () => {
    expect(transposeChordToken('C', 1, false)).toBe('C#')
    expect(transposeChordToken('C', 1, true)).toBe('Db')
  })

  it('preserves the original letter case', () => {
    // Jeff writes some chords lowercase; transposing shouldn't shout.
    expect(transposeChordToken('e', 2, false)).toBe('f#')
  })

  it('leaves bars, repeat counts, and empties alone', () => {
    expect(transposeChordToken('|', 2, false)).toBe('|')
    expect(transposeChordToken('x4', 2, false)).toBe('x4')
    expect(transposeChordToken('', 2, false)).toBe('')
  })

  it('transposes notes inside a parenthesised group', () => {
    expect(transposeChordToken('(/F /F#  G)', 2, false)).toBe('(/G /G#  A)')
  })

  it('leaves a non-chord parenthetical annotation untouched', () => {
    expect(transposeChordToken('(drums)', 2, false)).toBe('(drums)')
  })

  it('passes through anything it cannot parse as a chord', () => {
    expect(transposeChordToken('N.C.', 2, false)).toBe('N.C.')
  })
})

describe('displayChord', () => {
  it('returns the stored spelling verbatim at zero', () => {
    // Round-tripping at 0 would re-spell per the sharp/flat convention
    // and silently swap "Eb" for "D#" without transposing anything.
    expect(displayChord('Eb', 'Eb', 0)).toBe('Eb')
    expect(displayChord('D#', 'C', 0)).toBe('D#')
  })

  it('picks flats when the destination key is a flat key', () => {
    // Eb + 2 = F major, on the flat side of the circle of fifths.
    expect(displayChord('Ab', 'Eb', 2)).toBe('Bb')
  })

  it('picks sharps when the destination key is a sharp key', () => {
    // Eb - 1 = D major, a sharp key.
    expect(displayChord('Ab', 'Eb', -1)).toBe('G')
  })

  it('treats an unknown song key as C and spells from there', () => {
    // Matching transpose.js: an unknown key parses to C major, so the
    // DESTINATION key still decides the spelling. C + 1 is Db, a flat
    // key — hence "Db", not "C#". (The "no key known defaults to sharps"
    // note in transpose.js is about tonics outside the flat set, not
    // about the unknown-key case bypassing the lookup.)
    expect(displayChord('C', '', 1)).toBe('Db')
    // C + 2 is D, a sharp key, so this one does come back sharp.
    expect(displayChord('C#', '', 2)).toBe('D#')
  })
})

describe('transposeLabel', () => {
  it('names the destination key when the song key is known', () => {
    expect(transposeLabel('Eb', 0)).toBe('Eb')
    expect(transposeLabel('Eb', 2)).toBe('F')
  })

  it('keeps a minor key minor', () => {
    expect(transposeLabel('Am', 2)).toBe('Bm')
  })

  it('falls back to a signed offset when the key is unknown', () => {
    expect(transposeLabel('', 0)).toBe('0')
    expect(transposeLabel('', 2)).toBe('+2')
    expect(transposeLabel('', -2)).toBe('-2')
  })
})
