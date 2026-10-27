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

func TestHomeRouteStickyHeaderHoldsSearchFieldBoundToFilterForm(t *testing.T) {
	q := setupTestQueries(t)
	r := NewRouter(config.Config{}, q, nil)

	doc := getDoc(t, r, httptest.NewRequest(http.MethodGet, "/?search=toto", nil))

	header := doc.Find(".songs-header-row")
	if header.Length() != 1 {
		t.Fatalf("found %d .songs-header-row elements, want 1", header.Length())
	}
	search := header.Find(`.song-search-field input[type="search"][name="search"]`)
	if search.Length() != 1 {
		t.Fatalf("found %d search inputs in the sticky header, want 1", search.Length())
	}
	if val, _ := search.Attr("value"); val != "toto" {
		t.Errorf("search input value = %q, want %q", val, "toto")
	}
	if label, _ := search.Attr("aria-label"); label != "Search songs" {
		t.Errorf("search input aria-label = %q, want %q", label, "Search songs")
	}

	formID, _ := search.Attr("form")
	form := doc.Find("form#" + formID)
	if formID == "" || form.Length() != 1 {
		t.Fatalf("search input form attr = %q, want it to name the one filter form", formID)
	}
	if n := doc.Find(`input[name="search"]`).Length(); n != 1 {
		t.Errorf("found %d search inputs on the page, want only the sticky header's", n)
	}
	for _, name := range []string{"status", "added_by", "digested"} {
		if form.Find(`select[name="`+name+`"]`).Length() != 1 {
			t.Errorf("filter form is missing its %q select", name)
		}
	}
	if trigger, _ := form.Attr("hx-trigger"); !strings.Contains(trigger, "keyup changed delay:300ms from:#song-search") {
		t.Errorf("filter form hx-trigger = %q, want it to listen for keyup on #song-search", trigger)
	}

	href, _ := header.Find(".song-search-field svg use").Attr("href")
	if !strings.HasPrefix(href, "/static/icons.svg") || !strings.HasSuffix(href, "#search-icon") {
		t.Errorf("search icon href = %q, want the icons.svg sprite's #search-icon", href)
	}
}

func TestHomeRouteStickyHeaderPlusButtonOnlyRendersForSuperadmin(t *testing.T) {
	q := setupTestQueries(t)
	r := NewRouter(config.Config{}, q, nil)

	anonDoc := getDoc(t, r, httptest.NewRequest(http.MethodGet, "/", nil))
	if n := anonDoc.Find(".songs-header-row .new-song-icon-button").Length(); n != 0 {
		t.Errorf("anonymous viewer sees %d new-song buttons in the header, want 0", n)
	}

	token, _ := superadminSession(t, q)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	adminDoc := getDoc(t, r, req)

	button := adminDoc.Find(".songs-header-row .new-song-icon-button")
	if button.Length() != 1 {
		t.Fatalf("superadmin sees %d new-song buttons in the header, want 1", button.Length())
	}
	if href, _ := button.Attr("href"); href != "/songs/new" {
		t.Errorf("new-song button href = %q, want /songs/new", href)
	}
	if label, _ := button.Attr("aria-label"); label != "New song" {
		t.Errorf("new-song button aria-label = %q, want %q", label, "New song")
	}
	if href, _ := button.Find("svg use").Attr("href"); !strings.HasSuffix(href, "#plus-icon") {
		t.Errorf("new-song button icon href = %q, want the sprite's #plus-icon", href)
	}
}
