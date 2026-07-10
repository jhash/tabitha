package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// NewRouter builds tabitha's full HTTP handler: static assets, public
// routes, and (when Google credentials are configured) login and the
// superadmin-gated /admin section.
func NewRouter(cfg config.Config, q *db.Queries) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	r.Get("/", HomeHandler(q))
	r.Get("/songs/{id}", SongShowHandler(q))

	if auth.GoogleConfigured(cfg) {
		configureGoogleAuth(cfg)
		mountAuthRoutes(r, cfg, q)
	}

	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.RequireSuperadmin(q))
		r.Get("/", AdminHomeHandler)
		r.Get("/users", AdminUsersHandler(q))
		r.Post("/users/{id}/promote", AdminPromoteUserHandler(q))
	})

	return r
}
