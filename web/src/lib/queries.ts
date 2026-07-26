import { queryOptions } from '@tanstack/react-query'

import { fetchSong, fetchSongList, fetchViewer } from '~/server/songs'
import type { SongSearch } from './schemas'

/**
 * One definition per query, shared verbatim by the router loader (server,
 * during SSR) and the component (client, after hydration).
 *
 * This sharing is the whole point: `ensureQueryData` in the loader and
 * `useSuspenseQuery` in the component must agree on the query key down to
 * the last character, or the client re-fetches on mount and undoes the
 * SSR work. Exporting one factory rather than writing the key twice is
 * what makes that structural instead of a convention someone has to
 * remember.
 */

export const songKeys = {
  all: ['songs'] as const,
  list: (search: SongSearch) => [...songKeys.all, 'list', search] as const,
  detail: (slug: string) => [...songKeys.all, 'detail', slug] as const,
}

export const songListQueryOptions = (search: SongSearch) =>
  queryOptions({
    queryKey: songKeys.list(search),
    queryFn: () => fetchSongList({ data: search }),
    /**
     * Shorter than the Worker's own 60s fresh window on purpose. The edge
     * cache is what protects Postgres; this only decides how long a tab
     * that's already open goes without asking. A status edit made in
     * another tab should surface quickly, and the ask is nearly free when
     * the edge is warm.
     */
    staleTime: 30 * 1000,
  })

export const songQueryOptions = (slug: string) =>
  queryOptions({
    queryKey: songKeys.detail(slug),
    queryFn: () => fetchSong({ data: { slug } }),
    /**
     * A song's transcription only changes when Jeff re-digests the doc,
     * which is rare and already triggers a cache purge on the Go side —
     * so a long client stale time costs nothing and makes back-navigation
     * between songs instant.
     */
    staleTime: 5 * 60 * 1000,
  })

export const viewerQueryOptions = () =>
  queryOptions({
    queryKey: ['viewer'] as const,
    queryFn: () => fetchViewer(),
    /**
     * Per-viewer and never edge-cached (see fetchViewer). Kept fresh for
     * the tab's lifetime rather than refetched per route — a session
     * doesn't change mid-visit, and refetching would flicker the admin
     * affordances on every navigation.
     */
    staleTime: Infinity,
  })
