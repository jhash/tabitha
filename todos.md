# Tabitha — TODOs / Progress

Living document. Updated as work progresses — see `docs/plans/` for the
full design doc and implementation plan this tracks against.

## Done

- [x] Git repo initialized, design doc + implementation plan written
- [x] Go module scaffold, env-based config (`internal/config`)
- [x] Postgres schema migrations (users w/ role enum, songs, transcription_versions,
      sessions, google_oauth_tokens) — embedded, run via `tabitha migrate up|down`
- [x] Chord/lyric block parser (`internal/transcription`) — interleaved
      token-stream model, byte-for-byte verified against the real Satisfaction
      sample
- [x] sqlc query layer, integration-tested against real Postgres (dedup
      upsert, superadmin promotion)
- [x] River job queue wired (`toc_sync` working end-to-end against Jeff's
      **real live sheet** — 1,925 songs pulled into local `tabitha_dev`;
      `digest_song` stubbed pending OAuth)
- [x] Base SSR layout: Roux-derived reset, self-hosted Lora (no FOUT), htmx boost
- [x] Home page: sortable songs table
- [x] Song show page rendering (monospace chord/lyric blocks)
- [x] `gh repo create tabitha --public` + push (renamed the pre-existing
      2017 `jhash/tabitha` — a PhantomJS e-chords/Ultimate-Guitar scraper —
      to `tabitha-chord-scraper` first; see Future features below, it's
      relevant to the scraping TODO)
- [x] goth Google OAuth wired (provider registered only when
      `GOOGLE_KEY`/`GOOGLE_SECRET` are set; explicit `email`, `profile`,
      `drive.readonly` scopes) + `/admin` gated behind `RequireSuperadmin`
      (404, not 403, on failure) — real login still blocked on Jake's own
      Google credentials existing in `.env` (Task 23)
- [x] Fixed a real cross-process test-isolation bug: several packages'
      integration tests share one `tabitha_test` Postgres database, and
      `go test ./...` runs packages in parallel OS processes by default.
      Concurrent `TRUNCATE`s could deadlock, and the migrate up/down
      round-trip test was dropping every table out from under other
      packages mid-run. Fixed with a transaction-scoped advisory lock
      around each TRUNCATE, moved the destructive migrate test to its own
      `tabitha_test_migrate` database, and documented `go test -p 1 ./...`
      as the required way to run the full suite (see README).

- [x] Superadmin CLI promote command (`tabitha promote <email>`, docs at
      `docs/promote-admin.md` covering direct + `docker exec` use) +
      `/admin/users` UI to list/promote from the browser
- [x] `/admin/tools` UI to trigger a toc-sync from the browser (enqueues the
      same job the CLI does; verified a real `river_job` row lands, not just
      that the handler ran)
- [x] Inline superadmin "Edit" link on the song show page (`OptionalUser`
      middleware — session-aware but never gates the public page itself) +
      a placeholder `/songs/{id}/edit` page (raw transcription, superadmin-
      gated) for the ProseMirror editor to replace next
- [x] `air` configured for live-reload during local dev (`.air.toml`,
      rebuilds/restarts on `.go` changes)
- [x] Production Dockerfile (multi-stage, non-root, ~44MB image) — verified
      both run modes for real: `go run .`/`air` locally, and a built image
      running `migrate up` then `serve` against the host's real Postgres via
      `host.docker.internal`, including static assets and non-root user.
      Moved ahead of the ProseMirror editor at Jake's request (2026-07-10)
      so he can start on separate deployment work sooner.

- [x] `/healthz` — actually pings Postgres, not just "process alive." Added
      a self-contained `tabitha healthcheck` CLI subcommand (no curl/wget
      dependency) and wired it into the Dockerfile's `HEALTHCHECK`.
      Verified against a real running container. Prioritized ahead of
      everything else at Jake's request, to unblock a Docker Swarm deploy.
- [x] Captured Jake's 2026-07-10 meeting notes with Jeff — see
      [`docs/jeff-domain-notes.md`](docs/jeff-domain-notes.md) for the full
      writeup (transpose workflow, TOC color-coding conventions, chord
      notation edge cases, live-performance UX priorities, and several new
      future-feature ideas). The canonical notation legend Jeff uses is now
      saved at `music/template-song.txt`.
- [x] Jake's real Google OAuth credentials wired up, logged in, promoted
      to superadmin — `/admin` etc. all reachable for real now.
- [x] Added two real transcriptions Jake pulled from Jeff's actual Google
      Docs (`eye-of-the-tiger.txt`, `great-balls-of-fire.txt`) as parser
      fixtures — the Satisfaction sample alone wasn't representative.
      Round-trip (`Parse` → `Render` reproduces the original byte-for-byte)
      holds on both, confirmed via a new regression test. Concrete gaps in
      *classification* (not correctness — nothing corrupts) found and
      logged below.

- [x] `digest_song` implemented for real (was stubbed): `internal/auth`
      gained `ValidGoogleToken` (fetches the most-recently-stored token,
      refreshes via the real oauth2 flow if expired, re-persists —
      tested against a fake token endpoint, not just happy-path
      assumptions). `internal/jobs` gained the Sheets API call to resolve
      a title's `google_doc_id` from its cell hyperlink (title column
      found by header name, same convention as the CSV parser — it isn't
      necessarily column A) and the Docs API call to fetch content,
      flattened to plain text and run through the existing parser.
      `/admin/tools` got a "Digest song" form (exact title match) so a
      single song can be tested without running the whole catalog.
      Multi-key-per-doc splitting (Eye of the Tiger) handled separately,
      see below. Unit-tested (hyperlink extraction, doc-ID parsing,
      plain-text flattening, error paths); the actual Sheets/Docs API
      happy path was verified manually against Great Balls of Fire rather
      than mocked, per Jake's own "digest one specific song" test plan.
- [x] Multi-key docs (Eye of the Tiger's Gm-then-Cm pattern) handled for
      real: Jeff doesn't insert an actual Docs API page break, he mashes
      Enter, so the real delimiter is a long blank-line run (~50 in the
      real doc, vs. a max of 2 anywhere else) rather than a `PageBreak`
      structural element — found by inspecting the real fetched doc after
      a naive PageBreak-based split did nothing. `digest_song` now splits
      on that and keeps only the last section (the original key),
      discarding the transposed copy on top per Jake's call, rather than
      storing both as separate versions. Verified against the real doc:
      stored version dropped from 154 lines (both keys) to 53 (Cm only).

## In progress / next up

- [ ] agentic docs (durable `docs/architecture.md` synced from the design doc)
- [ ] ProseMirror editor (React island via Vite) — replaces the raw <pre>
      placeholder at `/songs/{id}/edit`
- [ ] River job-queue Prometheus metrics (see `docs/monitoring.md` — not
      built yet, `/admin/jobs` + psql are the current way to check for
      stuck jobs)

## Done (2026-07-11 session)

- [x] Sitemap (`/sitemap.xml`) + `robots.txt` + Open Graph meta tags
      (`og:title`/`og:description`/`og:type` on every page)
- [x] Monitoring: `/metrics` (Prometheus text format — request
      count/duration + Go runtime stats), `docs/monitoring.md` spelling out
      the actual SLOs (100ms p95 public-render, <0.1% 5xx rate) and what's
      NOT covered (job-queue metrics, alerting, tracing)
- [x] Cloudflare auto-purge on song/status changes
      (`internal/cloudflare`, wired into `digest_song`/`toc_sync`/status
      handlers) — **not verified against a real Cloudflare account**, only
      against a mock server; needs `CLOUDFLARE_API_TOKEN`/`CLOUDFLARE_ZONE_ID`
      to do anything at all. See `docs/cloudflare.md`.
- [x] **Real, catalog-wide parser bug found and fixed**: `chordTokenRe`
      required the full word "min" for a minor chord — the standard
      bare-`m` shorthand (`Bm`, `Em`, `Am`...) never matched, silently
      losing chord detection (bolding, chord-word rendering) for *any*
      chord line using it. Affected ~1410 of ~1500 transcription
      versions — nearly the whole catalog. Also fixed: `∆`/`Δ`
      (major7 shorthand, 584 versions) and Unicode fraction glyphs in
      section headers like "CHORUS ½:" (32 versions). Found by tracing
      why some chords on a real song page weren't bolded; fixed via
      `tabitha reparse` against the whole catalog (0 round-trip
      mismatches afterward, no re-digestion/Google API calls needed).
- [x] Responsive chord-chart rendering: chords no longer reconstruct
      fixed-width monospace columns — each chord+word is its own
      wrappable flex item, so charts reflow on mobile instead of forcing
      horizontal scroll. No data-model change needed (the token stream
      was already ChordPro-equivalent positionally). Verified in-browser
      at 1280px and ~375-469px.
- [x] Slugs (`songs.slug`, assigned during `toc_sync`, artist-suffix on
      collision) — `/songs/{slug}` is canonical, `/songs/{id}` 301s to it.
- [x] Home page: hide-undigested-by-default filter, bulk/inline Status
      editing for superadmins, dedup-match fix for artist name format
      differences (byline vs. TOC), Source column added then removed per
      feedback (see `songs.source_site` — a plain constant now, not
      URL-derived).
- [x] Bold + upper-cased chord keys on song show pages.
- [x] `/admin/jobs` — full paginated job history, separate from
      `/admin/tools`'s 10-most-recent view.
- [x] Content-hash cache-busting for static assets (root-caused a
      production stale-CSS incident — no Cloudflare purge, no explicit
      Cache-Control, so Cloudflare's own default policy served style.css
      stale for ~1h43m with nothing to invalidate it).
- [x] Google OAuth refresh-token bug: goth's Google provider wasn't
      requesting `access_type=offline`/`prompt=consent`, so a stored
      token could end up with no refresh_token and `digest_song` jobs
      failed once the access token aged out. Fixed; requires re-login at
      `/auth/google` in production to take effect.

## Explicitly paused (Jake's call, 2026-07-10) — not abandoned

*(all three of these were un-paused and finished in the 2026-07-11
session above — kept here for history)*

### Parser classification gaps found against the two new real fixtures

Round-trip is safe on all fixtures (see Done above) — these are about the
parser producing richer structure, not fixing corruption. Two of the four
gaps originally listed here are now fixed (see 2026-07-11 Done above:
lowercase section-header suffix, fraction glyphs); these two remain open:

- Section headers with real content on the *same* line get missed and
  fall back to an opaque `TextLine` (safe, just unstructured): e.g.
  `INTRO:  /G  | Gm  (F)  Gm  (F)  Gm  (Bb/D)  Eb | x4  Gm` and
  `OUTRO: x4 ish` (Eye of the Tiger), `INSTRUMENTAL:  C  F  G  F  C  x2  C
  Wellllll` (Great Balls of Fire). Current `sectionHeaderRe` requires the
  colon to be the last character on the line.
- A line combining multiple repeat-references plus new content misses
  entirely: `(CHORUS)   (VERSE 3)                END: (C) (C) (C)` — falls
  back to opaque `TextLine`. Note `(VERSE 3)` references a *specific*
  earlier verse, not just "the chorus" — repeat-reference resolution
  eventually needs to target arbitrary earlier sections, not just chorus.

Revisit once real digestion exists and there's a bigger, real sample to
tune against — fixing these against just two files risks overfitting.

## Known oddities in the real data (observed during the toc_sync smoke test)

- `status` really is free text, not a small enum: seen values so far are
  `Quality Check` (815), blank (724), `Done` (375), `Pose` (11, likely a
  Jeff-ism/typo — not correcting it, storing verbatim).
- A handful of rows look like spreadsheet section dividers rather than real
  songs (e.g. "More Rihanna" / "More Eminem" with no artist). Not filtering
  these out speculatively — revisit once we can see the actual sheet
  structure (formatting, merged cells) via the Sheets API.
- At least one row has a stray note in the ARTIST column instead of a real
  artist ("40th anniversary song so had to come out before 1985") — found
  while spot-checking that the home page's artist sort actually orders
  correctly. Storing verbatim, same reasoning as above.

## Future features (explicitly deferred, not forgotten)

- [ ] Transpose dropdown (auto key-change) — see chordchanger.com,
      go-music-theory/music-theory for primitives
- [ ] Scraping from other sources (e-chords, etc.)
- [ ] LLM-based auto-transcription: reconcile multiple scraped versions into
      a consensus on key/lyrics
- [ ] Audio-upload transcription (no source text at all, just audio) — Jeff
      separately asked whether AI-listen-and-transcribe raises legal
      questions; not researched yet, ask Jake before spending time on it
- [ ] Genre as many-to-many (own table + join table, not a `songs` column)
- [ ] Crowd-sourced quality-check workflow + a superadmin "seal of
      approval" step — implies more than the current user/superadmin role
      split
- [ ] Per-user notation preference (∆ / maj7 / M for major-seventh, etc.)
- [ ] Per-song sheet-music PDF, shown inline/popup (Jeff has these separately)
- [ ] Live-performance reading mode: swipe-between-pages (not scroll),
      auto-detect phone vs. iPad layout — see OnSong (what Jeff currently
      uses on cruise gigs) as prior art for this specifically
- [ ] Live MIDI recording (or other live-ingestion path) as an alternative
      way to create a transcription

Full context for all of the above: [`docs/jeff-domain-notes.md`](docs/jeff-domain-notes.md).
