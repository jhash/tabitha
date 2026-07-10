package web

import (
	"net/http"

	"github.com/jhash/tabitha/internal/db"
)

// HealthzHandler reports whether tabitha can actually reach Postgres, not
// just whether the process is alive — a hung DB pool should fail this so
// an orchestrator (Docker Swarm, etc.) can route around/restart it.
func HealthzHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := q.Ping(r.Context()); err != nil {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
