package web

import (
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/db"
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
		FormEl(Method("post"), Action("/admin/tools/digest-song"),
			Input(Type("text"), Name("title"), Placeholder("Song title (exact match)")),
			Button(Type("submit"), g.Text("Digest song")),
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

// AdminTriggerDigestSongHandler enqueues a digest_song job for one song,
// looked up by exact (case-insensitive) title match. A single-song
// trigger, not a full-catalog run — see todos.md for why.
func AdminTriggerDigestSongHandler(q *db.Queries, jobClient *river.Client[pgx.Tx]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		title := r.FormValue("title")
		song, err := q.GetSongByTitle(r.Context(), title)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := jobs.EnqueueDigestSong(r.Context(), jobClient, song.ID); err != nil {
			http.Error(w, "failed to enqueue digest", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/tools", http.StatusFound)
	}
}
