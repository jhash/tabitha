package web

import (
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/jobs"
)

// AdminToolsHandler is the ingestion-trigger page: for now, a single button
// to enqueue a table-of-contents sync. digest_song stays CLI/worker-only
// until Task 23's real Google OAuth token exists to run it with.
func AdminToolsHandler(w http.ResponseWriter, r *http.Request) {
	page := Page("Tools", "tabitha admin — tools", nil, adminToolsContent())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Render(w)
}

func adminToolsContent() g.Node {
	return Div(
		H1(g.Text("Tools")),
		FormEl(Method("post"), Action("/admin/tools/toc-sync"),
			Button(Type("submit"), g.Text("Sync table of contents")),
		),
	)
}

// AdminTriggerTocSyncHandler enqueues a toc_sync job, the same job
// `tabitha jobs enqueue toc-sync` queues from the CLI.
func AdminTriggerTocSyncHandler(jobClient *river.Client[pgx.Tx]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := jobs.EnqueueTocSync(r.Context(), jobClient); err != nil {
			http.Error(w, "failed to enqueue toc sync", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/tools", http.StatusFound)
	}
}
