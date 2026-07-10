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

### Environment variables

See [`.env.example`](.env.example) for the full list with descriptions.
Only `DATABASE_URL` is required to run `serve` locally — auth-related vars
(`GOOGLE_KEY`/`GOOGLE_SECRET`/`SESSION_SECRET`/`TOKEN_ENCRYPTION_KEY`) matter
once Google OAuth is wired up (see todos.md), and `NTFY_URL` is optional.

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
```

Superadmin promotion, including `docker exec` usage: see
[`docs/promote-admin.md`](docs/promote-admin.md).

## Regenerating the query layer

After editing anything in `internal/db/queries/*.sql` or the migrations:

```sh
sqlc generate
```

Generated code is committed (matches this project's closest reference,
go-jake) — no build step needed to run the app.
