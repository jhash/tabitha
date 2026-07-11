# Monitoring & SLOs

## Metrics endpoint

`GET /metrics` — Prometheus text format, from a per-process registry (not
the global default registry, so tests stay isolated). Not superadmin-gated
(a scraper can't complete an OAuth login), but it should only be reachable
from a private network in production, not exposed to the public internet —
firewall it at the load balancer/reverse proxy, not in application code.

Exposed series:

- `tabitha_http_requests_total{method, status}` — request count.
- `tabitha_http_request_duration_seconds{method}` — request duration
  histogram (default Prometheus buckets).
- Everything the `client_golang` default collectors add automatically (Go
  runtime: goroutines, GC pauses, memory).

## SLOs

| Target | Metric | Threshold |
|---|---|---|
| Public page render latency | `tabitha_http_request_duration_seconds` (GET, non-/admin, non-/static) | p95 < 100ms |
| Availability | `tabitha_http_requests_total` | 5xx rate < 0.1% over any rolling 1h window |
| Job queue health | River's own `river_job` table (see below — no custom metric yet) | no job stuck `retryable` for > 1h |

The 100ms public-render target is Jake's own bar for a htmx-boosted SSR
app with no client-side rendering to hide behind — if a page render is
slow, it's slow for the user, full stop.

### What's not covered yet

- **Job-queue metrics**: River doesn't have a built-in Prometheus
  exporter wired up here. For now, `/admin/jobs` and `river_job` itself
  (via psql) are the way to check for stuck/failing jobs. A
  `river_job`-scraping custom collector (count by state) would close this
  gap — not built yet, tracked in `todos.md`.
- **Alerting**: no Alertmanager/PagerDuty wiring. This assumes whoever
  operates tabitha in production points their own Prometheus at
  `/metrics` and layers alerting on top — this repo doesn't run its own
  monitoring stack.
- **Distributed tracing**: `go.opentelemetry.io/*` shows up in `go.mod` as
  an indirect dependency (pulled in transitively, likely via goth or
  river), not something tabitha instruments itself. Worth revisiting if
  request latency ever needs breaking down by internal call (DB query vs.
  template render vs. external API), but a single-process SSR app with a
  100ms target doesn't need it yet.

## How to point Prometheus at it

Standard scrape config, pointed at wherever tabitha's container is
reachable from your Prometheus instance (not the public internet):

```yaml
scrape_configs:
  - job_name: tabitha
    static_configs:
      - targets: ["tabitha:8080"] # or wherever /metrics is reachable
```
