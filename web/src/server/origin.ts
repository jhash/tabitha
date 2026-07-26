import { env } from 'cloudflare:workers'
import { getRequestHeader } from '@tanstack/react-start/server'

/**
 * Talks to the existing Go service, which stays the system of record.
 *
 * Nothing here touches Postgres directly: pgx speaks raw TCP, River's job
 * queue is Go, and the home page's fuzzy search leans on pg_trgm — none
 * of which survive a move into workerd. See docs/tanstack-migration.md.
 */

export class OriginError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'OriginError'
  }
}

/** Thrown for 404s specifically, so routes can map them to notFound(). */
export class OriginNotFound extends OriginError {
  constructor(path: string) {
    super(404, `origin: no such resource ${path}`)
    this.name = 'OriginNotFound'
  }
}

interface OriginOptions {
  /**
   * Forward the caller's `tabitha_session` cookie. Required for anything
   * whose response varies by viewer (superadmin affordances, /admin).
   *
   * Leave this false for cacheable reads — a per-viewer response must
   * never reach a shared cache, and `cachedJSON` has no way to know a
   * payload was personalised.
   */
  authenticated?: boolean
}

export async function fetchOrigin(
  path: string,
  opts: OriginOptions = {},
): Promise<unknown> {
  const url = new URL(path, env.ORIGIN_URL)

  const headers = new Headers({ accept: 'application/json' })

  // Shared secret proving this request came from the Worker rather than
  // straight off the internet, so the Go side can keep the JSON API off
  // the public surface. Set with: wrangler secret put ORIGIN_TOKEN
  const token = env.ORIGIN_TOKEN
  if (token) headers.set('authorization', `Bearer ${token}`)

  if (opts.authenticated) {
    const cookie = getRequestHeader('cookie')
    if (cookie) headers.set('cookie', cookie)
  }

  const response = await fetch(url, {
    headers,
    // The Worker does its own caching in edge-cache.ts; letting Cloudflare
    // also cache this subrequest would stack two independent TTLs and make
    // purges unpredictable.
    cf: { cacheTtl: 0, cacheEverything: false },
  })

  if (response.status === 404) throw new OriginNotFound(path)
  if (!response.ok) {
    throw new OriginError(
      response.status,
      `origin: ${path} returned ${response.status}`,
    )
  }

  return await response.json()
}
