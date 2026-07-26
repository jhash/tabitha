/**
 * Port of static/js/transpose.js — chord transposition by whole semitones.
 *
 * The Go/htmx version rewrote `.chord` spans in the DOM after render,
 * because server-rendered HTML was all it had. Here transposition is a
 * pure function applied during render, so there's no mutation step, no
 * `data-original` bookkeeping to restore from, and no re-initialisation
 * needed on navigation. The music theory below is unchanged.
 */

const NOTE_TO_PC: Record<string, number> = {
  C: 0, 'B#': 0,
  'C#': 1, Db: 1,
  D: 2,
  'D#': 3, Eb: 3,
  E: 4, Fb: 4,
  F: 5, 'E#': 5,
  'F#': 6, Gb: 6,
  G: 7,
  'G#': 8, Ab: 8,
  A: 9,
  'A#': 10, Bb: 10,
  B: 11, Cb: 11,
}

const SHARP_NAMES = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B']
const FLAT_NAMES = ['C', 'Db', 'D', 'Eb', 'E', 'F', 'Gb', 'G', 'Ab', 'A', 'Bb', 'B']

/**
 * Tonics conventionally spelled with flats — the circle-of-fifths flat
 * side: F Bb Eb Ab Db Gb (major), Dm Gm Cm Fm Bbm Ebm (minor). Everything
 * else, including "key unknown", defaults to sharps.
 */
const FLAT_MAJOR_PCS = new Set([1, 3, 5, 6, 8, 10])
const FLAT_MINOR_PCS = new Set([0, 2, 3, 5, 7, 10])

/**
 * Root, optional accidental, optional quality/extension, optional /bass —
 * the same shape as internal/transcription/parser.go's chordTokenRe,
 * minus the x-count/bar/paren alternatives handled separately below.
 */
const CHORD_RE =
  /^([A-Ga-g])(#|b)?((?:maj|min|dim|aug|sus2|sus4|sus|add2|add4|add6|add9|m|∆|Δ)?[0-9]{0,2})(?:\/([A-Ga-g])(#|b)?)?$/i

/** A bare note inside a parenthesised group, e.g. "/F" and "G" in "(/F /F /F#  G)". */
const PAREN_NOTE_RE = /^(\/?)([A-Ga-g])(#|b)?$/i

function mod12(n: number): number {
  return ((n % 12) + 12) % 12
}

function parseNote(letter: string, accidental?: string): number | undefined {
  return NOTE_TO_PC[letter.toUpperCase() + (accidental ?? '')]
}

/** `caseLike` carries the original letter's case so a lowercase chord
 * (Jeff writes some) stays lowercase after transposing. */
function spellNote(pc: number, useFlats: boolean, caseLike: string): string {
  const names = useFlats ? FLAT_NAMES : SHARP_NAMES
  const name = names[mod12(pc)] ?? 'C'
  return caseLike === caseLike.toLowerCase() ? name.toLowerCase() : name
}

function transposeNote(
  letter: string,
  accidental: string | undefined,
  semitones: number,
  useFlats: boolean,
): string {
  const pc = parseNote(letter, accidental)
  if (pc === undefined) return letter + (accidental ?? '')
  return spellNote(pc + semitones, useFlats, letter)
}

function transposeParenGroup(
  token: string,
  semitones: number,
  useFlats: boolean,
): string {
  const words = token
    .slice(1, -1)
    .split(' ')
    .map((w) => {
      const m = PAREN_NOTE_RE.exec(w)
      return m ? (m[1] ?? '') + transposeNote(m[2] ?? '', m[3], semitones, useFlats) : w
    })
  return `(${words.join(' ')})`
}

/**
 * Shifts one chord-row token, leaving repeat markers ("x4"), bar
 * delimiters ("|"), and non-chord parentheticals ("(drums)") untouched.
 */
export function transposeChordToken(
  token: string,
  semitones: number,
  useFlats: boolean,
): string {
  if (token === '' || token === '|' || /^x[0-9]+$/i.test(token)) return token

  if (token.startsWith('(') && token.endsWith(')')) {
    return transposeParenGroup(token, semitones, useFlats)
  }

  const m = CHORD_RE.exec(token)
  if (!m) return token

  const root = transposeNote(m[1] ?? '', m[2], semitones, useFlats)
  const bass = m[4] ? `/${transposeNote(m[4], m[5], semitones, useFlats)}` : ''
  return root + (m[3] ?? '') + bass
}

export interface Key {
  pc: number
  isMinor: boolean
}

/**
 * Reads "Bb" or "F#m" into a tonic. Used only to pick sharp-vs-flat
 * spelling, so an unrecognised key degrades to C major rather than
 * breaking transposition.
 */
export function parseKey(text: string): Key {
  const m = /^([A-Ga-g])(#|b)?(m)?$/i.exec((text || '').trim())
  if (!m) return { pc: 0, isMinor: false }
  const pc = parseNote(m[1] ?? '', m[2])
  return { pc: pc ?? 0, isMinor: Boolean(m[3]) }
}

export function keyUsesFlats(key: Key): boolean {
  const pcs = key.isMinor ? FLAT_MINOR_PCS : FLAT_MAJOR_PCS
  return pcs.has(mod12(key.pc))
}

export function spellKey(key: Key): string {
  const name = (keyUsesFlats(key) ? FLAT_NAMES : SHARP_NAMES)[mod12(key.pc)] ?? 'C'
  return key.isMinor ? `${name}m` : name
}

/**
 * Reduces any semitone count to its shortest-path representative in
 * (-6, 6]. An octave is 12 semitones — the same pitch classes — so
 * nothing is ever more than half an octave from 0, and holding the
 * stepper cycles back through 0 every 12 steps instead of growing without
 * bound. 0 always means "the untransposed original", so an old "?t=12"
 * link normalises to the original too.
 */
export function wrapSemitones(n: number): number {
  const m = mod12(n)
  return m > 6 ? m - 12 : m
}

/**
 * The label shown between the +/- buttons: a real key name when the song's
 * key is known, otherwise a signed semitone offset.
 */
export function transposeLabel(songKey: string, semitones: number): string {
  if (!songKey) return semitones > 0 ? `+${semitones}` : String(semitones)
  if (semitones === 0) return songKey

  const base = parseKey(songKey)
  return spellKey({ pc: base.pc + semitones, isMinor: base.isMinor })
}

/**
 * Transposes a chord for display.
 *
 * At 0 the original string is returned verbatim rather than round-tripped
 * through transposeChordToken — re-spelling always applies the sharp/flat
 * convention, which would silently swap an original "Eb" for "D#" even
 * though nothing was transposed.
 */
export function displayChord(
  chord: string,
  songKey: string,
  semitones: number,
): string {
  if (semitones === 0) return chord

  const base = parseKey(songKey)
  const useFlats = keyUsesFlats({ pc: base.pc + semitones, isMinor: base.isMinor })
  return transposeChordToken(chord, semitones, useFlats)
}
