package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/db"
)

// SongEditHandler renders the edit page behind RequireSuperadmin, reachable
// from the "Edit" link songShowContent shows superadmins. The page mounts
// the ProseMirror React island (editor/), which loads/saves the song's
// transcription via GetSongEditorContentHandler/PostSongEditorContentHandler
// rather than anything rendered server-side here.
func SongEditHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		song, err := resolveSongByIDOrSlug(r, q, chi.URLParam(r, "idOrSlug"))
		if err != nil {
			http.NotFound(w, r)
			return
		}

		_, _, hasVersion, err := currentVersionBlocks(r.Context(), q, song)
		if err != nil {
			http.Error(w, "failed to load transcription", http.StatusInternalServerError)
			return
		}

		// isSuperadmin is always true here — this route is RequireSuperadmin-gated.
		page := Page("Edit "+song.Title, "tabitha admin — edit", nil, true, songEditContent(song, hasVersion))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func songEditContent(song db.Song, hasVersion bool) g.Node {
	return Div(
		H1(g.Text("Edit "+song.Title)),
		g.If(!hasVersion,
			P(Class("no-content"), g.Text("This song hasn't been digested from Jeff's Google Doc yet. You can still write a transcription from scratch below.")),
		),
		Link(Rel("stylesheet"), Href(versionedHref("/static/css/editor.css", assets.EditorCSS))),
		Div(ID("tabitha-editor-root"), g.Attr("data-song-id", strconv.FormatInt(song.ID, 10))),
		Script(Type("module"), Src(versionedHref("/static/js/editor.js", assets.EditorJS))),
		P(A(Href(songShowHref(song)), g.Text("Back to song"))),
	)
}
