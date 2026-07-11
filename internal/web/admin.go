package web

import (
	"net/http"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/auth"
)

// AdminHomeHandler is the admin landing page behind RequireSuperadmin.
// Task 12 adds an ingestion-trigger section here.
func AdminHomeHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	// isSuperadmin is always true here — this route is RequireSuperadmin-gated.
	page := Page("Admin", "tabitha admin", nil, true,
		Div(
			H1(g.Text("Admin")),
			P(g.Text("Signed in as "+user.Email)),
			P(A(Href("/admin/songs"), g.Text("Manage songs"))),
			P(A(Href("/admin/users"), g.Text("Manage users"))),
			P(A(Href("/admin/tools"), g.Text("Tools"))),
			A(Href("/auth/logout"), g.Text("Log out")),
		),
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(w)
}
