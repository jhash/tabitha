# Promoting a superadmin

Only superadmins can reach `/admin`. A brand-new user has no way to
promote themselves — someone already-promoted must run this CLI command,
or promote them from `/admin/users` if you're already a superadmin
yourself.

The target user must have logged in via Google at least once first (the
command promotes an *existing* row in `users`; it doesn't create one).

## Direct (local dev, or a bare-metal/systemd deploy)

```sh
go run . promote jeff@example.com
```

Or against a built binary:

```sh
tabitha promote jeff@example.com
```

Both read the same environment as every other subcommand (`DATABASE_URL`
at minimum — see `.env.example`).

## Inside a running container

Once the app is running under Docker (see the Dockerfile — Task 15), run
the same command inside the container rather than from the host, so it
picks up the container's `DATABASE_URL`:

```sh
docker exec <container-name-or-id> tabitha promote jeff@example.com
```

With Docker Compose, substitute the service name instead:

```sh
docker compose exec app tabitha promote jeff@example.com
```

## Output

- Success: `tabitha: promoted jeff@example.com to superadmin`, exit code 0.
- Unknown email (no such user, or they haven't logged in yet): a clear
  error naming the email, exit code 1.
- No email given: a usage error, exit code 1.
