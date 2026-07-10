package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	g "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// SongShowHandler renders a single song's page: its transcription if one has
// been digested yet, otherwise a plain not-yet-digested placeholder (the
// state every song is in until Task 23's real digestion pipeline runs).
func SongShowHandler(q *db.Queries) http.HandlerFunc {
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

		blocks, hasVersion, err := currentVersionBlocks(r.Context(), q, song)
		if err != nil {
			http.Error(w, "failed to load transcription", http.StatusInternalServerError)
			return
		}

		viewer, _ := auth.UserFromContext(r.Context())
		viewerIsSuperadmin := viewer.Role == db.UserRoleSuperadmin

		description := fmt.Sprintf("%s, as performed by %s", song.Title, song.Artist)
		page := Page(song.Title, description, nil, songShowContent(song, blocks, hasVersion, viewerIsSuperadmin))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func currentVersionBlocks(ctx context.Context, q *db.Queries, song db.Song) ([]transcription.Block, bool, error) {
	if song.CurrentVersionID == nil {
		return nil, false, nil
	}

	row, err := q.GetSongCurrentVersion(ctx, song.ID)
	if err != nil {
		return nil, false, err
	}

	blocks, err := transcription.UnmarshalDocument(row.TranscriptionVersion.Content)
	if err != nil {
		return nil, false, err
	}
	return blocks, true, nil
}

func songShowContent(song db.Song, blocks []transcription.Block, hasVersion, viewerIsSuperadmin bool) g.Node {
	return Div(
		H1(g.Text(song.Title)),
		P(Class("byline"), g.Text("As performed by "+song.Artist)),
		g.If(viewerIsSuperadmin,
			P(Class("admin-affordance"), A(Href(fmt.Sprintf("/songs/%d/edit", song.ID)), g.Text("Edit"))),
		),
		g.If(!hasVersion,
			P(Class("no-content"), g.Text("This song hasn't been digested from Jeff's Google Doc yet.")),
		),
		g.If(hasVersion,
			Pre(Class("transcription"), g.Text(transcription.Render(omitDuplicateTitleByline(blocks, song)))),
		),
	)
}

// omitDuplicateTitleByline drops the digested doc's own leading title and
// "As performed by: <artist>" lines when this page's H1/byline already
// show the same thing — otherwise both render (see the "Downtown" bug
// report). Only trims when the first one or two lines actually match;
// docs that don't follow Jeff's title/byline convention exactly render
// unmodified rather than risk eating real content.
func omitDuplicateTitleByline(blocks []transcription.Block, song db.Song) []transcription.Block {
	i := 0
	if i < len(blocks) && isDuplicateLine(blocks[i], song.Title) {
		i++
	} else {
		return blocks
	}
	if i < len(blocks) && isDuplicateLine(blocks[i], "As performed by: "+song.Artist) {
		i++
	}
	return blocks[i:]
}

func isDuplicateLine(b transcription.Block, want string) bool {
	return b.Kind == transcription.TextLine && strings.EqualFold(normalizeWhitespace(b.Text), normalizeWhitespace(want))
}

// normalizeWhitespace collapses runs of whitespace to a single space and
// trims the ends — Jeff's docs aren't consistent about how many spaces
// follow "As performed by:", so an exact-match compare misses real
// duplicates (see TestSongShowOmitsDuplicateBylineWithExtraInternalWhitespace).
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
