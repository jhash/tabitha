import { useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { useMemo } from 'react'
import { Transcription } from '~/components/Transcription'
import { TransposeControls } from '~/components/TransposeControls'
import { songQueryOptions, viewerQueryOptions } from '~/lib/queries'
import { transposeSearchSchema } from '~/lib/schemas'
import { omitDuplicateHeaderLines } from '~/lib/song-blocks'
import { wrapSemitones } from '~/lib/transpose'

/**
 * Port of the Go route `GET /songs/{idOrSlug}` (internal/web/song_show.go).
 *
 * The Go handler accepted a numeric ID as well as a slug, 301-ing IDs to
 * the canonical slug URL. That redirect belongs at the edge now rather
 * than in a React route — see docs/tanstack-migration.md — so this route
 * is slug-only and stays a single canonical URL per song.
 */
export const Route = createFileRoute('/songs/$slug')({
  /**
   * Validated here rather than read raw off `location.search`, so `t` is
   * a typed `number` everywhere downstream and any `<Link>` targeting
   * this route is checked against the same shape at compile time.
   */
  validateSearch: transposeSearchSchema(wrapSemitones),

  /**
   * `ensureQueryData` (not `fetchQuery`) is the important choice: it's a
   * no-op when the cache already holds fresh data, so a client-side
   * navigation into a song already visited resolves without a round trip,
   * while an SSR request — where the per-request QueryClient is always
   * empty — always fetches.
   *
   * Nothing is returned. The data reaches the component through the
   * dehydrated QueryClient, not through loader data, so it isn't
   * serialised into the page twice.
   */
  loader: async ({ context: { queryClient }, params: { slug } }) => {
    await queryClient.ensureQueryData(songQueryOptions(slug))
  },

  // Runs after the loader, so the song is in cache and the title is
  // available without a second fetch.
  head: ({ loaderData: _, params: { slug } }) => ({
    meta: [{ title: `${slug} · tabitha` }],
  }),

  component: SongPage,
})

function SongPage() {
  const { slug } = Route.useParams()
  // Absent means untransposed — the param is omitted at zero to keep the
  // canonical URL clean (see transposeSearchSchema).
  const semitones = Route.useSearch().t ?? 0

  // Same query options object the loader used, so this reads straight out
  // of the hydrated cache — no fetch on mount, no loading flash, no
  // layout shift.
  const { data: song } = useSuspenseQuery(songQueryOptions(slug))

  // The viewer is a separate, uncached query on purpose (see
  // server/songs.ts): folding it into the song payload would make the
  // song un-shareable across viewers at the edge.
  const { data: viewer } = useSuspenseQuery(viewerQueryOptions())

  const blocks = useMemo(
    () => omitDuplicateHeaderLines(song.blocks, song),
    [song],
  )

  return (
    <main className="container">
      <h1>{song.title}</h1>

      {song.artist ? (
        <p className="byline">As performed by {song.artist}</p>
      ) : null}

      {song.key ? (
        <p className="key">
          Key: <b>{song.key.toUpperCase()}</b>
        </p>
      ) : null}

      {viewer.isSuperadmin ? (
        <p className="admin-affordance">
          {/* The ProseMirror editor is still the Go app's page — it's a
              build-time bundle served from static/, untouched by this
              migration — so this deliberately leaves the SPA. */}
          <a href={`/songs/${song.id}/edit`}>Edit</a>
        </p>
      ) : null}

      {song.hasVersion ? (
        <>
          <p className="play-affordance">
            <Link
              to="/songs/$slug/play"
              params={{ slug }}
              // Carries the current transposition into Play mode, which
              // the Go version achieved by rewriting this href from JS.
              // `|| undefined` keeps the param off an untransposed link.
              search={{ t: semitones || undefined }}
              aria-label="Play mode"
            >
              ▶ Play
            </Link>
          </p>
          <TransposeControls songKey={song.key} semitones={semitones} />
          <Transcription
            blocks={blocks}
            songKey={song.key}
            semitones={semitones}
          />
        </>
      ) : (
        <p className="no-content">
          This song hasn't been digested from Jeff's Google Doc yet.
        </p>
      )}
    </main>
  )
}
