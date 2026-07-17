package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// getDoc issues req against r and parses the response body as HTML,
// failing the test on any non-2xx status or parse error.
func getDoc(t *testing.T, r http.Handler, req *http.Request) *goquery.Document {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("%s %s status = %d, want 2xx, body: %s", req.Method, req.URL, rec.Code, rec.Body.String())
	}
	doc, err := goquery.NewDocumentFromReader(rec.Body)
	if err != nil {
		t.Fatalf("parsing response HTML: %v", err)
	}
	return doc
}

func rowTitles(doc *goquery.Document) []string {
	var titles []string
	doc.Find("table tbody tr td:first-of-type a").Each(func(_ int, s *goquery.Selection) {
		titles = append(titles, s.Text())
	})
	return titles
}

// TestHomeRouteSortOrdersRowsDescending checks the actual on-page row
// order after a sort — something a substring check on the body can't
// verify (the title text is present either way regardless of order), but
// goquery's Find walks the table rows in document order.
func TestHomeRouteSortOrdersRowsDescending(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	for _, s := range []db.UpsertSongFromTOCParams{
		{Title: "Africa", Artist: "Toto"},
		{Title: "Yesterday", Artist: "The Beatles"},
		{Title: "Mercy", Artist: "Duffy"},
	} {
		if _, err := q.UpsertSongFromTOC(ctx, s); err != nil {
			t.Fatalf("seeding song: %v", err)
		}
	}

	r := NewRouter(config.Config{}, q, nil)
	doc := getDoc(t, r, httptest.NewRequest(http.MethodGet, "/?sort=title&order=desc&digested=all", nil))

	got := rowTitles(doc)
	want := []string{"Yesterday", "Mercy", "Africa"}
	if len(got) != len(want) {
		t.Fatalf("rowTitles = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rowTitles = %v, want %v", got, want)
			break
		}
	}
}

// TestHomeRouteSortHeaderShowsArrowOnActiveColumnOnly checks that exactly
// one column header carries the sort arrow, and it's the right one —
// distinguishing "column has an arrow" from "column has THE arrow
// belonging to the active sort", which substring matching on the whole
// body can't do since every header link is present regardless of which
// column is active.
func TestHomeRouteSortHeaderShowsArrowOnActiveColumnOnly(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()
	if _, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"}); err != nil {
		t.Fatalf("seeding song: %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	doc := getDoc(t, r, httptest.NewRequest(http.MethodGet, "/?sort=artist&order=asc&digested=all", nil))

	var arrowCount int
	var artistHasArrow bool
	doc.Find("table thead th a").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		if strings.ContainsAny(text, "▴▾") {
			arrowCount++
			if strings.HasPrefix(text, "Artist") {
				artistHasArrow = true
			}
		}
	})

	if arrowCount != 1 {
		t.Errorf("got %d column headers with a sort arrow, want exactly 1", arrowCount)
	}
	if !artistHasArrow {
		t.Error("expected the Artist header to carry the active-sort arrow")
	}
}

// TestHomeRouteRowCheckboxesOnlyRenderForSuperadmin verifies the bulk-edit
// checkbox column is structurally absent (not just visually hidden) for a
// non-superadmin viewer, and present with the right value for one.
func TestHomeRouteRowCheckboxesOnlyRenderForSuperadmin(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()
	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("seeding song: %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)

	anonDoc := getDoc(t, r, httptest.NewRequest(http.MethodGet, "/?digested=all", nil))
	if n := anonDoc.Find(`input[name="ids"]`).Length(); n != 0 {
		t.Errorf("anonymous viewer sees %d bulk-edit checkboxes, want 0", n)
	}

	token, _ := superadminSession(t, q)
	req := httptest.NewRequest(http.MethodGet, "/?digested=all", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	adminDoc := getDoc(t, r, req)

	checkboxes := adminDoc.Find(`input[name="ids"]`)
	if checkboxes.Length() != 1 {
		t.Fatalf("superadmin sees %d bulk-edit checkboxes, want 1", checkboxes.Length())
	}
	if val, _ := checkboxes.Attr("value"); val != strconv.FormatInt(song.ID, 10) {
		t.Errorf("checkbox value = %q, want %q", val, strconv.FormatInt(song.ID, 10))
	}
}
