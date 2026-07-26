import { createServerFn } from '@tanstack/react-start'
import { notFound } from '@tanstack/react-router'
import { z } from 'zod'

import { songDetailSchema, songListSchema, songSearchSchema } from '~/lib/schemas'
import {
  CATALOG_CACHE_POLICY,
  SONG_CACHE_POLICY,
  cachedJSON,
} from './edge-cache'
import { OriginNotFound, fetchOrigin } from './origin'

/**
 * Server functions backing the song routes. Each one runs in the Worker,
 * reads through the edge cache, and validates the origin's payload before
 * it reaches a loader.
 */

/**
 * Canonicalises search params into a cache key. Sorted, and with empty
 * values dropped, so `?status=&search=hey` and `?search=hey` share one
 * entry instead of fragmenting the cache across equivalent URLs.
 */
function catalogCacheKey(search: z.infer<typeof songSearchSchema>): string {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(search)) {
    if (value !== undefined && value !== '') params.set(key, String(value))
  }
  params.sort()
  return `catalog:${params.toString()}`
}

export const fetchSongList = createServerFn({ method: 'GET' })
  .validator(songSearchSchema)
  .handler(async ({ data: search }) => {
    const query = new URLSearchParams()
    for (const [key, value] of Object.entries(search)) {
      if (value !== undefined && value !== '') query.set(key, String(value))
    }

    return await cachedJSON(
      catalogCacheKey(search),
      songListSchema,
      CATALOG_CACHE_POLICY,
      () => fetchOrigin(`/api/songs?${query.toString()}`),
    )
  })

export const fetchSong = createServerFn({ method: 'GET' })
  .validator(z.object({ slug: z.string().min(1) }))
  .handler(async ({ data: { slug } }) => {
    try {
      return await cachedJSON(
        `song:${slug}`,
        songDetailSchema,
        SONG_CACHE_POLICY,
        () => fetchOrigin(`/api/songs/${encodeURIComponent(slug)}`),
      )
    } catch (error) {
      // Let the router render its 404 boundary rather than the error
      // boundary — a missing slug is an expected outcome here, not a fault.
      if (error instanceof OriginNotFound) throw notFound()
      throw error
    }
  })

/**
 * The current viewer, resolved from the `tabitha_session` cookie the Go
 * side already issues. Deliberately uncached and never batched into the
 * cacheable reads above: this is the one response that varies per person,
 * and mixing it into a shared cache entry would leak admin affordances
 * (or worse) across viewers.
 */
export const fetchViewer = createServerFn({ method: 'GET' }).handler(async () => {
  const viewerSchema = z.object({
    email: z.string(),
    name: z.string(),
    isSuperadmin: z.boolean(),
  })

  try {
    return viewerSchema.parse(await fetchOrigin('/api/me', { authenticated: true }))
  } catch {
    // Signed out, expired session, or origin hiccup — all three mean the
    // same thing for rendering: show the public view.
    return { email: '', name: '', isSuperadmin: false }
  }
})
