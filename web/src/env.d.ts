/**
 * Bindings that `wrangler types` can't see, kept out of
 * worker-configuration.d.ts because that file is regenerated (and any
 * hand-edit lost) on every `npm run cf-typegen`.
 */
/**
 * The DOM lib's `CacheStorage` has no `default` — that's a Workers
 * extension. Because the client half of this app genuinely needs `lib.dom`,
 * the two definitions coexist and DOM's wins on conflict; merging the
 * missing member back in is what lets `caches.default` typecheck in
 * Worker-only modules without dropping DOM types app-wide.
 */
interface CacheStorage {
  readonly default: Cache
}

declare namespace Cloudflare {
  interface Env {
    /**
     * Shared secret authenticating the Worker to the Go origin. Set via
     * `wrangler secret put ORIGIN_TOKEN`, so it's absent in local dev
     * unless .dev.vars provides it — hence optional.
     */
    ORIGIN_TOKEN?: string
  }
}
