# Cloudflare CDN

## Cache-busting (done, no config needed)

Static assets (`style.css`, `reset.css`, `htmx.min.js`, the Lora font) are
served with a content-hash `?v=` query param computed at server startup —
see `internal/web/assets.go`. A versioned URL is cached forever
(`immutable`); an unversioned one gets a short 60s max-age. This means a
new deploy that changes a static file automatically gets a new URL
Cloudflare has never cached, with no manual purge needed. This is the
bigger fix — it was the actual cause of the stale-CSS incident that
prompted this work, and it needs zero credentials.

## Auto-purge on content change (needs `CLOUDFLARE_API_TOKEN`/`CLOUDFLARE_ZONE_ID`)

HTML pages (the home page, individual song pages) aren't asset-versioned —
if Cloudflare caches them at all, a manual purge or short TTL is the only
way to keep them fresh. `internal/cloudflare` wraps Cloudflare's
[purge_cache API](https://developers.cloudflare.com/api/operations/zone-purge)
and is wired into the places content actually changes:

- `digest_song` job: purges the song's own page + the home page after a
  successful digest.
- `toc_sync` job: purges the home page after a sync (title/status/genre
  changes show up there).
- Inline/bulk status edits (`/admin/songs/{id}/status`,
  `/admin/songs/bulk-status`): purge the home page.

All of these are **no-ops** unless both `CLOUDFLARE_API_TOKEN` and
`CLOUDFLARE_ZONE_ID` are set (see `Client.Configured()`) — same pattern as
Google OAuth being optional in dev. A purge failure never fails the
triggering request/job; it's logged and swallowed, since a stale cached
page is a much smaller problem than losing a digest or a status update
over an API hiccup.

### Getting the credentials

1. Cloudflare dashboard → My Profile → API Tokens → Create Token → use
   the "Edit zone DNS" template as a starting point, or a custom token
   scoped to **Zone → Cache Purge → Edit** for the `jakehash.com` zone
   specifically (not "All zones") — least-privilege, this token can't
   touch DNS, WAF rules, or anything else.
2. Zone ID: Cloudflare dashboard → the `jakehash.com` zone's Overview page
   → right sidebar → "Zone ID".
3. Set both as `CLOUDFLARE_API_TOKEN`/`CLOUDFLARE_ZONE_ID` in the
   production environment (`.env`, or however secrets are injected into
   the deploy).

**Not verified against a real Cloudflare account** — I (Claude) don't have
your API token, so `internal/cloudflare` is tested only against a mock
HTTP server (`purge_test.go`). Once real credentials are in place, worth
triggering one status edit and confirming (via Cloudflare's dashboard
Analytics, or just watching the page update immediately) that the purge
actually lands.
