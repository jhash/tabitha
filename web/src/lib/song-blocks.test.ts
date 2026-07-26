import { describe, expect, it } from 'vitest'

import type { TranscriptionBlock } from './schemas'
import { omitDuplicateHeaderLines } from './song-blocks'

/**
 * Parity suite for the port of internal/web/song_show.go's
 * `omitDuplicateHeaderLines`. Cases mirror the Go tests named
 * TestSongShowOmitsDuplicate* in internal/web/song_show_*_test.go.
 */

const text = (s: string): TranscriptionBlock => ({ kind: 'text_line', text: s })
const header = (s: string): TranscriptionBlock => ({
  kind: 'section_header',
  text: s,
})

const song = { title: 'Downtown', artist: 'Petula Clark' }

describe('omitDuplicateHeaderLines', () => {
  // TestSongShowOmitsDuplicateTitleAndBylineFromTranscription
  it('drops a repeated title, byline, and key line', () => {
    const blocks = [
      text('Downtown'),
      text('As performed by: Petula Clark'),
      text('Key: Eb'),
      header('VERSE 1:'),
    ]
    expect(omitDuplicateHeaderLines(blocks, song)).toEqual([header('VERSE 1:')])
  })

  // TestSongShowOmitsDuplicateBylineWithExtraInternalWhitespace
  it('tolerates extra whitespace after "As performed by:"', () => {
    // Jeff's docs aren't consistent about spacing, so an exact-match
    // compare would miss real duplicates.
    const blocks = [
      text('  Downtown  '),
      text('As performed by:      Petula   Clark'),
      header('VERSE 1:'),
    ]
    expect(omitDuplicateHeaderLines(blocks, song)).toEqual([header('VERSE 1:')])
  })

  // TestSongShowOmitsDuplicateBylineWhenDocUsesDifferentArtistNameFormat
  it('matches a sort-order artist against a natural-order one', () => {
    const stones = { title: 'Satisfaction', artist: 'Rolling Stones, the' }
    const blocks = [
      text('Satisfaction'),
      text('As performed by: The Rolling Stones'),
      header('INTRO:'),
    ]
    expect(omitDuplicateHeaderLines(blocks, stones)).toEqual([header('INTRO:')])
  })

  it('leaves the document untouched when it does not open with the title', () => {
    // The title line gates everything — a doc that doesn't follow the
    // convention must render verbatim rather than risk eating real
    // content.
    const blocks = [header('VERSE 1:'), text('Downtown')]
    expect(omitDuplicateHeaderLines(blocks, song)).toEqual(blocks)
  })

  it('stops after the title when the byline names a different artist', () => {
    // Only the title is a confirmed duplicate here, so the byline (and
    // therefore the key check, which never advances past it) must stay.
    const blocks = [
      text('Downtown'),
      text('As performed by: Someone Else'),
      text('Key: Eb'),
    ]
    expect(omitDuplicateHeaderLines(blocks, song)).toEqual([
      text('As performed by: Someone Else'),
      text('Key: Eb'),
    ])
  })

  it('drops the title alone when no byline follows', () => {
    const blocks = [text('Downtown'), header('VERSE 1:')]
    expect(omitDuplicateHeaderLines(blocks, song)).toEqual([header('VERSE 1:')])
  })

  it('does not treat a section header matching the title as a duplicate', () => {
    // Only text lines are eligible — a section header is real content.
    const blocks = [header('Downtown'), text('Key: Eb')]
    expect(omitDuplicateHeaderLines(blocks, song)).toEqual(blocks)
  })

  it('handles an empty document', () => {
    expect(omitDuplicateHeaderLines([], song)).toEqual([])
  })
})
