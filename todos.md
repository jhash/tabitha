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

## In progress / next up

- [ ] Base SSR layout: Roux-derived reset, self-hosted Lora (no FOUT), htmx boost
- [ ] Home page: sortable songs table
- [ ] Song show page rendering (monospace chord/lyric blocks)
- [ ] goth Google OAuth scaffold + `/admin` gating (routes only — no real
      consent flow until Jake logs in)
- [ ] Superadmin CLI promote command + docs
- [ ] Dockerfile + verify both run modes
- [ ] `/healthz`
- [ ] README + agentic docs

## Blocked on Jake's Google login

- [ ] Real Sheets API hyperlink extraction (`google_doc_id` per song) + real
      Docs fetch in `digest_song`, using the stored OAuth token. ntfy push on
      re-auth needed.
- [ ] `/admin/tools` UI to trigger ingestion
- [ ] `/admin/users` UI to promote superadmins
- [ ] Inline admin edit/add affordances on public pages
- [ ] First full catalog digestion — then revisit the block parser against
      whatever real formatting variety shows up (expected; see design doc)
- [ ] ProseMirror editor, schema finalized against the confirmed real-world range
- [ ] Cloudflare CDN + auto-purge on song change
- [ ] Sitemap + per-page meta tags
- [ ] Monitoring/SLO dashboards for the <100ms public-render target
- [ ] `gh repo create tabitha --public` + push

## Known oddities in the real data (observed during the toc_sync smoke test)

- `status` really is free text, not a small enum: seen values so far are
  `Quality Check` (815), blank (724), `Done` (375), `Pose` (11, likely a
  Jeff-ism/typo — not correcting it, storing verbatim).
- A handful of rows look like spreadsheet section dividers rather than real
  songs (e.g. "More Rihanna" / "More Eminem" with no artist). Not filtering
  these out speculatively — revisit once we can see the actual sheet
  structure (formatting, merged cells) via the Sheets API.

## Future features (explicitly deferred, not forgotten)

- [ ] Transpose dropdown (auto key-change) — see chordchanger.com,
      go-music-theory/music-theory for primitives
- [ ] Scraping from other sources (e-chords, etc.)
- [ ] LLM-based auto-transcription: reconcile multiple scraped versions into
      a consensus on key/lyrics
- [ ] Audio-upload transcription (no source text at all, just audio)
