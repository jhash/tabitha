# tabitha

An app-ified version of a music transcription format Jeffrey has been
building up for years in Google Docs — chord charts with chords positioned
above the lyric words they go with. Tabitha pulls his catalog (a master
Google Sheet linking out to one Google Doc per song) into Postgres,
re-renders it on the web with the same alignment, and will eventually
replace Google Docs entirely as the place transcriptions get written and
edited.

Full design/architecture: [`docs/plans/2026-07-10-tabitha-design.md`](docs/plans/2026-07-10-tabitha-design.md).
Progress and what's left: [`todos.md`](todos.md).
Domain notes from Jeff (chord notation, TOC conventions, workflow): [`docs/jeff-domain-notes.md`](docs/jeff-domain-notes.md).
Monitoring/SLOs: [`docs/monitoring.md`](docs/monitoring.md).
Cloudflare caching: [`docs/cloudflare.md`](docs/cloudflare.md).

## Stack

Go, [chi](https://github.com/go-chi/chi) (router), [gomponents](https://www.gomponents.com/)
+ [gomponents-htmx](https://github.com/maragudk/gomponents-htmx) (server-rendered
HTML, htmx for interactivity), [pgx](https://github.com/jackc/pgx) + [sqlc](https://sqlc.dev/)
(Postgres access), [River](https://riverqueue.com) (Postgres-backed job queue),
[golang-migrate](https://github.com/golang-migrate/migrate) (schema migrations),
[goth](https://github.com/markbates/goth) (Google OAuth). Postgres for
everything — data, sessions, and the job queue.

## Local setup

Requires Go 1.25+, Postgres 17, and (only if regenerating queries) [sqlc](https://sqlc.dev/).

```sh
brew install postgresql@17
brew services start postgresql@17

createdb tabitha_dev
createdb tabitha_test           # used by the test suite
createdb tabitha_test_migrate   # isolated db for the migrate up/down round-trip test

cp .env.example .env    # then fill in values — see below
```

Load `.env` into your shell (`set -a; source .env; set +a`) or export the
vars another way, then:

```sh
go run . migrate up   # applies both tabitha's schema and River's own tables
go run . serve         # http://localhost:8080
```

For live-reload during development, use [air](https://github.com/air-verse/air)
(`go install github.com/air-verse/air@latest`) instead of `go run . serve` —
`.air.toml` is already configured to rebuild and restart on any `.go` change:

```sh
air   # same http://localhost:8080, rebuilds on save
```

### Environment variables

See [`.env.example`](.env.example) for the full list with descriptions.
Only `DATABASE_URL` is required to run `serve` locally — auth-related vars
(`GOOGLE_KEY`/`GOOGLE_SECRET`/`SESSION_SECRET`/`TOKEN_ENCRYPTION_KEY`) matter
once Google OAuth is wired up (see todos.md), `NTFY_URL` is optional, and
`CLOUDFLARE_API_TOKEN`/`CLOUDFLARE_ZONE_ID` are optional (see
`docs/cloudflare.md` — auto-purges the cache on song/status changes when set).

## Pulling in Jeff's catalog

The table-of-contents Google Sheet is readable unauthenticated (confirmed —
it's the public CSV export), so this works today without any Google login:

```sh
go run . jobs enqueue toc-sync   # inserts a toc_sync job
go run . jobs work                # processes whatever's queued, then exits
```

`serve` also runs a River client continuously in the background, so once the
server's running you don't need `jobs work` — just `jobs enqueue toc-sync`
and it'll pick it up.

Per-song digestion (`digest_song`) needs a Google OAuth token with readonly
Drive/Docs access — see todos.md for the current status of that gate.

## Tests

```sh
go test -p 1 ./...
```

`-p 1` matters: several packages' integration tests truncate and reseed
tables in the same shared `tabitha_test` database, and Go otherwise runs
different packages' tests in parallel OS processes — without `-p 1` two
packages can truncate/insert into the same tables at the same moment and
produce flaky, cross-contaminated results. Tests within a single package
already run sequentially, so `go test ./internal/web/...` (etc.) is fine
without the flag.

Most tests are pure/unit (sub-millisecond). A few integration tests hit a
real local Postgres — they read `TEST_DATABASE_URL` (defaults to
`postgres:///tabitha_test?sslmode=disable`) and run migrations
automatically; nothing needs to be pre-seeded.

One test (`TestMigrateUpCreatesAllTablesThenDownDropsThem`) exercises
`migrate down`, which drops every table — it runs against its own database
(`TEST_MIGRATE_DATABASE_URL`, defaults to
`postgres:///tabitha_test_migrate?sslmode=disable`) so it can never race
other tests relying on `tabitha_test`'s schema staying intact.

## CLI reference

```sh
tabitha serve                       # run the web server
tabitha migrate up|down             # apply/revert tabitha's + River's schema
tabitha jobs enqueue toc-sync       # queue a table-of-contents sync
tabitha jobs work                   # process queued jobs once, then exit
tabitha promote <email>             # grant an existing user the superadmin role
tabitha healthcheck                 # self-contained /healthz check (used by Docker HEALTHCHECK)
tabitha reparse                     # re-derive every current transcription's
                                     # content from its stored raw_text using
                                     # the current parser — no Google API
                                     # calls, safe to run after any parser fix
```

## Other routes worth knowing about

- `/robots.txt`, `/sitemap.xml` — SEO; the sitemap lists the home page plus
  every slugged song, so crawlers reach song pages the home page's
  hide-undigested-by-default view wouldn't otherwise link to.
- `/metrics` — Prometheus text format (request count/duration + Go runtime
  stats). Not superadmin-gated (a scraper can't OAuth-login), but should
  stay off the public internet in production — see `docs/monitoring.md`.
- `/admin/jobs` — full, cursor-paginated job history (10 at a time);
  `/admin/tools` shows just the 10 most recent plus a link there.

Superadmin promotion, including `docker exec` usage: see
[`docs/promote-admin.md`](docs/promote-admin.md).

## Song editor (`/songs/{id}/edit`)

A ProseMirror-based rich editor for a song's chord-over-lyric transcription,
superadmin-gated, living in `editor/` (Vite + React + TypeScript). It's a
build-time-only tool — `npm run build` there emits a fixed-name JS/CSS bundle
straight into this repo's `static/js/editor.js` and `static/css/editor.css`
(see `editor/vite.config.ts`), picked up by the existing Go static file
server and content-hash cache-busting (`internal/web/assets.go`). No Node.js
is needed at runtime; `docker build` runs the editor's build in its own
stage (see the Dockerfile's `editor-build` stage) so the image never needs
Node either.

```sh
cd editor
npm install
npm run build     # or: npm run dev, for live-reloading against http://localhost:5173
npm test          # vitest — unit tests for the Block<->ProseMirror-doc conversion
```

`static/js/editor.js`/`static/css/editor.css` are gitignored (generated,
like `static/dist/`) — run the build above (or `docker build`, which does it
for you) before loading `/songs/{id}/edit` locally.

The editor's document model (`editor/src/editor/schema.ts`) mirrors
`internal/transcription`'s Block/Token model directly: each transcription
Block becomes a ProseMirror node, and a chord line's Tokens become a flat
run of atomic `chordWord` nodes (chord stacked above its one lyric word),
matching the flex-based rendering `internal/web/transcription_render.go`
uses for the public song page. Saving posts the edited document back to
`POST /songs/{id}/editor-content`, which stores it as a new `manual_edit`
transcription_versions row and marks it current — the original
`google_doc_scrape` versions stay in history, never overwritten.

## Running CLI commands on the OCI instance

The running tabitha service in Docker Swarm can't access environment variables directly via `docker exec` — secrets are only mounted at startup. To run any CLI command, export the secrets as env vars first:

```bash
# SSH to server and run any command (migrate, jobs, promote, etc.)
# OCI_SERVER_IP from oracle/secrets/oci_ip.enc.json (never hard-code)
export OCI_SERVER_IP=$(sops --decrypt ../oracle/secrets/oci_ip.enc.json | jq -r '.oci_server_ip')
CONTAINER_ID=$(ssh deploy@$OCI_SERVER_IP 'docker ps --filter "label=com.docker.swarm.service.name=tabitha_tabitha" -q | head -1')

ssh deploy@$OCI_SERVER_IP "docker exec $CONTAINER_ID sh -c '
  for f in /run/secrets/tabitha_*; do
    [ -f \"\$f\" ] || continue
    name=\"\$(basename \"\$f\")\"
    key=\"\${name#tabitha_}\"
    export \"\$key=\$(cat \"\$f\")\"
  done
  ./tabitha <command>
'"
```

Examples:

```bash
# Enqueue and process a table-of-contents sync
./tabitha jobs enqueue toc-sync
./tabitha jobs work

# Promote a user to superadmin
./tabitha promote user@example.com

# Run migrations (if needed)
./tabitha migrate up
```

## Regenerating the query layer

After editing anything in `internal/db/queries/*.sql` or the migrations:

```sh
sqlc generate
```

Generated code is committed (matches this project's closest reference,
go-jake) — no build step needed to run the app.

## Docker

Multi-stage build (`golang:1.25-alpine` → `alpine:3.20`), static binary,
runs as a non-root user. Migrations are a separate explicit step, not run
automatically on container boot — same two-step flow as local dev, and
safer if this ever runs as more than one replica.

```sh
docker build -t tabitha .

# One-time (or after adding a migration) — point at your real database:
docker run --rm -e DATABASE_URL=postgres://user@host:5432/tabitha_prod tabitha migrate up

# Then run the server. --env-file is the easiest way to pass everything
# in .env at once (APP_URL, GOOGLE_KEY/SECRET, TOKEN_ENCRYPTION_KEY, etc.)
docker run --rm -p 8080:8080 --env-file .env tabitha
```

Testing against your own machine's Postgres from inside the container
(Docker Desktop on Mac/Windows): use `host.docker.internal` in
`DATABASE_URL` instead of `localhost` — confirmed working against a local
homebrew Postgres with the default trust-auth `pg_hba.conf`.

`tabitha promote <email>` works the same way against a running
container — see [`docs/promote-admin.md`](docs/promote-admin.md).
