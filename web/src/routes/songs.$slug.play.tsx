import { useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'
import { useMemo } from 'react'
import { Transcription } from '~/components/Transcription'
import { songQueryOptions } from '~/lib/queries'
import { transposeSearchSchema } from '~/lib/schemas'
import { omitDuplicateHeaderLines } from '~/lib/song-blocks'
import { wrapSemitones } from '~/lib/transpose'
import playCss from '~/styles/play.css?url'

/**
 * Port of the Go route `GET /songs/{idOrSlug}/play` (internal/web/song_play.go)
 * — a fullscreen, full-bleed reader with no site chrome.
 *
 * The Go version got its bare shell from a separate `PagePlay` layout
 * that skipped the header and content column. Here the shell is shared
 * (there's one `__root.tsx`), so Play mode instead hides the chrome via
 * `.play-mode` in play.css — loaded only on this route, through the
 * route's own `head`.
 *
 * Note it reuses `songQueryOptions`: navigating Show -> Play hits the
 * same query key, so the transcription is already in cache and Play mode
 * opens with no fetch at all.
 */
export const Route = createFileRoute('/songs/$slug/play')({
  validateSearch: transposeSearchSchema(wrapSemitones),

  loader: async ({ context: { queryClient }, params: { slug } }) => {
    await queryClient.ensureQueryData(songQueryOptions(slug))
  },

  head: () => ({
    links: [{ rel: 'stylesheet', href: playCss }],
  }),

  component: PlayPage,
})

function PlayPage() {
  const { slug } = Route.useParams()
  const semitones = Route.useSearch().t ?? 0
  const { data: song } = useSuspenseQuery(songQueryOptions(slug))

  const blocks = useMemo(
    () => omitDuplicateHeaderLines(song.blocks, song),
    [song],
  )

  return (
    <div className="play-root play-mode">
      <Link
        to="/songs/$slug"
        params={{ slug }}
        // Hands the current transposition back to the show page, so
        // closing Play mode doesn't silently snap to the original key.
        search={{ t: semitones || undefined }}
        className="play-close"
        aria-label="Close play mode"
      >
        ✕
      </Link>
      <h1 className="play-title">{song.title}</h1>
      <Transcription blocks={blocks} songKey={song.key} semitones={semitones} />
    </div>
  )
}
