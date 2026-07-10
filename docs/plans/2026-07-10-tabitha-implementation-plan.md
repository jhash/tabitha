# Tabitha Implementation Plan

> **For Claude:** Executed directly in this session (single continuous build,
> not subagent-per-task or a separate executing-plans session — user gave an
> explicit go-ahead to just build). Still following TDD/bite-sized-commit
> discipline per task. See `2026-07-10-tabitha-design.md` for the full
> architecture and reasoning.

**Goal:** Get Phase 1 (everything not gated on Jake's Google OAuth login) fully
working: repo scaffold, schema, block parser, River jobs (digest stubbed),
SSR layout/styling, home page + song show page, goth login scaffold (no
consent needed to reach this point), Docker, base tests, docs.

**Architecture:** Go, chi, gomponents+gomponents-htmx, pgx+sqlc, River,
golang-migrate, goth. Postgres for everything (data, sessions, jobs).

**Tech stack:** Go 1.24, Postgres 17 (homebrew locally), River, sqlc, chi,
gomponents, htmx, Roux-inspired CSS reset, self-hosted Lora.

---

### Task 1: Go module + project skeleton

**Files:**
- Create: `go.mod`, `main.go`, `.gitignore`, `.env.example`
- Create dirs: `internal/config`, `internal/db`, `internal/web`,
  `internal/jobs`, `internal/transcription`, `static/fonts`, `css`, `migrations`

**Steps:**
1. `go mod init github.com/jhash147/tabitha` (confirm actual GH username
   before push — using `jhash` per `gh auth status`).
2. `internal/config`: load env vars (godotenv for local dev) — `DATABASE_URL`,
   `APP_URL` (default `https://tabitha.jakehash.com`), `PORT`,
   `TOKEN_ENCRYPTION_KEY`, `GOOGLE_KEY`/`GOOGLE_SECRET`, `NTFY_URL`,
   `SESSION_SECRET`.
3. `main.go`: load config, open pgx pool, start chi server on `PORT`.
4. Commit: `chore: scaffold go module and project layout`.

### Task 2: Postgres schema migrations

**Files:**
- Create: `migrations/0001_songs.up.sql` / `.down.sql`
- Create: `migrations/0002_transcription_versions.up.sql` / `.down.sql`
- Create: `migrations/0003_users_sessions.up.sql` / `.down.sql`
- Create: `migrations/0004_google_oauth_tokens.up.sql` / `.down.sql`

**Steps:**
1. Write migrations per the design doc's data model (songs, transcription_versions,
   users, sessions, google_oauth_tokens). Unique index on
   `(lower(title), lower(artist))` for songs.
2. `golang-migrate` CLI wired via a `Makefile` target (`make migrate-up` /
   `make migrate-down`).
3. Create local DB: `createdb tabitha_dev`, run migrations, confirm with
   `\d songs` in psql.
4. River's own migrations added via its CLI (`river migrate-up --database-url
   ...`) — separate from our golang-migrate set, per River's docs.
5. Commit: `feat: add initial schema migrations`.

### Task 3: sqlc setup

**Files:**
- Create: `sqlc.yaml`, `internal/db/queries/songs.sql`,
  `internal/db/queries/versions.sql`, `internal/db/queries/users.sql`,
  `internal/db/queries/tokens.sql`
- Generated: `internal/db/*.go` (gitignored source, committed generated code —
  match go-jake's convention of committing generated sqlc output)

**Steps:**
1. `sqlc.yaml` pointing at `migrations/` for schema and `internal/db/queries`
   for query files, emitting into `internal/db`.
2. Write queries: `UpsertSongFromTOC`, `GetSongBySlugOrID`, `ListSongsSorted`
   (parametrized sort column via `sqlc.arg`/multiple named queries — sqlc
   doesn't do dynamic ORDER BY well, so generate one query per sort column
   rather than fighting it), `CreateTranscriptionVersion`,
   `SetCurrentVersion`, `FindOrCreateUser`, `PromoteToSuperadmin`,
   `UpsertGoogleOAuthToken`.
3. `sqlc generate`, confirm it compiles.
4. Commit: `feat: add sqlc queries and generated code`.

### Task 4: Chord/lyric block model + parser (TDD)

**Files:**
- Create: `internal/transcription/blocks.go` (types)
- Create: `internal/transcription/parser.go`
- Test: `internal/transcription/parser_test.go`

**Step 1 — write failing tests first**, against the real sample file. Copy
`music/satisfaction-rolling-stones.txt` content into test fixtures (strip the
leading line-number column, which is a Read-tool artifact, not part of the
source doc — confirm the actual doc plaintext export has no line numbers).

```go
func TestParseSectionHeader(t *testing.T) {
    blocks := Parse("CHORUS:\n")
    require.Equal(t, []Block{{Kind: SectionHeader, Text: "CHORUS:"}}, blocks)
}

func TestParseChordLyricPair(t *testing.T) {
    input := "E                 A7                E                  A7\n" +
        "  I can't get no     satisfaction,     I can't get no      satisfaction.\n"
    blocks := Parse(input)
    require.Len(t, blocks, 1)
    require.Equal(t, ChordLyricPair, blocks[0].Kind)
    // tokens interleave text-runs and chord-atoms in stream order
    require.Equal(t, "E", blocks[0].Tokens[0].Chord)
    require.Contains(t, blocks[0].Tokens[1].Text, "I can't get no")
}

func TestParseChordOnlyLine(t *testing.T) {
    // INTRO line: chords/notes with no paired lyric line beneath
    input := "INTRO:  b b b c# d d d c# c# b     E  (E6)  E7/D   E6/D   E/D  x2\n\n"
    blocks := Parse(input)
    require.Equal(t, ChordOnlyLine, blocks[1].Kind)
}

func TestParseRepeatReferenceAsTextLine(t *testing.T) {
    blocks := Parse("(CHORUS)\n")
    require.Equal(t, TextLine, blocks[0].Kind)
    require.Equal(t, "(CHORUS)", blocks[0].Text)
}

func TestParseBarTableFormatUsesChordLyricPairModel(t *testing.T) {
    input := "|   E             /D | x3       |         E        /D | x3              E\n" +
        "| I can't get no,    | x3       | no satisfaction,    | x3      no satisfaction\n"
    blocks := Parse(input)
    require.Equal(t, ChordLyricPair, blocks[0].Kind)
    // no special "|" parsing — falls out of the generic model
}

func TestParseFullSatisfactionFileRoundTrips(t *testing.T) {
    raw := readFixture(t, "satisfaction.txt")
    blocks := Parse(raw)
    require.Equal(t, raw, Render(blocks)) // byte-for-byte re-render
}
```

**Step 2:** run `go test ./internal/transcription/... -v`, confirm all fail
(package doesn't exist yet).

**Step 3:** implement `blocks.go` (types: `BlockKind` enum
`SectionHeader|ChordLyricPair|ChordOnlyLine|TextLine`, `Token{Chord, Text
string}`, `Block{Kind BlockKind, Text string, Tokens []Token}`) and
`parser.go`:
- Split input into lines.
- Blank line → `TextLine` (empty).
- Line matching `^[A-Z0-9 '()/-]+:$` → `SectionHeader`.
- Otherwise: a line is a "chord line" if every non-space run matches a chord/
  annotation token shape (letters/digits/`#b/()x` — deliberately loose, this
  is a heuristic not a validator) AND the *next* line isn't blank/another
  chord line → pair with next line as `ChordLyricPair`. If next line is blank
  or another candidate chord line, it's `ChordOnlyLine`.
- Anything else → `TextLine` verbatim (this catches repeat-refs, stage
  directions on their own line, mid-line bleed like "When I'm").
- Tokenizing a chord+lyric pair into the interleaved stream: walk both lines
  by character column; a chord line's non-space runs become `{Chord}` tokens
  positioned at their column; the lyric line's characters are sliced into
  text-runs *between* those chord columns. This is the one place columns are
  used — only during parsing, to build the token stream. Never stored.
- `Render(blocks)` reverses this: for each `ChordLyricPair`, derive column
  positions by summing preceding text-run lengths, pad with spaces, emit
  chord line then lyric line.

**Step 4:** run tests again, iterate until green. Given the real catalog has
formatting we haven't seen yet, expect to revisit this parser after Task 23
(first real digestion) — that's expected, not a plan failure.

**Step 5:** commit: `feat: add chord/lyric block parser with token-stream model`.

### Task 5: River job queue wiring

**Files:**
- Create: `internal/jobs/toc_sync.go`, `internal/jobs/digest_song.go`,
  `internal/jobs/client.go`
- Create: `cmd/tabitha/main.go` subcommands (`jobs enqueue toc_sync`, etc.) —
  or a flag on the main binary; keep it a single binary, not multiple.

**Steps:**
1. `river.NewClient` wired against the pgx pool.
2. `toc_sync` job args: none. Work: fetch
   `https://docs.google.com/spreadsheets/d/1uqJfZ7TyH-Ii_dJGvby6MH-uVyH0xFkG7RQ3bxARezQ/export?format=csv&gid=0`
   (follow the redirect — confirmed working unauthenticated earlier this
   session), parse CSV, upsert each row via `UpsertSongFromTOC`. Per-row
   `google_doc_id` hyperlink extraction requires the Sheets API + stored
   OAuth token — stub this with a TODO and a log line until Task 23.
3. `digest_song` job args: `SongID`. Work: **stubbed** — returns
   `river.JobCancel(errors.New("no OAuth token yet"))` until Task 23. Write
   the real fetch+parse+version-write logic then, using the parser from
   Task 4.
4. CLI: `go run . jobs enqueue toc-sync` inserts a River job directly.
5. Commit: `feat: wire River job queue with toc_sync and stubbed digest_song`.

### Task 6: Base SSR layout, Roux reset, self-hosted Lora

**Files:**
- Create: `internal/web/layout.go` (gomponents `Page(...)` wrapper)
- Create: `static/css/reset.css` (Roux-derived), `static/css/style.css`
- Create: `static/fonts/Lora-{Regular,Bold,Italic,BoldItalic}.ttf` (download
  from Google Fonts' open-source repo, self-host, do not link to
  fonts.googleapis.com)
- Test: `internal/web/layout_test.go` (mirrors go-jake's
  `font_loading_test.go` almost exactly)

**Steps:**
1. Download Lora static TTFs into `static/fonts/`.
2. `@font-face` blocks with `font-display: optional`, `<link rel="preload"
   as="font">` in `<head>` for the regular weight.
3. Roux reset adapted (thoughtbot/roux is Sass-based scaffolding, not a
   drop-in reset file — pull just its box-sizing/normalize primitives into
   plain CSS rather than pulling in the whole Sass toolchain, since this repo
   doesn't otherwise need Sass).
4. `layout.go`: `Page(title, description string, body ...Node) Node` — html5
   boilerplate, header, optional sidebar slot, `<main class="container">` with
   max-width, htmx script tag with boost attribute on `<body hx-boost="true">`.
5. Tests (adapt go-jake's `font_loading_test.go` almost verbatim): preload
   link present, no Google Fonts CDN reference, `font-display: optional`
   present, font files exist on disk.
6. Commit: `feat: add SSR layout, Roux-derived reset, self-hosted Lora`.

### Task 7: Home page — sortable songs table

**Files:**
- Create: `internal/web/home.go`, `internal/web/home_test.go`

**Steps:**
1. Route `GET /` and `GET /songs?sort=title|artist|updated|added|status|added_by`
   — htmx-friendly (`hx-get` on column headers, `hx-target` swaps just the
   `<tbody>`).
2. `ListSongsSorted` sqlc query per sort column (Task 3).
3. Render table: columns Title (links to `/songs/{id}`), Artist, Status,
   Last Updated, Added, Added By. Sort arrows as links preserving current
   sort in query string.
4. Rendered-HTML test: table headers present, row links to correct show-page
   URL, sort param round-trips.
5. Commit: `feat: add home page with sortable songs table`.

### Task 8: Song show page

**Files:**
- Create: `internal/web/song_show.go`, `internal/web/song_show_test.go`

**Steps:**
1. Route `GET /songs/{id}`.
2. Render `current_version.content` blocks: `chord_lyric_pair`/`chord_only_line`
   in a monospace `<pre>`-like block (derive display columns from token
   stream per Task 4's `Render`), `section_header` as a heading, `text_line`
   verbatim.
3. Rendered-HTML test against a fixture built from the Satisfaction blocks —
   assert visual column alignment survives (compare rendered chord-line
   length/spacing against expected).
4. Commit: `feat: add song show page rendering`.

### Task 9: goth Google OAuth scaffold (no real consent yet)

**Files:**
- Create: `internal/web/auth.go`

**Steps:**
1. goth google provider configured from `GOOGLE_KEY`/`GOOGLE_SECRET` (empty
   in this phase — routes exist and compile, consent flow untested until
   Task 23 when real credentials exist).
2. Routes: `/auth/google`, `/auth/google/callback`, `/auth/logout`. Scopes:
   default profile/email **plus** `https://www.googleapis.com/auth/drive.readonly`
   requested at consent — read-only, confirmed once at Task 23 that no
   write scope is ever requested.
3. Scaffold (routes only, not wired to a real provider) for future
   non-Google providers — matches go-jake's multi-provider shape, so adding
   GitHub/etc. later is additive.
4. `/admin` route group behind a `RequireSuperadmin` middleware — returns 404
   (not 403, don't reveal the route exists) for non-superadmins.
5. Commit: `feat: scaffold goth Google OAuth and admin route gating`.

### Task 10: Superadmin CLI promote command

**Files:**
- Create: `cmd/tabitha/promote.go` (or a flag/subcommand on the single binary)
- Create: `docs/promote-admin.md`

**Steps:**
1. `go run . promote-admin --email someone@example.com` — looks up user by
   email, sets `is_superadmin = true`. Errors clearly if the user hasn't
   logged in yet (no row to promote).
2. Document both direct (`go run .`) and `docker exec <container>
   /app/tabitha promote-admin --email ...` usage in
   `docs/promote-admin.md`, linked from the README.
3. Commit: `feat: add superadmin promotion CLI command`.

### Task 11: Dockerfile + local non-Docker run verification

**Files:**
- Create: `Dockerfile`, `.dockerignore`, `docker-compose.yml` (optional local
  Postgres alternative to homebrew, for parity testing)

**Steps:**
1. Multi-stage Dockerfile: build stage (Go build + Vite build for the future
   ProseMirror bundle), slim runtime stage (distroless or alpine), non-root
   user, `HEALTHCHECK` hitting `/healthz`.
2. Verify: `go run .` locally against homebrew Postgres 17 works.
3. Verify: `docker build` + `docker run` against the same DB (or
   docker-compose Postgres) works, `/healthz` returns 200.
4. Commit: `feat: add production Dockerfile, verify local and Docker run modes`.

### Task 12: /healthz + base test pass + docs

**Files:**
- Create: `internal/web/health.go`
- Create/Update: `README.md`, `docs/architecture.md`, `todos.md`

**Steps:**
1. `GET /healthz` — pings DB, returns 200/503 JSON.
2. Run full test suite, confirm every test <100ms
   (`go test ./... -v -json | ...` timing check, or just eyeball `go test
   -v` per-test elapsed since Go prints it).
3. Write README (what tabitha is, local dev setup, env vars, how to run
   tests, how to promote a superadmin), `docs/architecture.md` (durable
   agentic-docs version of the design doc, kept in sync as the app evolves),
   `todos.md` (running list — update as each task above completes, plus the
   TODOs-for-later the user listed: transpose UI, e-chords scraping,
   LLM auto-transcription/consensus, audio-upload transcription).
4. Commit: `docs: add README, architecture docs, todos`.

---

## Phase 2 (after Jake's Google OAuth login) — milestone-level, detailed later

Deliberately not broken into TDD-granular tasks yet — several of these depend
on what the real catalog's Google Docs actually look like once we can read
them, which we won't know until the first real `digest_song` run.

1. Wire real Sheets API hyperlink extraction + real Docs fetch in
   `digest_song`, using the stored OAuth token. ntfy notification on
   token/auth failure.
2. `/admin/tools` UI: buttons to enqueue `toc_sync` / `digest_song` manually.
3. `/admin/users` UI: list users, promote to superadmin from the browser.
4. Inline admin edit/add affordances on the public table and show pages,
   gated by superadmin session.
5. Run the first full catalog digestion. Re-open Task 4's parser against
   whatever formatting variety shows up; expand the fixture-driven test suite
   to match.
6. ProseMirror editor: schema finalized against the confirmed real-world
   formatting range, React island via Vite, SSR chrome around it.
7. Cloudflare CDN in front of public pages + auto-purge webhook on song
   change.
8. Sitemap + per-page meta tags.
9 . Monitoring/SLO dashboards for the <100ms public-render target.
10. `gh repo create tabitha --public` + push.
