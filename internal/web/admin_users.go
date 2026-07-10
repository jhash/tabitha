package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/db"
)

// AdminUsersHandler lists every user and lets a superadmin promote others.
func AdminUsersHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := q.ListUsers(r.Context())
		if err != nil {
			http.Error(w, "failed to load users", http.StatusInternalServerError)
			return
		}

		page := Page("Users", "tabitha admin — users", nil, adminUsersTable(users))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func adminUsersTable(users []db.User) g.Node {
	return Table(
		THead(Tr(Th(g.Text("Email")), Th(g.Text("Name")), Th(g.Text("Role")), Th())),
		TBody(
			g.Map(users, func(u db.User) g.Node {
				return Tr(
					Td(g.Text(u.Email)),
					Td(g.Text(u.Name)),
					Td(g.Text(string(u.Role))),
					Td(g.If(u.Role != db.UserRoleSuperadmin, promoteForm(u.ID))),
				)
			}),
		),
	)
}

func promoteForm(userID int64) g.Node {
	return FormEl(Method("post"), Action(fmt.Sprintf("/admin/users/%d/promote", userID)),
		Button(Type("submit"), g.Text("Promote to superadmin")),
	)
}

// AdminPromoteUserHandler promotes one user to superadmin. Looks the user
// up by ID first (rather than taking an email straight from the form) so
// an unknown/bad ID 404s the same way every other admin route does.
func AdminPromoteUserHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		user, err := q.GetUserByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if _, err := q.PromoteToSuperadmin(r.Context(), user.Email); err != nil {
			http.Error(w, "failed to promote user", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/users", http.StatusFound)
	}
}
