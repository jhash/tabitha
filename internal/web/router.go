package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riverqueue/river"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// NewRouter builds tabitha's full HTTP handler: static assets, public
// routes, and (when Google credentials are configured) login and the
// superadmin-gated /admin section. jobClient may be nil wherever nothing
// under /admin/tools is exercised (most routes never touch it).
func NewRouter(cfg config.Config, q *db.Queries, jobClient *river.Client[pgx.Tx]) http.Handler {
	SetAssetVersions(LoadAssetVersions("static"))

	reg := prometheus.NewRegistry()
	metrics := newRequestMetrics(reg)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(metrics.middleware)
	r.Use(auth.OptionalUser(q))

	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", staticCacheHeaders(http.StripPrefix("/static/", fileServer)))

	// Not superadmin-gated: Prometheus scrapers can't complete an OAuth
	// login. Only request counts/durations are exposed — nothing
	// sensitive — but this should still be reached only from inside a
	// private network in production, not opened to the public internet.
	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	r.Get("/healthz", HealthzHandler(q))
	r.Get("/robots.txt", RobotsTxtHandler(cfg.AppURL))
	r.Get("/sitemap.xml", SitemapHandler(q, cfg.AppURL))
	r.Get("/", HomeHandler(q))
	r.Get("/songs/{idOrSlug}", SongShowHandler(q))
	r.With(auth.RequireSuperadmin(q)).Get("/songs/{id}/edit", SongEditHandler(q))

	if auth.GoogleConfigured(cfg) {
		configureGoogleAuth(cfg)
		mountAuthRoutes(r, cfg, q)
	}

	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.RequireSuperadmin(q))
		r.Get("/", AdminHomeHandler)
		r.Get("/users", AdminUsersHandler(q))
		r.Post("/users/{id}/promote", AdminPromoteUserHandler(q))
		r.Post("/songs/bulk-status", AdminBulkSetSongStatusHandler(q))
		r.Post("/songs/{id}/status", AdminSetSongStatusHandler(q))
		r.Get("/tools", AdminToolsHandler(jobClient))
		r.Get("/jobs", AdminJobsHandler(jobClient))
		r.Post("/tools/toc-sync", AdminTriggerTocSyncHandler(jobClient))
		r.Post("/tools/digest-song", AdminTriggerDigestSongHandler(q, jobClient))
		r.Post("/tools/digest-batch", AdminTriggerDigestBatchHandler(q, jobClient))
	})

	return r
}
