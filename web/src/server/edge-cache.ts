import { env, waitUntil } from 'cloudflare:workers'
import type { z } from 'zod'

/**
 * Two-tier read-through cache for origin JSON, with real
 * stale-while-revalidate semantics.
 *
 *   1. `caches.default` — per-colo, single-digit-ms, free.
 *   2. KV             — global, so a song digested once is warm at every
 *                       edge rather than once per colo. Matters here
 *                       because the catalog's long tail gets read far
 *                       less often than any sane TTL.
 *   3. The Go origin  — Postgres, the system of record.
 *
 * Why freshness is tracked by hand rather than left to `Cache-Control`:
 * the Cache API refuses to return an entry once its own max-age has
 * lapsed, which gives you expiry, not SWR — every reader past the TTL
 * would block on the origin. So entries are stored with a long max-age
 * and a `x-fresh-until` stamp we compare ourselves. Past `freshTtl` we
 * serve the stale copy immediately and refresh in the background via
 * `waitUntil`; only past `staleTtl` does a request actually wait.
 */

export interface CachePolicy {
  /** Seconds an entry is served without any revalidation. */
  freshTtl: number
  /**
   * Seconds past `freshTtl` an entry may still be served, while a
   * background refresh runs. Beyond this, reads block on the origin.
   */
  staleTtl: number
}

/** Song pages change only when Jeff re-digests a doc — minutes are fine. */
export const SONG_CACHE_POLICY: CachePolicy = { freshTtl: 300, staleTtl: 3600 }

/** The catalog listing shifts more often (status edits, new songs). */
export const CATALOG_CACHE_POLICY: CachePolicy = { freshTtl: 60, staleTtl: 600 }

const FRESH_UNTIL_HEADER = 'x-tabitha-fresh-until'

/**
 * Cache keys must be absolute URLs for the Cache API. This host is never
 * resolved — it only namespaces entries — but it must stay stable, since
 * changing it silently invalidates every colo cache at once.
 */
const CACHE_KEY_ORIGIN = 'https://cache.tabitha.internal'

function cacheKeyFor(key: string): Request {
  return new Request(`${CACHE_KEY_ORIGIN}/${key}`, { method: 'GET' })
}

interface CachedEnvelope<T> {
  value: T
  freshUntil: number
}

function nowSeconds(): number {
  return Math.floor(Date.now() / 1000)
}

/**
 * Fetch JSON from the origin, validate it, and serve it through the edge
 * cache. `schema` is not optional on purpose — a cache is exactly where a
 * silently-changed origin payload would otherwise persist for an hour.
 */
export async function cachedJSON<TSchema extends z.ZodType>(
  key: string,
  schema: TSchema,
  policy: CachePolicy,
  fetchFromOrigin: () => Promise<unknown>,
): Promise<z.infer<TSchema>> {
  const cacheKey = cacheKeyFor(key)
  const cache = caches.default

  const hit = await readColoCache<z.infer<TSchema>>(cacheKey, cache, schema)
  if (hit) {
    if (hit.freshUntil > nowSeconds()) return hit.value
    // Stale but usable: refresh behind the response, not in front of it.
    waitUntil(revalidate(key, cacheKey, cache, schema, policy, fetchFromOrigin))
    return hit.value
  }

  const fromKv = await readKvCache<z.infer<TSchema>>(key, schema)
  if (fromKv) {
    // Populate this colo so the next local read skips KV entirely.
    waitUntil(writeColoCache(cacheKey, cache, fromKv, policy))
    if (fromKv.freshUntil <= nowSeconds()) {
      waitUntil(revalidate(key, cacheKey, cache, schema, policy, fetchFromOrigin))
    }
    return fromKv.value
  }

  // Cold on both tiers — this is the only path that pays for the origin.
  return await revalidate(key, cacheKey, cache, schema, policy, fetchFromOrigin)
}

async function revalidate<TSchema extends z.ZodType>(
  key: string,
  cacheKey: Request,
  cache: Cache,
  schema: TSchema,
  policy: CachePolicy,
  fetchFromOrigin: () => Promise<unknown>,
): Promise<z.infer<TSchema>> {
  const value = schema.parse(await fetchFromOrigin())
  const envelope: CachedEnvelope<z.infer<TSchema>> = {
    value,
    freshUntil: nowSeconds() + policy.freshTtl,
  }

  await Promise.all([
    writeColoCache(cacheKey, cache, envelope, policy),
    writeKvCache(key, envelope, policy),
  ])

  return value
}

async function readColoCache<T>(
  cacheKey: Request,
  cache: Cache,
  schema: z.ZodType,
): Promise<CachedEnvelope<T> | null> {
  const cached = await cache.match(cacheKey)
  if (!cached) return null

  try {
    const value = schema.parse(await cached.json())
    const freshUntil = Number(cached.headers.get(FRESH_UNTIL_HEADER) ?? 0)
    return { value: value as T, freshUntil }
  } catch {
    // A payload that no longer parses is worse than a miss — drop it and
    // let the caller fall through to KV/origin.
    waitUntil(cache.delete(cacheKey))
    return null
  }
}

async function writeColoCache<T>(
  cacheKey: Request,
  cache: Cache,
  envelope: CachedEnvelope<T>,
  policy: CachePolicy,
): Promise<void> {
  const response = new Response(JSON.stringify(envelope.value), {
    headers: {
      'content-type': 'application/json; charset=utf-8',
      // Deliberately the FULL window: expiry is our job (via
      // x-tabitha-fresh-until), not the Cache API's — see the module doc.
      'cache-control': `public, max-age=${policy.freshTtl + policy.staleTtl}`,
      [FRESH_UNTIL_HEADER]: String(envelope.freshUntil),
    },
  })
  await cache.put(cacheKey, response)
}

async function readKvCache<T>(
  key: string,
  schema: z.ZodType,
): Promise<CachedEnvelope<T> | null> {
  // Two type args, not one: the first is the stored value's type, the
  // second the metadata's. Passing only one silently types the METADATA
  // as the value shape.
  const { value, metadata } = await env.SONG_CACHE.getWithMetadata<
    unknown,
    { freshUntil: number }
  >(key, { type: 'json' })
  if (value === null) return null

  try {
    return { value: schema.parse(value) as T, freshUntil: metadata?.freshUntil ?? 0 }
  } catch {
    waitUntil(env.SONG_CACHE.delete(key))
    return null
  }
}

async function writeKvCache<T>(
  key: string,
  envelope: CachedEnvelope<T>,
  policy: CachePolicy,
): Promise<void> {
  await env.SONG_CACHE.put(key, JSON.stringify(envelope.value), {
    // KV's own floor is 60s; anything shorter is silently rejected.
    expirationTtl: Math.max(60, policy.freshTtl + policy.staleTtl),
    metadata: { freshUntil: envelope.freshUntil },
  })
}

/**
 * Drop a key from both tiers. Call this from the Go side's existing
 * cache-purge path (internal/cloudflare) so a re-digest or status edit
 * shows up immediately instead of riding out the stale window.
 *
 * Colo caches are per-datacenter, so this only clears the colo handling
 * the purge request — clearing KV is what actually matters, since a
 * missing KV entry forces the next revalidation everywhere.
 */
export async function purgeCached(key: string): Promise<void> {
  await Promise.all([
    caches.default.delete(cacheKeyFor(key)),
    env.SONG_CACHE.delete(key),
  ])
}
