# Conductor Setup for tabitha Worktrees

Enables running multiple tabitha dev servers (one per Conductor worktree) on
different ports, sharing a single Postgres database.

## Configuration

### Scripts

- **setup.sh** — copies `.env`/`.env.local` from the main repo checkout if missing, verifies Go + air, runs `go mod download`
- **run.sh** — auto-detects an available port starting from 8080, sets `PORT`/`APP_URL`, and runs `air`

### Environment

All worktrees read `DATABASE_URL` from `.env` (copied from the main checkout by
setup.sh), so they all point at the same Postgres database. Only `PORT` and
`APP_URL` are overridden per-worktree by run.sh.

## Usage

```bash
conductor setup   # copies .env, installs air if needed
conductor run      # starts air on an auto-assigned port
```

Each worktree gets:
- Independent source tree (git branch/state)
- Same Postgres connection (`DATABASE_URL`) — all instances share data
- Different port, auto-assigned starting at 8080

### Troubleshooting

**Port already in use**
- run.sh uses `nc -z localhost $PORT` to probe, increments from 8080, up to 100 attempts

**Wrong .env picked up**
- setup.sh only copies `.env`/`.env.local` if not already present in the worktree; edit the worktree's copy directly to override per-instance (e.g. a different `DATABASE_URL` for isolated testing)
