# Tabitha — Design

**Date:** 2026-07-10
**Status:** Validated with Jake, ready for implementation.

## What this is

Tabitha is an app-ified version of a music transcription format Jeffrey has been
building up for years in Google Docs — chord charts with chords positioned above
the lyric words they go with, plus section labels (VERSE, CHORUS, etc.), repeat
markers, and assorted freeform annotations. Jeffrey keeps a master Google Sheet
("table of contents") listing every song he's transcribed, each row linking out
to the actual Google Doc.

Tabitha's job: pull that catalog into Postgres, re-render it on the web with the
same visual alignment, and eventually replace Google Docs entirely as the place
Jeffrey and Jake write and edit these transcriptions.

Reference implementations for the SSR/htmx style this app should follow:
`~/Development/articles/go-jake` (Go, gomponents, goth, chi — closest match,
already solves self-hosted Lora font loading and goth multi-provider auth) and
`~/Development/articles/lovely-rs` (Rust equivalent, more of a Rust-learning
vehicle for Jake than a production reference).

## Stack decision: Go

Research comparison (full reasoning in the stack-research fork output, summarized
here):

- **Job queue:** [River](https://riverqueue.com) — Postgres-native, transactional
  enqueue, batch insert, admin UI, SOC2 Type II, no Rust equivalent matches its
  maturity (closest are apalis-postgres and underway, both younger/thinner).
- **Music theory primitives:** `github.com/go-music-theory/music-theory` (459
  stars, actively maintained, covers notes/scales/chords/keys) beats the
  fragmented Rust equivalents — useful once we build the transpose feature.
- **Prior art already exists in Go for this exact app shape:** go-jake already
  has goth (Google OAuth), gomponents+htmx, chi, pgx, sqlc, gorilla
  sessions/csrf, and — notably — self-hosted Lora with `font-display: optional`
  and preload, already implemented and tested.
- Rust wins on memory footprint and tail latency, which matters on the free-tier
  OCI box, but tabitha is a small text-CRUD app, not a latency-critical proxy —
  Go's win on iteration speed and job-relevance dominates here.

**Core libraries:** chi (router), gomponents + gomponents-htmx (SSR rendering),
pgx + sqlc (Postgres access), goth (OAuth), River (jobs), golang-migrate
(schema migrations), go-music-theory/music-theory (future transpose).

## Data model

### `songs`

One row per canonical (title, artist) pair, sourced from the TOC sheet:

- `title`, `artist`, `genre`, `film_show_album`, `decade`, `bob_tag`, `notes`,
  `transpose_hint` — verbatim sheet columns. (`bob_tag` and `transpose_hint`
  are opaque — meaning TBD once we see more of the sheet; stored, not
  interpreted.)
- `status` — free text, not an enum. We've only observed "Done" so far; don't
  guess the full value set Jeffrey uses.
- `source_url` — the sheet's SCRAPE LINK column (Jeffrey's original source,
  e.g. an ultimate-guitar tab). Provenance, and groundwork for the "scrape
  from other sources" TODO.
- `google_doc_id` — extracted from the TITLE cell's hyperlink via the Sheets
  API (not visible in plain CSV/gviz export — confirmed by direct check).
- `current_version_id` — FK to the currently-published `transcription_versions`
  row.
- Unique constraint on `(lower(title), lower(artist))` (normalized) — this is
  the title/artist clash-detection and dedup the user asked for.

### `transcription_versions`

- FK `song_id`.
- `kind` — free text, default `'primary'`. Allows `'alternate'` or similar
  later without a schema change. Not over-modeled per the user's explicit
  "don't over-optimize this upfront" on versioning.
- `source` — `google_doc_scrape` | `manual_edit`.
- `raw_text` — the full verbatim digested plaintext. Cheap insurance / audit
  trail, satisfies "store it fully."
- `content` — JSONB, the parsed block model (below). Source of truth for
  rendering and for the ProseMirror editor.
- `key`, `capo` — nullable, best-effort regex-extracted from the doc's own
  "Key:" line. Non-fatal if extraction fails.
- `is_current` boolean, `created_by`, `created_at`.

### `google_oauth_tokens`

- FK `user_id` (the superadmin who authenticated).
- `encrypted_access_token`, `encrypted_refresh_token` — `bytea`, AES-GCM
  encrypted with a key from `TOKEN_ENCRYPTION_KEY` env var. Never touch
  plaintext tokens in SQL or logs.
- `scope`, `expiry`.
- Ingestion jobs run as this stored identity — no separate service account.
  One login serves both admin authentication *and* the Drive/Docs read
  credential.

### `users` / `sessions`

- `users`: email, name, avatar_url, `is_superadmin` boolean.
- `sessions`: Postgres-backed (not just signed cookies) — durability across
  restarts, matches "postgres for everything."

### Jobs (River)

- `toc_sync` — fetch the TOC sheet (public CSV export works unauthenticated
  for values; Sheets API + stored OAuth token needed for per-row hyperlinks →
  `google_doc_id`), upsert `songs`, enqueue `digest_song` for new/changed rows.
- `digest_song` — fetch the Google Doc via Docs/Drive API (readonly scope,
  stored OAuth token), parse into blocks, write a new `transcription_versions`
  row, flip `is_current`.
- Manually enqueued via CLI/`/admin/tools` for now. Periodic re-check is a
  future cron calling the same enqueue path — not built until asked for.

## Chord/lyric block model

A transcription's `content` is an ordered list of blocks:

- `section_header` — e.g. "CHORUS:"
- `chord_lyric_pair` — a chord line paired with the lyric line beneath it
- `chord_only_line` — chord line with no paired lyric (intros, instrumentals)
- `text_line` — catch-all: blank lines, repeat-references like "(CHORUS)",
  inline stage directions like "(drums)", mid-line section bleed — anything
  that doesn't fit the above. Rendered as-is, no forced semantics.

**Key decision, informed by ChordPro and OnSong's own handling of this exact
legacy format:** chords are NOT stored as `{chord, column}` offset pairs
against the lyric text. They're stored as an **interleaved token stream** —
alternating text-runs and chord-atoms in document order, same model ChordPro
uses (`[G]` inline in the lyric stream) and the same normalization OnSong
applies when importing legacy space-aligned charts like Jeffrey's.

```json
{
  "kind": "chord_lyric_pair",
  "tokens": [
    { "chord": "E" },
    { "text": "  I can't get no     satisfaction,     I can't get no      satisfaction." },
    { "chord": "A7" }
  ]
}
```

Why this beats storing column offsets: offsets rot the instant lyric text is
edited (insert a word before the anchor, the stored column is now wrong — an
entire bug class). The token-stream form also maps directly onto ProseMirror's
actual document model (inline content = a mix of text nodes and an atom node),
so editing the chord's position is just moving a node in the stream — no
coordinate bookkeeping anywhere.

Column position for monospace re-rendering (byte-for-byte reproduction of
Jeffrey's original alignment) is *derived* at render time by summing preceding
text-run lengths — computed, never stored, so there's exactly one source of
truth.

The bar/table format at the end of some charts (`| E /D | x3 | ...`) falls out
of this same model for free as long as we don't try to parse `|` specially —
it's still just text-runs and chord-atoms in a line.

## Rendering

- MVP: render `chord_lyric_pair` blocks in a monospace font specifically for
  that content (body text stays Lora/serif). This reproduces Jeffrey's exact
  spacing with zero guesswork.
- Later (once the full catalog is digested and we know the real range of
  formatting): prettier absolute/inline-positioned chord tags over serif text,
  using the same token stream — no data migration needed to get there.

## Auth model

- goth, Google provider only for now. Scaffold the route shape so other
  providers / general user login can be added later without rework.
- `/admin` prefix, superadmin-gated. Not linked from public nav.
- Additional OAuth scopes requested at consent: `drive.readonly` (covers
  Sheets + Docs read access under one scope) — **read-only, always**, per
  explicit requirement. No write scopes ever requested for Jeffrey's Google
  account data.
- Promotion to superadmin: CLI command (works standalone and via
  `docker exec`, documented in README) plus a `/admin/users` UI page for
  promoting other users once at least one superadmin exists.
- Ingestion is manually triggered (CLI now, `/admin/tools` UI once OAuth is
  wired) — periodic background re-sync is a later addition.
- If the stored OAuth token goes stale (revoked/expired refresh), ingestion
  jobs fail gracefully and fire an ntfy push notification prompting
  re-auth — no silent retries against a dead credential.

## Styling

- Minimal CSS reset, Roux-inspired (https://github.com/thoughtbot/roux).
- Lora everywhere, self-hosted (not Google Fonts CDN), preloaded, with
  `font-display: optional` to avoid FOUT/FOIT — mirrors go-jake's existing
  (tested) approach.
- Plain black and white. Readable max-width on all public pages. Plain
  header/sidebar.

## Editor

- ProseMirror, custom schema mirroring the block/token model above — the one
  deliberate exception to SSR-first, per the user's explicit call-out.
  Compiled via Vite as a React island; surrounding layout (header, sidebar,
  chrome) stays SSR via gomponents.
- Inline admin affordances (edit/add) render into the same public-facing
  pages/tables, gated by superadmin session — no separate admin-only UI shell.

## Ops

- `/healthz` — checks DB connectivity.
- Structured request-timing middleware, SLO target: public pages render
  <100ms.
- Cloudflare in front of public pages, auto-purge on song add/update.
- Dynamic `sitemap.xml`, per-page meta tags (song pages + home).
- Runs locally without Docker (homebrew Postgres 17) and via a production
  Dockerfile — both verified.
- Env-driven base URL, defaults to `tabitha.jakehash.com` in production.

## Testing

- Unit tests for the block parser against the real catalog's formatting
  variety (starting with `music/satisfaction-rolling-stones.txt`, expanding
  once the full catalog is digested).
- Rendered-HTML assertions, htmx endpoint behavior tests, ProseMirror
  schema/editor tests, fast e2e.
- Target: every test runs in <100ms.

## Sequencing

Everything not gated on Jake's Google login happens first: repo scaffold, full
schema, block parser + tests, River wiring (with `digest_song` stubbed),
SSR layout/styling, home page + song show page, goth scaffold (login UI works,
but no consent yet), Docker, base test suite, docs.

Then: Jake logs in via the UI (goth), gets promoted to superadmin via CLI,
and triggers the first real `toc_sync` + `digest_song` run via `/admin/tools`.
Only after seeing the real catalog's full formatting variety do we harden the
parser against edge cases we can't currently anticipate, and build the
ProseMirror editor against the confirmed schema range.
