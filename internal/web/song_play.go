package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// SongPlayHandler renders Play mode: a fullscreen, paginated ereader-style
// view of the same transcription the show page renders, for swipe/keyboard/
// arrow-button navigation between screens (see static/js/play.js and
// static/css/play.css for the pagination mechanics).
func SongPlayHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idOrSlug := chi.URLParam(r, "idOrSlug")

		song, err := resolveSongByIDOrSlug(r, q, idOrSlug)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if _, parseErr := strconv.ParseInt(idOrSlug, 10, 64); parseErr == nil && song.Slug != "" {
			http.Redirect(w, r, "/songs/"+song.Slug+"/play", http.StatusMovedPermanently)
			return
		}

		blocks, key, hasVersion, err := currentVersionBlocks(r.Context(), q, song)
		if err != nil {
			http.Error(w, "failed to load transcription", http.StatusInternalServerError)
			return
		}
		// Nothing to paginate yet — send the viewer to the show page's
		// own not-yet-digested message instead of a blank reader.
		if !hasVersion {
			http.Redirect(w, r, songShowHref(song), http.StatusFound)
			return
		}

		description := fmt.Sprintf("%s, as performed by %s", song.Title, song.Artist)
		page := PagePlay(song.Title, description, songPlayContent(song, blocks, key))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func songPlayContent(song db.Song, blocks []transcription.Block, key string) g.Node {
	return Div(ID("play-root"), Class("play-root"), g.Attr("data-show-href", songShowHref(song)),
		Div(ID("play-scroller"), Class("play-scroller"),
			Div(Class("play-columns"),
				Div(Class("play-page-inset"),
					H1(g.Text(song.Title)),
					g.If(song.Artist != "", P(Class("byline"), g.Text("As performed by "+song.Artist))),
					g.If(key != "", P(Class("key"), g.Text("Key: "), B(g.Text(strings.ToUpper(key))))),
					renderTranscriptionHTML(omitDuplicateHeaderLines(blocks, song)),
				),
			),
		),
		Button(Class("play-close"), g.Attr("aria-label", "Close play mode"), Type("button"), g.Text("×")),
		Button(Class("play-prev"), g.Attr("aria-label", "Previous page"), Type("button"), g.Text("‹")),
		Button(Class("play-next"), g.Attr("aria-label", "Next page"), Type("button"), g.Text("›")),
		Script(Type("module"), Src(versionedHref("/static/js/play.js", assets.PlayJS))),
	)
}
