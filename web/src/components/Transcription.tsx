import type { ReactNode } from 'react'

import { splitIntoChordWords } from '~/lib/chord-words'
import type { TranscriptionBlock, TranscriptionToken } from '~/lib/schemas'
import { displayChord } from '~/lib/transpose'

/**
 * Port of internal/web/transcription_render.go's `renderTranscriptionHTML`.
 *
 * The emitted class names (`transcription`, `chord-line`, `chord-word`,
 * `chord`, `lyric`, `section-header`, `text-line`, `annotation`) match the
 * Go output exactly, so static/css/style.css carries over unchanged.
 *
 * Transposition, however, does NOT carry over as-is: the Go page shipped
 * original chords and let static/js/transpose.js rewrite the spans after
 * load. Here `semitones` is applied during render, so the server sends
 * already-transposed markup for a `?t=` URL and there's no post-hydration
 * text swap to flash.
 */

export interface TranscriptionProps {
  blocks: ReadonlyArray<TranscriptionBlock>
  /** The song's stored key; drives sharp-vs-flat spelling only. */
  songKey: string
  /** Whole semitones to shift by. 0 renders chords exactly as stored. */
  semitones: number
}

export function Transcription({ blocks, songKey, semitones }: TranscriptionProps) {
  return (
    <div className="transcription">
      {blocks.map((block, i) => (
        // Blocks have no stable identity — they're positional lines in a
        // document, and the whole document is replaced wholesale on any
        // edit — so the index is the honest key here.
        <Block key={i} block={block} songKey={songKey} semitones={semitones} />
      ))}
    </div>
  )
}

function Block({
  block,
  songKey,
  semitones,
}: {
  block: TranscriptionBlock
  songKey: string
  semitones: number
}) {
  switch (block.kind) {
    case 'section_header':
      return <div className="section-header">{block.text}</div>
    case 'text_line':
      return <div className="text-line">{textLineContent(block)}</div>
    case 'chord_only_line':
    case 'chord_lyric_pair':
      return <ChordLine block={block} songKey={songKey} semitones={semitones} />
  }
}

/**
 * Renders a text line's marks when the editor has set tokens, falling
 * back to the plain string for unformatted lines and for versions stored
 * before marks existed.
 */
function textLineContent(block: TranscriptionBlock): ReactNode {
  if (!block.tokens || block.tokens.length === 0) return block.text

  return block.tokens.map((token, i) => (
    <Marked key={i} marks={token}>
      {token.text}
    </Marked>
  ))
}

function ChordLine({
  block,
  songKey,
  semitones,
}: {
  block: TranscriptionBlock
  songKey: string
  semitones: number
}) {
  const words = splitIntoChordWords(block.tokens ?? [])

  return (
    <div className="chord-line">
      {words.map((word, i) => (
        <span className="chord-word" key={i}>
          {/* Rendered even when empty: the empty span is what reserves the
              chord row's height, keeping lyrics on one baseline instead of
              letting chordless words ride up. */}
          <span className="chord">
            {displayChord(word.chord, songKey, semitones)}
          </span>
          <span className="lyric">
            <Marked marks={word}>{word.word}</Marked>
          </span>
        </span>
      ))}
      {block.annotation ? (
        <span className="annotation">{block.annotation}</span>
      ) : null}
    </div>
  )
}

/**
 * Wraps content in strong/em/u per its marks. Nesting order matches the
 * Go version (underline innermost, bold outermost) so the generated DOM
 * is byte-comparable against the Go renderer's during the migration.
 */
function Marked({
  marks,
  children,
}: {
  marks: Pick<TranscriptionToken, 'bold' | 'italic' | 'underline'>
  children: ReactNode
}) {
  let node: ReactNode = children
  if (marks.underline) node = <u>{node}</u>
  if (marks.italic) node = <em>{node}</em>
  if (marks.bold) node = <strong>{node}</strong>
  return <>{node}</>
}
