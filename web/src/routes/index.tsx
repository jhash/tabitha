import { useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'

import { songListQueryOptions, viewerQueryOptions } from '~/lib/queries'
import type { SongRow, SongSearch, SortColumn } from '~/lib/schemas'
import { songSearchSchema, sortColumnSchema } from '~/lib/schemas'
import {
  addedByLabel,
  effectiveOrder,
  effectiveSort,
  effectiveUpdatedAt,
  effectiveAddedAt,
  formatDate,
  withSort,
} from '~/lib/songs'

/**
 * Port of the Go route `GET /` (internal/web/home.go): the public table of
 * contents — fuzzy-searchable, filterable by status and added-by, and
 * sortable by any column.
 *
 * The Go version drove this with htmx: the filter form posted itself with
 * `hx-get`/`hx-push-url`, and every sort header carried its own
 * hand-built query string. Here the URL is the state, validated once by
 * `songSearchSchema`, and both the loader and the component read it — so
 * a filtered view is server-rendered on first hit rather than assembled
 * by a follow-up request.
 */
export const Route = createFileRoute('/')({
  validateSearch: songSearchSchema,

  /**
   * Without this the loader would re-run on every keystroke's URL update.
   * Listing exactly the params the query depends on means an unrelated
   * change (a future `?highlight=`) doesn't refetch the catalog.
   */
  loaderDeps: ({ search }) => search,

  loader: async ({ context: { queryClient }, deps }) => {
    // Both are warmed here so the page renders in one pass. They're
    // separate queries — not one payload — because only the catalog is
    // edge-cacheable; the viewer varies per person (see server/songs.ts).
    await Promise.all([
      queryClient.ensureQueryData(songListQueryOptions(deps)),
      queryClient.ensureQueryData(viewerQueryOptions()),
    ])
  },

  head: () => ({
    meta: [
      { title: 'Songs · tabitha' },
      { name: 'description', content: "Jeff's music transcription catalog" },
      { property: 'og:title', content: 'Songs' },
    ],
  }),

  component: HomePage,
})

function HomePage() {
  const search = Route.useSearch()
  const { data } = useSuspenseQuery(songListQueryOptions(search))
  const { data: viewer } = useSuspenseQuery(viewerQueryOptions())

  return (
    <main className="container container-wide">
      <div className="songs-header-row">
        <h1>Songs</h1>
        {viewer.isSuperadmin ? <NewSongButton /> : null}
      </div>

      <SearchAndFilterForm
        search={search}
        statuses={data.statuses}
        addedByUsers={data.addedByUsers}
      />

      <SongsTable
        songs={data.songs}
        search={search}
        viewerIsSuperadmin={viewer.isSuperadmin}
      />
    </main>
  )
}

function NewSongButton() {
  // Song creation is still a Go-rendered form; not yet migrated.
  return (
    <a className="new-song-button" href="/songs/new">
      + Song
    </a>
  )
}

function SearchAndFilterForm({
  search,
  statuses,
  addedByUsers,
}: {
  search: SongSearch
  statuses: ReadonlyArray<string>
  addedByUsers: ReadonlyArray<string>
}) {
  const navigate = useNavigate({ from: '/' })

  /**
   * One updater for every control. `replace: true` keeps a session of
   * filter-fiddling from burying the previous page under a dozen history
   * entries — the same reasoning behind the Go version's `hx-push-url`
   * being applied per-navigation rather than per-keystroke.
   */
  const update = (patch: Partial<SongSearch>) => {
    void navigate({
      search: (prev) => {
        const next = { ...prev, ...patch }
        // Empty means "no filter", and an empty param in the URL is noise
        // that also fragments the edge cache key.
        for (const key of Object.keys(next) as Array<keyof SongSearch>) {
          if (next[key] === '' || next[key] === undefined) delete next[key]
        }
        return next
      },
      replace: true,
    })
  }

  return (
    <form
      // The router owns navigation; without this the browser would do a
      // full page load on Enter.
      onSubmit={(e) => e.preventDefault()}
    >
      <input
        type="search"
        name="search"
        value={search.search ?? ''}
        placeholder="Search title, artist, genre…"
        onChange={(e) => update({ search: e.target.value })}
      />

      <select
        name="status"
        value={search.status ?? ''}
        onChange={(e) => update({ status: e.target.value })}
      >
        <option value="">Any status</option>
        {statuses.map((s) => (
          <option key={s} value={s}>
            {s}
          </option>
        ))}
      </select>

      <select
        name="added_by"
        value={search.added_by ?? ''}
        onChange={(e) => update({ added_by: e.target.value })}
      >
        <option value="">Anyone</option>
        {addedByUsers.map((u) => (
          <option key={u} value={u}>
            {u}
          </option>
        ))}
      </select>

      {/* Mirrors the Go default: undigested songs are hidden unless
          explicitly asked for. */}
      <select
        name="digested"
        value={search.digested ?? 'hide'}
        onChange={(e) =>
          update({ digested: e.target.value === 'all' ? 'all' : undefined })
        }
      >
        <option value="hide">Hide undigested songs</option>
        <option value="all">Show all songs</option>
      </select>
    </form>
  )
}

function SongsTable({
  songs,
  search,
  viewerIsSuperadmin,
}: {
  songs: ReadonlyArray<SongRow>
  search: SongSearch
  viewerIsSuperadmin: boolean
}) {
  const columns: Array<[label: string, column: SortColumn]> = [
    ['Title', 'title'],
    ['Artist', 'artist'],
    ['Status', 'status'],
    ['Last Updated', 'updated'],
    ['Added', 'added'],
    ['Added By', 'added_by'],
  ]

  return (
    <table>
      <thead>
        <tr>
          {viewerIsSuperadmin ? <th /> : null}
          {columns.map(([label, column]) => (
            <SortHeader
              key={column}
              label={label}
              column={column}
              search={search}
            />
          ))}
        </tr>
      </thead>
      <tbody>
        {songs.map((song) => (
          <tr key={song.id} className={song.hasVersion ? undefined : 'no-version'}>
            {viewerIsSuperadmin ? (
              <td>
                <input type="checkbox" name="ids" value={song.id} />
              </td>
            ) : null}
            <td>
              <Link to="/songs/$slug" params={{ slug: song.slug }}>
                {song.title}
              </Link>
            </td>
            <td>{song.artist}</td>
            <td>{song.status}</td>
            <td>{formatDate(effectiveUpdatedAt(song))}</td>
            <td>{formatDate(effectiveAddedAt(song))}</td>
            <td>{addedByLabel(song)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function SortHeader({
  label,
  column,
  search,
}: {
  label: string
  column: SortColumn
  search: SongSearch
}) {
  const active = effectiveSort(search) === column
  const arrow = active ? (effectiveOrder(search) === 'desc' ? ' ▾' : ' ▴') : ''

  return (
    <th>
      {/* A real <Link>, so the header is still a middle-clickable,
          copyable URL exactly as the Go version's <a href> was. */}
      <Link to="/" search={withSort(search, column)} replace>
        {label}
        {arrow}
      </Link>
    </th>
  )
}

// Re-exported so the sort-column allowlist has one definition shared with
// the schema, rather than a second list drifting out of sync (the bug the
// Go side avoided by deriving isValidSort from sortColumns).
export { sortColumnSchema }
