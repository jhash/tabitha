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

		blocks, key, hasVersion, err := currentVersionBlocks(r.Context(), q, song)
		if err != nil {
			http.Error(w, "failed to load transcription", http.StatusInternalServerError)
			return
		}

		viewer, _ := auth.UserFromContext(r.Context())
		viewerIsSuperadmin := viewer.Role == db.UserRoleSuperadmin

		description := fmt.Sprintf("%s, as performed by %s", song.Title, song.Artist)
		page := Page(song.Title, description, nil, viewerIsSuperadmin, songShowContent(song, blocks, key, hasVersion, viewerIsSuperadmin))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Render(w)
	}
}

func currentVersionBlocks(ctx context.Context, q *db.Queries, song db.Song) ([]transcription.Block, string, bool, error) {
	if song.CurrentVersionID == nil {
		return nil, "", false, nil
	}

	row, err := q.GetSongCurrentVersion(ctx, song.ID)
	if err != nil {
		return nil, "", false, err
	}

	blocks, err := transcription.UnmarshalDocument(row.TranscriptionVersion.Content)
	if err != nil {
		return nil, "", false, err
	}
	return blocks, deref(row.TranscriptionVersion.Key), true, nil
}

func songShowContent(song db.Song, blocks []transcription.Block, key string, hasVersion, viewerIsSuperadmin bool) g.Node {
	return Div(
		H1(g.Text(song.Title)),
		g.If(song.Artist != "", P(Class("byline"), g.Text("As performed by "+song.Artist))),
		g.If(key != "", P(Class("key"), g.Text("Key: "), B(g.Text(strings.ToUpper(key))))),
		g.If(viewerIsSuperadmin,
			P(Class("admin-affordance"), A(Href(fmt.Sprintf("/songs/%d/edit", song.ID)), g.Text("Edit"))),
		),
		g.If(!hasVersion,
			P(Class("no-content"), g.Text("This song hasn't been digested from Jeff's Google Doc yet.")),
		),
		g.If(hasVersion,
			renderTranscriptionHTML(omitDuplicateHeaderLines(blocks, song)),
		),
	)
}

// omitDuplicateHeaderLines drops the digested doc's own leading title,
// "As performed by: <artist>", and "Key: ..." lines when the page's own
// chrome already shows the same information (H1/byline, and the Key
// line above, sourced from the database rather than re-parsed from raw
// text here) — otherwise it all renders twice (see the "Downtown" bug
// report). Only trims when the lines actually match the expected
// pattern; docs that don't follow Jeff's convention exactly render
// unmodified rather than risk eating real content.
func omitDuplicateHeaderLines(blocks []transcription.Block, song db.Song) []transcription.Block {
	i := 0
	if i < len(blocks) && isDuplicateLine(blocks[i], song.Title) {
		i++
	} else {
		return blocks
	}
	if i < len(blocks) && isDuplicateArtistLine(blocks[i], song.Artist) {
		i++
	}
	if i < len(blocks) && blocks[i].Kind == transcription.TextLine && strings.HasPrefix(strings.ToLower(strings.TrimSpace(blocks[i].Text)), "key:") {
		i++
	}
	return blocks[i:]
}

func isDuplicateLine(b transcription.Block, want string) bool {
	return b.Kind == transcription.TextLine && strings.EqualFold(normalizeWhitespace(b.Text), normalizeWhitespace(want))
}

// isDuplicateArtistLine checks a doc's "As performed by: <artist>" line
// against the stored artist, tolerating Jeff's TOC "Sortname, The" form vs
// a doc's own natural-order byline ("Rolling Stones, the" vs "The Rolling
// Stones") — same artist, different word order.
func isDuplicateArtistLine(b transcription.Block, artist string) bool {
	if b.Kind != transcription.TextLine {
		return false
	}
	const prefix = "as performed by:"
	text := normalizeWhitespace(b.Text)
	lower := strings.ToLower(text)
	if !strings.HasPrefix(lower, prefix) {
		return false
	}
	docArtist := strings.TrimSpace(text[len(prefix):])
	return normalizeArtistName(docArtist) == normalizeArtistName(artist)
}

// normalizeArtistName strips a leading "The " or trailing ", the" (either
// convention for the same artist) and lowercases, so sort-friendly and
// natural-order forms compare equal.
func normalizeArtistName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimSuffix(n, ", the")
	n = strings.TrimPrefix(n, "the ")
	return strings.TrimSpace(n)
}

// normalizeWhitespace collapses runs of whitespace to a single space and
// trims the ends — Jeff's docs aren't consistent about how many spaces
// follow "As performed by:", so an exact-match compare misses real
// duplicates (see TestSongShowOmitsDuplicateBylineWithExtraInternalWhitespace).
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
