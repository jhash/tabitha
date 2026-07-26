import { z } from 'zod'

/**
 * Wire contract with the Go origin.
 *
 * Every schema here mirrors a Go type one-for-one. Keep the two in sync:
 *   - transcriptionBlockSchema  <- internal/transcription.Block
 *   - transcriptionTokenSchema  <- internal/transcription.Token
 *   - songRowSchema             <- internal/web.SongRow
 *   - songDetailSchema          <- internal/web.OfflineSong, minus the
 *                                  prerendered HTML/PlayHTML strings
 *
 * Field names are lowerCamel because that's the convention the Go side
 * already uses on the wire (see OfflineSong's `contentHash`/`playHtml`
 * json tags) — the SongRow struct has no json tags yet, so the new API
 * handler must add them in this shape.
 */

// ---------------------------------------------------------------------
// Transcription document
// ---------------------------------------------------------------------

/**
 * Matches BlockKind's MarshalJSON, which deliberately emits the readable
 * string form rather than the underlying iota — see internal/transcription/json.go.
 */
export const blockKindSchema = z.enum([
  'section_header',
  'chord_lyric_pair',
  'chord_only_line',
  'text_line',
])
export type BlockKind = z.infer<typeof blockKindSchema>

export const transcriptionTokenSchema = z.object({
  chord: z.string().optional(),
  text: z.string().optional(),
  /**
   * Alignment padding the parser invented because the chord line ran
   * further right than the lyric line. Counts for column math on the Go
   * side; dropped entirely when we regroup into chord-words.
   */
  synthetic: z.boolean().optional(),
  // Editor-set only — never derived from parsing raw text.
  bold: z.boolean().optional(),
  italic: z.boolean().optional(),
  underline: z.boolean().optional(),
})
export type TranscriptionToken = z.infer<typeof transcriptionTokenSchema>

export const transcriptionBlockSchema = z.object({
  kind: blockKindSchema,
  /** Verbatim line content for section_header and text_line. */
  text: z.string().optional(),
  /** Interleaved chord/text stream for the two chord-line kinds. */
  tokens: z.array(transcriptionTokenSchema).optional(),
  /** Trailing non-chord content on a chord line, e.g. "3rd x: Girl reaction". */
  annotation: z.string().optional(),
})
export type TranscriptionBlock = z.infer<typeof transcriptionBlockSchema>

/** The JSONB envelope stored in transcription_versions.content. */
export const transcriptionDocumentSchema = z.object({
  blocks: z.array(transcriptionBlockSchema),
})

// ---------------------------------------------------------------------
// Songs
// ---------------------------------------------------------------------

export const songRowSchema = z.object({
  id: z.number().int(),
  title: z.string(),
  artist: z.string(),
  status: z.string(),
  slug: z.string(),
  genres: z.string(),
  addedByName: z.string(),
  addedByEmail: z.string(),
  /** True when the song has a current transcription version (i.e. digested). */
  hasVersion: z.boolean(),
  createdAt: z.iso.datetime({ offset: true }),
  updatedAt: z.iso.datetime({ offset: true }),
  /**
   * The Google Doc's own createdTime/modifiedTime, absent for songs never
   * digested. The Go side prefers these over tabitha's own timestamps
   * when present (effectiveAddedAt/effectiveUpdatedAt) — that preference
   * is reimplemented client-side in lib/songs.ts so sorting and display
   * agree.
   */
  docCreatedAt: z.iso.datetime({ offset: true }).nullable().default(null),
  docModifiedAt: z.iso.datetime({ offset: true }).nullable().default(null),
})
export type SongRow = z.infer<typeof songRowSchema>

export const songListSchema = z.object({
  songs: z.array(songRowSchema),
  /** Distinct status values, for the filter dropdown. */
  statuses: z.array(z.string()),
  /** Display labels (name, falling back to email) for the added-by filter. */
  addedByUsers: z.array(z.string()),
})
export type SongList = z.infer<typeof songListSchema>

export const songDetailSchema = z.object({
  id: z.number().int(),
  slug: z.string(),
  title: z.string(),
  artist: z.string(),
  status: z.string(),
  /** Song key as stored on the current version; "" when unknown. */
  key: z.string(),
  hasVersion: z.boolean(),
  blocks: z.array(transcriptionBlockSchema),
  /**
   * Same md5 expression the offline manifest uses (queries/songs.sql), so
   * it doubles as an ETag and as the cache key suffix for edge writes.
   */
  contentHash: z.string(),
})
export type SongDetail = z.infer<typeof songDetailSchema>

// ---------------------------------------------------------------------
// Transposition search param (?t=)
// ---------------------------------------------------------------------

/**
 * Shared by the song show and play routes so both read `?t=` identically
 * and can hand it to each other on navigation.
 *
 * `.optional()` rather than `.default(0)` is deliberate and load-bearing:
 * a Zod default makes the router treat the bare URL as non-canonical and
 * 307 it to `?t=0`. The Go version explicitly dropped the param at zero
 * (see withTransposeParam in static/js/transpose.js), and an untransposed
 * song should keep the clean, shareable URL. Consumers read `t ?? 0`.
 *
 * Normalisation happens on the way in, so an out-of-range or hand-edited
 * value (an old `?t=12` link) lands on its equivalent in (-6, 6]; a
 * garbage value falls back to the original key rather than erroring the
 * route.
 */
export const transposeSearchSchema = (wrap: (n: number) => number) =>
  z.object({
    t: z.coerce.number().int().transform(wrap).catch(0).optional(),
  })

// ---------------------------------------------------------------------
// Home page search params
// ---------------------------------------------------------------------

/** Mirrors internal/web.sortColumns — the allowlist gating ORDER BY. */
export const sortColumnSchema = z.enum([
  'title',
  'artist',
  'status',
  'updated',
  'added',
  'added_by',
])
export type SortColumn = z.infer<typeof sortColumnSchema>

export const sortOrderSchema = z.enum(['asc', 'desc'])
export type SortOrder = z.infer<typeof sortOrderSchema>

/**
 * Mirrors internal/web.parseSongQueryParams, including its forgiving
 * behaviour: an unrecognised sort or order is dropped rather than
 * rejected, so a hand-edited URL degrades to the default view instead of
 * erroring. `.catch()` is what buys us that here.
 *
 * Note the `digested` inversion — the Go side defaults to HIDING
 * undigested songs, and only an explicit `digested=all` turns the filter
 * off. Keeping the raw param in the URL (rather than a derived boolean)
 * means existing links keep working unchanged.
 */
export const songSearchSchema = z.object({
  search: z.string().optional().catch(undefined),
  sort: sortColumnSchema.optional().catch(undefined),
  order: sortOrderSchema.optional().catch(undefined),
  status: z.string().optional().catch(undefined),
  added_by: z.string().optional().catch(undefined),
  digested: z.enum(['hide', 'all']).optional().catch(undefined),
})
export type SongSearch = z.infer<typeof songSearchSchema>
