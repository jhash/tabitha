package web

import (
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/jobs"
)

// adminJobsPageSize is deliberately small — this page exists so a long job
// history doesn't bloat /admin/tools; keeping each page short keeps it fast
// to scan and to load.
const adminJobsPageSize = 10

// AdminJobsHandler is the full, paginated job history — /admin/tools shows
// only the most recent handful; this is the "view all" page.
func AdminJobsHandler(jobClient *river.Client[pgx.Tx]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var page jobs.JobPage
		if jobClient != nil {
			var err error
			page, err = jobs.RecentJobsPage(r.Context(), jobClient, adminJobsPageSize, r.URL.Query().Get("cursor"))
			if err != nil {
				http.Error(w, "failed to load jobs", http.StatusInternalServerError)
				return
			}
		}
		htmlPage := Page("Jobs", "tabitha admin — all jobs", nil, true, adminJobsContent(page))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = htmlPage.Render(w)
	}
}

func adminJobsContent(page jobs.JobPage) g.Node {
	return Div(
		H1(g.Text("Jobs")),
		recentJobsTable(page.Jobs),
		g.If(page.NextCursor != "",
			P(A(Href("/admin/jobs?cursor="+url.QueryEscape(page.NextCursor)), g.Text("Next →"))),
		),
	)
}
