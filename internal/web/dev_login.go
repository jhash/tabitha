package web

import (
	"net/http"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/db"
)

// DevLoginHandler mints a superadmin session with no auth check, for
// e2e tests to drive superadmin-gated pages without a real Google OAuth
// login. Only ever mounted when config.DevLoginEnabled is true (see
// NewRouter) — never in production.
func DevLoginHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "e2e-test@example.com", Name: "E2E Test"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := q.PromoteToSuperadmin(ctx, user.Email); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		token, expiresAt, err := auth.CreateSession(ctx, q, user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: token, Path: "/", Expires: expiresAt})
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
