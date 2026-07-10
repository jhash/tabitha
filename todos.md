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
      Doesn't yet handle the multi-key-per-doc case (Eye of the Tiger) —
      stores the doc's full text as one version; splitting on Jeff's
      page-break-separated keys is still open, see below. Unit-tested
      (hyperlink extraction, doc-ID parsing, plain-text flattening, error
      paths); the actual Sheets/Docs API happy path was verified manually
      against Great Balls of Fire rather than mocked, per Jake's own
      "digest one specific song" test plan.

## In progress / next up

- [ ] agentic docs (durable `docs/architecture.md` synced from the design doc)
- [ ] Splitting a multi-key doc (Eye of the Tiger's Gm-then-Cm pattern)
      into separate `transcription_versions` rows on the page break —
      `digest_song` currently stores the whole doc as one version
- [ ] Sitemap + per-page meta tags

## Explicitly paused (Jake's call, 2026-07-10) — not abandoned

- [ ] ProseMirror editor (React island via Vite) — replaces the raw <pre>
      placeholder at `/songs/{id}/edit`
- [ ] Monitoring/SLO dashboards for the <100ms public-render target
- [ ] Cloudflare CDN + auto-purge on song change

### Parser classification gaps found against the two new real fixtures

Round-trip is safe on both (see Done above) — these are about the parser
producing richer structure, not fixing corruption:

- Section headers with real content on the *same* line get missed and
  fall back to an opaque `TextLine` (safe, just unstructured): e.g.
  `INTRO:  /G  | Gm  (F)  Gm  (F)  Gm  (Bb/D)  Eb | x4  Gm` and
  `OUTRO: x4 ish` (Eye of the Tiger), `INSTRUMENTAL:  C  F  G  F  C  x2  C
  Wellllll` (Great Balls of Fire). Current `sectionHeaderRe` requires the
  colon to be the last character on the line.
- Section headers with a lowercase letter suffix don't match:
  `VERSE 1a:` — `sectionHeaderRe`'s char class doesn't allow lowercase.
- Multi-word parenthesized chord groups (space inside the parens) don't
  get recognized as a single chord-line token, since tokenization splits
  on whitespace first: `(/F /F /F#  G)`, `(/G /A /B /C)`.
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
