package web

import (
	"net/http"
	"strings"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/slug"
)

// SongNewHandler renders the "add a song from scratch" form: title and
// artist plus the same optional catalog metadata toc_sync populates from
// Jeff's spreadsheet. Submitting creates the song and redirects straight
// into the ProseMirror editor (see CreateSongHandler) rather than an
// index/detail step in between.
func SongNewHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := Page("New song", "tabitha admin — new song", nil, true, songNewContent())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func songNewContent() g.Node {
	return Div(
		H1(g.Text("New song")),
		FormEl(Method("post"), Action("/songs"),
			labeledInput("Title", "title", true),
			labeledInput("Artist", "artist", false),
			labeledInput("Genre", "genre", false),
			labeledInput("Film/Show/Album", "film_show_album", false),
			labeledInput("Decade", "decade", false),
			labeledInput("Bob tag", "bob_tag", false),
			labeledInput("Status", "status", false),
			labeledInput("Source URL", "source_url", false),
			labeledInput("Notes", "notes", false),
			labeledInput("Transpose hint", "transpose_hint", false),
			Button(Type("submit"), g.Text("Create and start editing")),
		),
		P(A(Href("/"), g.Text("Cancel"))),
	)
}

func labeledInput(label, name string, required bool) g.Node {
	return P(
		Label(g.Text(label+": "), Input(Type("text"), Name(name), g.If(required, Required()))),
	)
}

// CreateSongHandler creates a song from the /songs/new form, assigns it a
// slug immediately (the same collision-resolution toc_sync uses — see
// internal/slug), and redirects into its editor by that slug so a freshly
// created song's URL matches the canonical one every other song gets once
// digested.
func CreateSongHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		if title == "" {
			http.Error(w, "title is required", http.StatusBadRequest)
			return
		}

		var addedBy *int64
		if user, ok := auth.UserFromContext(r.Context()); ok {
			addedBy = &user.ID
		}

		song, err := q.CreateSong(r.Context(), db.CreateSongParams{
			Title:         title,
			Artist:        strings.TrimSpace(r.FormValue("artist")),
			Genre:         strings.TrimSpace(r.FormValue("genre")),
			FilmShowAlbum: strings.TrimSpace(r.FormValue("film_show_album")),
			Decade:        strings.TrimSpace(r.FormValue("decade")),
			BobTag:        strings.TrimSpace(r.FormValue("bob_tag")),
			Status:        strings.TrimSpace(r.FormValue("status")),
			SourceUrl:     strings.TrimSpace(r.FormValue("source_url")),
			Notes:         strings.TrimSpace(r.FormValue("notes")),
			TransposeHint: strings.TrimSpace(r.FormValue("transpose_hint")),
			AddedByUserID: addedBy,
		})
		if err != nil {
			http.Error(w, "failed to create song", http.StatusInternalServerError)
			return
		}

		existingSlugs, err := q.ListAllSongSlugs(r.Context())
		if err != nil {
			http.Error(w, "failed to assign slug", http.StatusInternalServerError)
			return
		}
		taken := make(map[string]bool, len(existingSlugs))
		for _, s := range existingSlugs {
			taken[s.Slug] = true
		}
		newSlug := slug.ResolveUnique(slug.Slugify(song.Title), slug.Slugify(song.Artist), func(s string) bool {
			return taken[s]
		})
		if err := q.SetSongSlug(r.Context(), db.SetSongSlugParams{ID: song.ID, Slug: newSlug}); err != nil {
			http.Error(w, "failed to assign slug", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/songs/"+newSlug+"/edit", http.StatusFound)
	}
}
