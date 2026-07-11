package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/jobs"
)

// AdminToolsHandler is the ingestion-trigger page: buttons to enqueue a
// toc-sync, digest one song by title, or digest a batch of undigested
// songs — plus a status table of recent job runs (state, song, last
// error) so Jake can see what's being digested and why something failed
// without going to psql. jobClient may be nil in contexts that never
// render this page for real (kept nil-safe rather than assuming callers
// always have one wired up).
func AdminToolsHandler(jobClient *river.Client[pgx.Tx]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var jobsList []jobs.JobSummary
		if jobClient != nil {
			var err error
			jobsList, err = jobs.RecentJobs(r.Context(), jobClient, 10)
			if err != nil {
				http.Error(w, "failed to load recent jobs", http.StatusInternalServerError)
				return
			}
		}
		// isSuperadmin is always true here — this route is RequireSuperadmin-gated.
		page := Page("Tools", "tabitha admin — tools", nil, true, adminToolsContent(jobsList))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func adminToolsContent(recentJobs []jobs.JobSummary) g.Node {
	return Div(
		H1(g.Text("Tools")),
		FormEl(Method("post"), Action("/admin/tools/toc-sync"),
			Button(Type("submit"), g.Text("Sync table of contents")),
		),
		FormEl(Method("post"), Action("/admin/tools/digest-song"),
			Input(Type("text"), Name("title"), Placeholder("Song title (exact match)")),
			Button(Type("submit"), g.Text("Digest song")),
		),
		FormEl(Method("post"), Action("/admin/tools/digest-batch"),
			Input(Type("number"), Name("limit"), Value("50")),
			Button(Type("submit"), g.Text("Digest a batch of undigested songs")),
		),
		H2(g.Text("Recent jobs")),
		recentJobsTable(recentJobs),
		P(A(Href("/admin/jobs"), g.Text("View all jobs →"))),
	)
}

func recentJobsTable(jobsList []jobs.JobSummary) g.Node {
	if len(jobsList) == 0 {
		return P(g.Text("No jobs run yet."))
	}
	rows := make([]g.Node, len(jobsList))
	for i, j := range jobsList {
		rows[i] = Tr(
			Td(g.Text(fmt.Sprintf("%d", j.ID))),
			Td(g.Text(j.Kind)),
			Td(g.Text(j.State)),
			Td(g.Text(fmt.Sprintf("%d", j.Attempt))),
			Td(g.Text(j.Detail)),
			Td(g.Text(j.LastError)),
		)
	}
	return Table(
		THead(Tr(
			Th(g.Text("ID")), Th(g.Text("Kind")), Th(g.Text("State")),
			Th(g.Text("Attempt")), Th(g.Text("Detail")), Th(g.Text("Last error")),
		)),
		TBody(rows...),
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

// AdminTriggerDigestBatchHandler enqueues digest_song jobs for the oldest
// (by id) undigested songs, up to the given limit — a small, checkable
// slice of the 1,925-song catalog rather than the whole thing at once, so
// unknown edge cases surface on a batch small enough to review.
func AdminTriggerDigestBatchHandler(q *db.Queries, jobClient *river.Client[pgx.Tx]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := strconv.Atoi(r.FormValue("limit"))
		if err != nil || limit <= 0 {
			http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
			return
		}
		if _, err := jobs.EnqueueDigestSongsForUndigested(r.Context(), jobClient, q, int32(limit)); err != nil {
			http.Error(w, "failed to enqueue digest batch", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/tools", http.StatusFound)
	}
}
