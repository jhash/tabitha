package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// SongEditHandler is a placeholder edit page behind RequireSuperadmin,
// reachable from the "Edit" link songShowContent shows superadmins.
// Task 14 replaces the raw <pre> below with the real ProseMirror island;
// the route, gating, and page chrome stay the same.
func SongEditHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		song, err := q.GetSongByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		blocks, _, hasVersion, err := currentVersionBlocks(r.Context(), q, song)
		if err != nil {
			http.Error(w, "failed to load transcription", http.StatusInternalServerError)
			return
		}

		// isSuperadmin is always true here — this route is RequireSuperadmin-gated.
		page := Page("Edit "+song.Title, "tabitha admin — edit", nil, true, songEditContent(song, blocks, hasVersion))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func songEditContent(song db.Song, blocks []transcription.Block, hasVersion bool) g.Node {
	return Div(
		H1(g.Text("Edit "+song.Title)),
		P(g.Text("The real editor (ProseMirror) lands in the next task. For now, here's the current raw transcription:")),
		g.If(!hasVersion,
			P(Class("no-content"), g.Text("This song hasn't been digested from Jeff's Google Doc yet.")),
		),
		g.If(hasVersion,
			Pre(Class("transcription"), g.Text(transcription.Render(blocks))),
		),
		P(A(Href(songShowHref(song)), g.Text("Back to song"))),
	)
}
