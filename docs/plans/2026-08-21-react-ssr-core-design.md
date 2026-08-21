# React/TanStack SSR core rearchitecture — design

Sub-project 1 of 4 in the broader React-centric frontend initiative:

1. **This doc** — core rearchitecture: React (TanStack Start) SSR replaces
   gomponents for the home page and song show page only; Go/React
   communication; Cloudflare caching.
2. Unified show/play/edit page (merge into one inline-editable UI,
   permission-gated) — separate spec, depends on #1.
3. React Native app (replaces the Capacitor WebView shell) — separate spec,
   depends on #1/#2.
4. Agent harness for live-coding in production (hot reload + background git
   commits) — separate spec, largely independent of 1–3.

Node-vs-Go-backend was a live question going in; resolved below (Go stays
the backend, no rewrite).

## Why

Jeff's app is currently 100% server-rendered gomponents + htmx. The plan
is to modernize the frontend (React, better caching, an eventual React
Native app, inline editing) without abandoning the Go backend that's
already built — most of the app (auth, jobs, admin, the DB layer, the
editor's data API) has nothing to do with the two pages actually being
rebuilt here.

## Non-goals (this spec)

- Rewriting the Go backend in Node. Rejected — see "Backend language"
  below.
- Rebuilding Play mode or the song editor. Explicitly out of scope; they
  keep working exactly as today, just relocated under `/old`.
- CRDT / offline editing with sync. Noted as a future constraint (see
  "Offline & data model"), not built here.
- Feature-flagged dual-serving or gradual rollout. Straight cutover.

## Backend language: staying on Go

TanStack Start needs a JS/TS server runtime (Node/Bun/Deno) to do SSR —
that's unavoidable regardless of what the Go backend does. Given that a
second runtime is already required for rendering, there is no forcing
function to also move business logic, the DB layer, auth, jobs, or admin
into it. Go stays the backend and system of record; the new runtime's job
is SSR + presentation only, talking to Go over JSON.

## Architecture

- New `web/` app: **TanStack Start** (React, file-based routing, SSR +
  server functions), runtime **Bun**.
- Ships as its own container in the same docker stack as Go
  (`oracle/services/tabitha/stack.yml`), joined to an internal
  network only (e.g. `gojake-net`) — **no Traefik labels, no public
  route of its own**. The existing `tabitha` (Go) service keeps sole
  ownership of `proxy-net` / `Host(tabitha.jakehash.com)`.
- Go's chi router proxies `GET /` and `GET /songs/{idOrSlug}` to the Bun
  sidecar over Docker service DNS (internal, not public internet).
- Public traffic path is unchanged in shape: Cloudflare → Traefik → Go →
  (proxied) → Bun. One TLS cert, one DNS record, one ingress.
- Future-portability note: the Start app talks to Go through one thin
  fetch-adapter module, not scattered inline `fetch()` calls, and Go's
  API is reachable over the public internet regardless (the future React
  Native app needs that anyway). If the sidecar is ever swapped for
  Cloudflare Workers edge SSR, that's a deploy-target + adapter-config
  change, not a rewrite.

## Routes

All existing Go HTML view handlers (home, song show, play, edit, song/new,
admin/*) are **moved** (not aliased) under `/old/*`, same handlers, no
logic changes:

- `/old/` — home
- `/old/songs/{idOrSlug}` — show
- `/old/songs/{idOrSlug}/play` — play
- `/old/songs/{idOrSlug}/edit` — edit
- `/old/songs/new` — new song form
- `/old/admin/*` — admin section

Their real canonical paths (`/songs/{id}/play`, `/songs/{id}/edit`,
`/admin/*`, etc.) **stop resolving** the moment this ships — accepted
gap, not redirected. Play mode and admin come back at their real paths
once #2 rebuilds them; `/old` exists for manual comparison/reference
during development, not as a live fallback.

New `/api/*`: JSON-only, no HTML, superadmin gating preserved per-route
exactly as today.

- Existing `/songs/{id}/editor-content` GET/POST move to
  `/api/songs/{id}/content`.
- New: song listing for the home page (lifts the existing query/shaping
  logic out of `song_query.go` into JSON instead of gomponents).
- New: song detail for the show page (lifts `currentVersionBlocks` et al.
  into JSON — title, artist, blocks, key, status).
- New: `GET /api/auth/whoami` — takes the forwarded session cookie,
  returns `{loggedIn, role}`. This is how Start's loaders (which have no
  direct DB access) resolve whether to render superadmin-only affordances
  (e.g. inline edit entry points). Same endpoint the React Native app
  will reuse later.

Canonical `GET /` and `GET /songs/{idOrSlug}` now proxy to the Start app
instead of hitting a Go handler directly.

## Mutations ("server actions")

TanStack Start server functions run on the Bun sidecar and call Go's
`/api/*` endpoints directly, forwarding the session cookie — e.g. an
inline status edit calls a server function, which POSTs to
`/api/admin/songs/{id}/status` on Go, which does the write and runs the
existing Cloudflare purge. Same shape the editor's save flow already
uses today, generalized. No separate RPC/GraphQL layer — Go's JSON API
is the entire contract. Payloads stay JSON (not protobuf) — small
payloads at this scale, and JSON keeps tooling/debuggability simple on
both sides; revisit only if this becomes a measured bottleneck.

## Auth bridging

Start has no direct DB access, so it can't call `CurrentUser` itself.
The root loader calls `GET /api/auth/whoami` once per request with the
incoming cookie header and gets back role info — that's the only auth
state the home/show pages need (whether to show superadmin affordances).
Actual session validation logic stays single-sourced in Go
(`internal/auth`), never reimplemented in JS.

## Caching (Cloudflare)

Reuses `internal/cloudflare`'s existing purge-on-write hooks
(`digest_song`, `toc_sync`, status edits) — add the two new canonical
URLs to what gets purged, same trigger points, no new mechanism.

Cache policy: Cloudflare caches the anonymous-rendered HTML for `/` and
`/songs/{slug}`. Any request carrying a session cookie bypasses cache
(Cache Rule: bypass on cookie present), so a logged-in superadmin always
sees live data and edit affordances, never a stale anonymous-cached page.

## Offline & data model

Offline storage moves from pre-rendered HTML strings to structured JSON,
in anticipation of eventual local-first editing:

- `/offline/songs/{slug}` returns JSON (title, artist, blocks, key,
  contentHash) for the show page instead of a baked `HTML` string.
  `PlayHTML` stays as-is (Play isn't rebuilt here, still Go-rendered).
- The client stores that JSON in IndexedDB. The **same React chord/lyric
  component** renders it both during normal SSR/hydration and offline
  from cache — one renderer, no online/offline drift, and the client
  already holds transcription state as structured data rather than
  opaque HTML.
- The service worker keeps serving the app shell offline; TanStack
  Router navigates client-side from IndexedDB-cached song JSON when
  there's no network. Same experience as today; Play's swipe UX is
  untouched (not rebuilt in #1).

**CRDT / offline-editing-sync is explicitly out of scope for #1.** Noted
only as a constraint on today's choices: keep transcription state as the
structured JSON blocks it already is (`internal/transcription`'s Block
model), don't build anything that would fight a future Yjs/Automerge
layer. Actual local-first editing becomes its own future project once #2
(unified inline edit) exists.

## Testing

- Go: unchanged patterns — unit tests for `/api/*` handlers' JSON shape,
  `httptest`+goquery still covers everything staying gomponents-rendered
  under `/old`, router tests updated for the new prefix and proxy
  behavior (proxy target mockable via `httptest.Server`).
- Start app: Vitest for the shared chord/lyric render component (same
  style as `editor/`'s existing unit tests) — one component, tested
  once, used for SSR/hydration/offline alike.
- Playwright (`e2e/`): extend the existing suite to cover home + song
  show once rebuilt, and add offline-mode coverage (service worker +
  IndexedDB genuinely need a real browser — wasn't covered before since
  offline had no dedicated e2e tests).

## Rollout

Straight cutover, no feature flag or dual-serving. `/old` is a
development-time reference, not a live A/B split. Ship when home + song
show are solid in the new stack; extend the Cloudflare purge config the
same day.
