import type { SongRow, SongSearch, SortColumn, SortOrder } from './schemas'

/**
 * View-model helpers ported from internal/web/home.go.
 */

/**
 * Both prefer the Google Doc's own timestamps over tabitha's, which only
 * record when tabitha last touched the row — a doc written in 2019 and
 * imported last week should read as 2019.
 */
export function effectiveAddedAt(song: SongRow): string {
  return song.docCreatedAt ?? song.createdAt
}

export function effectiveUpdatedAt(song: SongRow): string {
  return song.docModifiedAt ?? song.updatedAt
}

/**
 * Matches Go's `t.Format("2006-01-02")`. Deliberately not
 * `toLocaleDateString`: this renders on the server during SSR and again
 * on the client at hydration, and a Worker in UTC disagreeing with a
 * browser in another zone is a hydration mismatch. Slicing the ISO string
 * gives the same answer in both places.
 */
export function formatDate(iso: string): string {
  return iso.slice(0, 10)
}

export function addedByLabel(song: SongRow): string {
  return song.addedByName || song.addedByEmail
}

export function songHref(song: SongRow): string {
  return `/songs/${song.slug}`
}

/**
 * What's actually driving the current order, including the implicit
 * defaults — relevance while searching, title otherwise. Sort headers use
 * this to decide which one is active and which arrow to show.
 */
export function effectiveSort(search: SongSearch): SortColumn | 'relevance' {
  if (search.sort) return search.sort
  if (search.search) return 'relevance'
  return 'title'
}

export function effectiveOrder(search: SongSearch): SortOrder {
  return search.order ?? 'asc'
}

/**
 * The search params a sort header should link to: everything else
 * preserved, sort set to this column, order toggled if it's already
 * active. Returns an object rather than a query string — the router
 * serialises it, and typechecks it against the route's schema.
 */
export function withSort(search: SongSearch, column: SortColumn): SongSearch {
  const order: SortOrder =
    effectiveSort(search) === column && effectiveOrder(search) === 'asc'
      ? 'desc'
      : 'asc'
  return { ...search, sort: column, order }
}
