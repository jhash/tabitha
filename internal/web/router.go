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
	"github.com/jhash/tabitha/internal/cloudflare"
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
	cfClient := &cloudflare.Client{APIToken: cfg.CloudflareAPIToken, ZoneID: cfg.CloudflareZoneID}

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
	r.Get("/sw.js", ServiceWorkerHandler())
	r.Get("/offline/manifest.json", OfflineManifestHandler(q))
	r.Get("/offline/songs/{slug}", OfflineSongHandler(q))
	r.Get("/", HomeHandler(q))
	r.With(auth.RequireSuperadmin(q)).Get("/songs/new", SongNewHandler())
	r.With(auth.RequireSuperadmin(q)).Post("/songs", CreateSongHandler(q))
	r.Get("/songs/{idOrSlug}", SongShowHandler(q))
	r.Get("/songs/{idOrSlug}/play", SongPlayHandler(q))
	r.With(auth.RequireSuperadmin(q)).Get("/songs/{idOrSlug}/edit", SongEditHandler(q))
	r.With(auth.RequireSuperadmin(q)).Get("/songs/{idOrSlug}/editor-content", GetSongEditorContentHandler(q))
	r.With(auth.RequireSuperadmin(q)).Post("/songs/{idOrSlug}/editor-content", PostSongEditorContentHandler(q))

	// e2e-test-only: mints a superadmin session with no auth check, so
	// Playwright etc. can drive superadmin-gated pages without a real
	// Google OAuth login. Never mounted unless DEV_LOGIN_ENABLED=true.
	if cfg.DevLoginEnabled {
		r.Get("/dev-login", DevLoginHandler(q))
	}

	if auth.GoogleConfigured(cfg) {
		configureGoogleAuth(cfg)
		mountAuthRoutes(r, cfg, q)
	}

	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.RequireSuperadmin(q))
		r.Get("/", AdminHomeHandler)
		r.Get("/songs", AdminSongsHandler)
		r.Get("/users", AdminUsersHandler(q))
		r.Post("/users/{id}/promote", AdminPromoteUserHandler(q))
		r.Post("/songs/bulk-status", AdminBulkSetSongStatusHandler(q, cfg.AppURL, cfClient))
		r.Post("/songs/{id}/status", AdminSetSongStatusHandler(q, cfg.AppURL, cfClient))
		r.Get("/tools", AdminToolsHandler(jobClient))
		r.Get("/jobs", AdminJobsHandler(jobClient))
		r.Post("/tools/toc-sync", AdminTriggerTocSyncHandler(jobClient))
		r.Post("/tools/digest-song", AdminTriggerDigestSongHandler(q, jobClient))
		r.Post("/tools/digest-batch", AdminTriggerDigestBatchHandler(q, jobClient))
	})

	return r
}
