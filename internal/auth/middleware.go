package auth

import (
	"context"
	"net/http"

	"github.com/jhash/tabitha/internal/db"
)

type contextKey int

const userContextKey contextKey = iota

// UserFromContext returns the user a RequireSuperadmin (or future
// RequireUser) middleware placed on the request context.
func UserFromContext(ctx context.Context) (db.User, bool) {
	user, ok := ctx.Value(userContextKey).(db.User)
	return user, ok
}

// RequireSuperadmin gates a handler behind a valid session belonging to a
// superadmin. Failure is always 404, never 403 — a non-superadmin (or
// logged-out visitor) shouldn't learn that the route exists at all.
func RequireSuperadmin(q *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			user, err := CurrentUser(r.Context(), q, cookie.Value)
			if err != nil || user.Role != db.UserRoleSuperadmin {
				http.NotFound(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
