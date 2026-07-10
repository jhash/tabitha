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

## In progress / next up

- [ ] Inline admin edit/add affordances on public pages
- [ ] Dockerfile + verify both run modes
- [ ] `/healthz`
- [ ] agentic docs (durable `docs/architecture.md` synced from the design doc)

## Blocked on Jake's Google login

Inline edit affordances are buildable and testable now the same way /admin
itself was: a hand-constructed session bypassing real OAuth. What's
actually blocked on Jake's own Google credentials existing:

- [ ] Real Sheets API hyperlink extraction (`google_doc_id` per song) + real
      Docs fetch in `digest_song`, using the stored OAuth token. ntfy push on
      re-auth needed.
- [ ] First full catalog digestion — then revisit the block parser against
      whatever real formatting variety shows up (expected; see design doc)
- [ ] ProseMirror editor, schema finalized against the confirmed real-world range
- [ ] Cloudflare CDN + auto-purge on song change
- [ ] Sitemap + per-page meta tags
- [ ] Monitoring/SLO dashboards for the <100ms public-render target

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
- [ ] Audio-upload transcription (no source text at all, just audio)
