package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jhash/tabitha/internal/db"
)

// NewRouter builds tabitha's full HTTP handler: static assets and the
// public routes. Auth/admin routes are added in later tasks.
func NewRouter(q *db.Queries) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	r.Get("/", HomeHandler(q))

	return r
}
