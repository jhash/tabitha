package web

import (
	"net/http"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/auth"
)

// AdminHomeHandler is a placeholder landing page behind RequireSuperadmin.
// Tasks 11/12 add real content (user promotion, ingestion triggers) here.
func AdminHomeHandler(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	page := Page("Admin", "tabitha admin", nil,
		Div(
			H1(g.Text("Admin")),
			P(g.Text("Signed in as "+user.Email)),
			A(Href("/auth/logout"), g.Text("Log out")),
		),
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(w)
}
