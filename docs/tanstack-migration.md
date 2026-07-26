# TanStack Start on Cloudflare Workers — migration

Status: **step 1 of the migration.** The `web/` app builds, typechecks,
passes its suites, and server-renders the home page, song page, and Play
mode against a stub origin. It is not wired to the real Go service yet —
the JSON API it expects doesn't exist on the Go side (see
[What Go still owes](#what-go-still-owes)).

## The one decision that diverges from the brief

The brief asked for the data layer to move onto Workers, with D1 and KV as
bindings. **KV is in use; D1 is not, and Postgres stays where it is.**

Three things in this app can't cross into `workerd`:

- **pgx speaks raw TCP.** Workers have no TCP sockets outside Hyperdrive.
- **The home page's fuzzy search is `pg_trgm`.** `buildSongsQuery`
  (`internal/web/song_query.go:57`) ranks by `similarity()` across title,
  artist, and genres. D1 is SQLite — there is no `pg_trgm`, and the
  closest substitute (FTS5) ranks differently enough to change what the
  search returns.
- **River is a Postgres-backed Go job queue.** `toc_sync` and
  `digest_song` (`internal/jobs/`) poll Postgres and call Google APIs on a
  schedule. Workers have no long-running process to host that.

Sessions are a smaller version of the same problem: `tabitha_session`
tokens are rows in Postgres (`internal/auth/session.go:36`), and OAuth
tokens are AES-encrypted at rest.

So the architecture here is **Go as the data origin, the Worker as the
frontend**:

```
browser ──▶ Cloudflare Worker (TanStack Start SSR)
                 │  caches.default  (per-colo)
                 │  KV SONG_CACHE   (global)
                 └──▶ Go service ──▶ Postgres
                        + River jobs, Google OAuth, the ProseMirror editor
```

The Worker reaches the origin with `fetch` — a Web-standard API, so the
"Node-agnostic" requirement holds without qualification. Nothing in
`web/src/` imports `node:*`; the `ssr` environment is compiled against
`workerd` (`vite.config.ts`), so a stray Node import fails the build
rather than the deploy.

**If you do want Go gone entirely**, the path is Hyperdrive, not D1: it
gives Workers pooled Postgres over TCP, keeping `pg_trgm` and the existing
schema. The job queue still needs somewhere to live — Cron Triggers plus a
Durable Object, or leaving a small Go worker running. That's a separate
project; this migration doesn't assume it.

## What's here

```
web/
  vite.config.ts          cloudflare() + tanstackStart() + viteReact()
  wrangler.jsonc          bindings, vars, compat flags
  vitest.config.ts        node-only, deliberately not the app config
  src/
    router.tsx            per-request QueryClient + SSR/query integration
    routes/
      __root.tsx          document shell, head tags, boundaries
      index.tsx           GET /            <- internal/web/home.go
      songs.$slug.tsx     GET /songs/{...} <- internal/web/song_show.go
      songs.$slug.play.tsx GET /songs/{...}/play <- internal/web/song_play.go
    server/
      origin.ts           authenticated fetch to Go
      edge-cache.ts       two-tier read-through SWR cache
      songs.ts            server functions (createServerFn)
    lib/
      schemas.ts          Zod mirrors of the Go structs
      queries.ts          queryOptions shared by loader + component
      chord-words.ts      port of splitIntoChordWords
      transpose.ts        port of static/js/transpose.js
      song-blocks.ts      port of omitDuplicateHeaderLines
```

### The SSR + Query pattern

Every data route follows the same shape, and the sharing is structural
rather than conventional:

```ts
// lib/queries.ts — ONE definition
export const songQueryOptions = (slug: string) =>
  queryOptions({ queryKey: ['songs', 'detail', slug], queryFn: ..., staleTime: 5 * 60_000 })
```

```tsx
// routes/songs.$slug.tsx
loader: ({ context: { queryClient }, params }) =>
  queryClient.ensureQueryData(songQueryOptions(params.slug)),   // server
...
const { data } = useSuspenseQuery(songQueryOptions(slug))       // client
```

The loader returns nothing. Data reaches the component through the
dehydrated QueryClient (wired once in `router.tsx` via
`setupRouterSsrQueryIntegration`), not through loader data — so it's
serialised into the page once, not twice.

`ensureQueryData` rather than `fetchQuery` is deliberate: it no-ops when
the cache is already fresh, so a client-side navigation back to a visited
song costs no round trip, while SSR — where the per-request QueryClient is
always empty — always fetches.

**The QueryClient is built inside `getRouter()`, never at module scope.** A
module-scope client would live for the whole isolate, which outlives a
request and is shared across concurrent ones — one viewer's data served to
the next.

### Edge caching

`server/edge-cache.ts` is a read-through cache with real
stale-while-revalidate. The Cache API alone can't do SWR — it refuses to
return an entry past its own `max-age`, which is expiry, not SWR. So
entries are stored with a long `max-age` plus an `x-tabitha-fresh-until`
stamp compared by hand. Past `freshTtl` the stale copy is served
immediately and refreshed via `waitUntil`; only past `staleTtl` does a
request block.

| | fresh | stale-while-revalidate |
|---|---|---|
| song detail | 300s | 3600s |
| catalog listing | 60s | 600s |

Two tiers, because they fail differently: `caches.default` is per-colo and
fast; KV is global, so a song digested once is warm at every edge rather
than once per datacenter — which matters for a catalog whose long tail is
read far less often than any sane TTL.

Origin payloads are Zod-validated **before** being cached. A cache is
exactly where a silently-changed payload would otherwise persist for an
hour.

`fetchViewer` is deliberately uncached and never folded into the cacheable
reads — it's the one response that varies per person, and mixing it into a
shared entry would leak admin affordances across viewers.

### What the ports preserve

`web/src/lib/*.test.ts` mirrors the Go tests case for case (36 tests).
Both renderers stay live during the migration, so this is the contract
between them. The subtle ones:

- A chord landing mid-word is stored as two adjacent text tokens with no
  whitespace (`"yo"` + `"u"`). Walking code points across token
  boundaries is what keeps `"you"` one wrappable unit — and the chord
  binds to the reunited word.
- At zero semitones the stored chord spelling is returned verbatim.
  Round-tripping would re-spell per the sharp/flat convention and turn
  `Eb` into `D#` without transposing anything.
- `omitDuplicateHeaderLines` only trims when the title line matches
  first; a doc that doesn't follow Jeff's convention renders untouched
  rather than risking eating real content.

### What changed on purpose

| Go / htmx | Here | Why |
|---|---|---|
| `?v=` content-hash asset URLs (`assets.go`) | Vite filename fingerprinting | Same guarantee, no runtime hashing |
| `hx-boost` on `<html>` | Router navigation | Native |
| `transpose.js` rewrites `.chord` spans post-load | `semitones` applied during render | SSR emits transposed markup for `?t=`; no post-hydration text swap |
| `updateTransposeLinks` patches hrefs | `<Link search={{ t }}>` | Router carries params |
| Hand-built sort query strings | `validateSearch` + typed `<Link>` | Params typechecked at compile time |
| Separate chrome-less `PagePlay` layout | Shared shell + `.play-mode` | One document shell |

`?t=` uses `.optional()`, not `.default(0)` — a Zod default makes the
router 307 a bare URL to `?t=0`, and an untransposed song should keep the
clean, shareable URL the Go version had.

## What Go still owes

The Worker expects three endpoints that **do not exist yet**. Shapes are
in `web/src/lib/schemas.ts`; they're lowerCamel to match the convention
`OfflineSong` already uses.

- `GET /api/songs?search=&sort=&order=&status=&added_by=&digested=`
  → `{ songs: SongRow[], statuses: string[], addedByUsers: string[] }`.
  Wraps the existing `ListSongsQuery` + `ListDistinctStatuses` +
  `ListDistinctAddedByUsers`.
  `SongRow` needs json tags — it has none today.
- `GET /api/songs/{slug}` → `SongDetail`. Essentially
  `RenderOfflineSong` minus the prerendered HTML: blocks instead of
  markup. 404 for unknown slugs; the Worker maps that to `notFound()`.
- `GET /api/me` → `{ email, name, isSuperadmin }`, reading the existing
  `tabitha_session` cookie.

All three should require `Authorization: Bearer $ORIGIN_TOKEN` so the JSON
API isn't public. Set it with `wrangler secret put ORIGIN_TOKEN`.

Also worth doing: extend `internal/cloudflare`'s existing purge path to
call `purgeCached()` on re-digest and status edits, so an edit shows up
immediately instead of riding out the stale window.

## Not migrated

Still Go-rendered, and linked to with plain `<a>` so they leave the SPA:

- `/songs/{id}/edit` — the ProseMirror editor (`editor/`). Its bundle is
  built into `static/` and has no Node runtime dependency; unaffected.
- `/songs/new`, `/admin/*`, `/auth/*`.
- `/healthz`, `/metrics`, `/robots.txt`, `/sitemap.xml`.
- **The offline PWA.** `sw.js` and `offline-sync.js` cache
  server-rendered HTML strings from `/offline/songs/{slug}` into
  IndexedDB (`internal/web/offline_snapshot.go`). That contract assumes
  Go renders the page. Moving it means caching JSON and letting the
  Worker's client bundle render offline — a real piece of work, and the
  reason `/offline/*` should keep pointing at Go until it's done.

## Running it

```sh
cd web
npm install
npm run test        # 36 parity tests, node
npm run typecheck
npm run dev         # vite dev, SSR inside workerd
npm run build
npm run deploy      # wrangler deploy
```

Local dev against a real Go server — put this in `web/.dev.vars`
(gitignored):

```
ORIGIN_URL="http://localhost:8080"
```

Before the first deploy, create the KV namespace and replace the
placeholder ids in `wrangler.jsonc`:

```sh
npx wrangler kv namespace create SONG_CACHE
npx wrangler kv namespace create SONG_CACHE --preview
```
