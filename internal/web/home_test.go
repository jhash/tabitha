package web

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
	"time"
)

func sampleSongRows() []SongRow {
	return []SongRow{
		{
			ID: 2, Title: "Africa", Artist: "Toto", Status: "Done",
			AddedByName: "Jake", AddedByEmail: "jhash147@gmail.com",
			CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: 1, Title: "(I Can't Get No) Satisfaction", Artist: "Rolling Stones, the", Status: "Quality Check",
			// No AddedByName on purpose — a user who hasn't set a display
			// name yet should still show up by email, not blank.
			AddedByEmail: "noname@example.com",
			CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		},
	}
}

func renderHomeTable(t *testing.T, songs []SongRow, params SongQueryParams) string {
	t.Helper()
	var buf bytes.Buffer
	if err := homeTable(songs, params).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestHomeTableListsEachSongWithLinkToShowPage(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), SongQueryParams{Sort: "title"})
	if !strings.Contains(html, `href="/songs/2"`) {
		t.Error("expected a link to /songs/2 for Africa")
	}
	if !strings.Contains(html, `href="/songs/1"`) {
		t.Error("expected a link to /songs/1 for Satisfaction")
	}
	if !strings.Contains(html, "Africa") || !strings.Contains(html, "Satisfaction") {
		t.Error("expected both song titles to render")
	}
}

func TestHomeTableLinksBySlugWhenSet(t *testing.T) {
	songs := []SongRow{{ID: 2, Title: "Africa", Artist: "Toto", Slug: "africa"}}
	html := renderHomeTable(t, songs, SongQueryParams{Sort: "title"})
	if !strings.Contains(html, `href="/songs/africa"`) {
		t.Errorf("expected a link to /songs/africa, got: %s", html)
	}
	if strings.Contains(html, `href="/songs/2"`) {
		t.Errorf("expected no ID-based link once a slug is set, got: %s", html)
	}
}

func TestHomeTableHasSortableColumnHeaders(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), SongQueryParams{Sort: "title"})
	for _, col := range []string{"title", "artist", "updated", "added", "status", "added_by"} {
		if !strings.Contains(html, "sort="+col) {
			t.Errorf("expected a sort link for column %q, got: %s", col, html)
		}
	}
}

func TestHomeTableShowsStatusAndAddedBy(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), SongQueryParams{Sort: "title"})
	if !strings.Contains(html, "Done") || !strings.Contains(html, "Quality Check") {
		t.Error("expected both status values to render")
	}
	// Africa has a display name set — prefer it over the raw email.
	if !strings.Contains(html, "Jake") {
		t.Error("expected added-by display name to render when one is set")
	}
	// Satisfaction's adder has no display name — fall back to email rather
	// than rendering a blank cell.
	if !strings.Contains(html, "noname@example.com") {
		t.Error("expected added-by email to render as a fallback when no display name is set")
	}
}

func TestHomeTableWorksWithoutJavaScriptAsPlainLinks(t *testing.T) {
	// Sort headers must be real <a href> links (not just hx-get-only spans)
	// so the page still works with JS disabled, per the SSR-first goal.
	html := renderHomeTable(t, sampleSongRows(), SongQueryParams{Sort: "title"})
	if !strings.Contains(html, `<a href="/?`) || !strings.Contains(html, "sort=artist") {
		t.Errorf("expected a real <a href> sort link, got: %s", html)
	}
}

func TestHomeTableActiveSortColumnShowsAscendingArrowByDefault(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), SongQueryParams{Sort: "title", Order: "asc"})
	if !strings.Contains(html, "Title ▴") {
		t.Errorf("expected an ascending arrow on the active Title column, got: %s", html)
	}
}

func TestHomeTableActiveSortColumnTogglesToDescendingLinkWhenAlreadyAscending(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), SongQueryParams{Sort: "status", Order: "asc"})
	if !strings.Contains(html, "Status ▴") {
		t.Errorf("expected ascending arrow on active Status column, got: %s", html)
	}
	if !strings.Contains(html, "order=desc&amp;sort=status") {
		t.Errorf("expected Status header's link to toggle to order=desc, got: %s", html)
	}
}

func TestParseSongQueryParamsDefaultsToHideUndigested(t *testing.T) {
	p := parseSongQueryParams(url.Values{})
	if !p.HideUndigested {
		t.Error("HideUndigested = false on a fresh page load, want true (default on)")
	}
}

func TestParseSongQueryParamsShowsAllWhenDigestedIsAll(t *testing.T) {
	p := parseSongQueryParams(url.Values{"digested": {"all"}})
	if p.HideUndigested {
		t.Error("HideUndigested = true with digested=all, want false")
	}
}

func TestSearchAndFilterFormDigestedSelectDefaultsToHideSelected(t *testing.T) {
	var buf bytes.Buffer
	if err := searchAndFilterForm(SongQueryParams{HideUndigested: true}, nil, nil).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `name="digested"`) {
		t.Fatalf("expected a digested filter select, got: %s", html)
	}
	if !strings.Contains(html, `value="hide" selected`) {
		t.Errorf(`expected the "hide" option selected by default, got: %s`, html)
	}
}

func TestSearchAndFilterFormDigestedSelectShowsAllSelectedWhenNotHiding(t *testing.T) {
	var buf bytes.Buffer
	if err := searchAndFilterForm(SongQueryParams{HideUndigested: false}, nil, nil).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `value="all" selected`) {
		t.Errorf(`expected the "all" option selected, got: %s`, html)
	}
}

func TestHomeTableActiveSortColumnTogglesToAscendingLinkWhenAlreadyDescending(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), SongQueryParams{Sort: "status", Order: "desc"})
	if !strings.Contains(html, "Status ▾") {
		t.Errorf("expected descending arrow on active Status column, got: %s", html)
	}
	if !strings.Contains(html, "order=asc&amp;sort=status") {
		t.Errorf("expected Status header's link to toggle back to order=asc, got: %s", html)
	}
}
