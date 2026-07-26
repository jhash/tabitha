import type { TranscriptionBlock } from './schemas'

/**
 * Port of internal/web/song_show.go's `omitDuplicateHeaderLines` and its
 * helpers.
 *
 * Jeff's Google Docs open with their own title / "As performed by:" /
 * "Key:" lines, and the page chrome already shows all three from the
 * database — so without this they render twice (the "Downtown" bug).
 *
 * Trimming only happens when the lines actually match the expected
 * pattern; a doc that doesn't follow the convention renders untouched
 * rather than risking eating real content.
 */

/** Collapses whitespace runs and trims — Jeff's docs aren't consistent
 * about spacing after "As performed by:", so exact compares miss real
 * duplicates. */
function normalizeWhitespace(s: string): string {
  return s.trim().split(/\s+/).join(' ')
}

/**
 * Strips a leading "The " or trailing ", the" and lowercases, so the
 * TOC's sort-friendly form and a doc's natural-order byline compare equal
 * ("Rolling Stones, the" vs "The Rolling Stones").
 */
function normalizeArtistName(name: string): string {
  let n = name.trim().toLowerCase()
  if (n.endsWith(', the')) n = n.slice(0, -', the'.length)
  if (n.startsWith('the ')) n = n.slice('the '.length)
  return n.trim()
}

function isTextLine(block: TranscriptionBlock | undefined): boolean {
  return block?.kind === 'text_line'
}

function isDuplicateLine(
  block: TranscriptionBlock | undefined,
  want: string,
): boolean {
  if (!isTextLine(block)) return false
  return (
    normalizeWhitespace(block?.text ?? '').toLowerCase() ===
    normalizeWhitespace(want).toLowerCase()
  )
}

function isDuplicateArtistLine(
  block: TranscriptionBlock | undefined,
  artist: string,
): boolean {
  if (!isTextLine(block)) return false

  const text = normalizeWhitespace(block?.text ?? '')
  const prefix = 'as performed by:'
  if (!text.toLowerCase().startsWith(prefix)) return false

  return normalizeArtistName(text.slice(prefix.length)) === normalizeArtistName(artist)
}

export function omitDuplicateHeaderLines(
  blocks: ReadonlyArray<TranscriptionBlock>,
  song: { title: string; artist: string },
): ReadonlyArray<TranscriptionBlock> {
  let i = 0

  // The title line gates everything: if the doc doesn't open with it,
  // this isn't the convention we know how to trim, so nothing is dropped.
  if (isDuplicateLine(blocks[i], song.title)) i++
  else return blocks

  if (isDuplicateArtistLine(blocks[i], song.artist)) i++

  const keyLine = blocks[i]
  if (
    isTextLine(keyLine) &&
    (keyLine?.text ?? '').trim().toLowerCase().startsWith('key:')
  ) {
    i++
  }

  return blocks.slice(i)
}
