package web

import (
	"bytes"
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

func renderHomeTable(t *testing.T, songs []SongRow, sort string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := homeTable(songs, sort).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestHomeTableListsEachSongWithLinkToShowPage(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), "title")
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

func TestHomeTableHasSortableColumnHeaders(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), "title")
	for _, col := range []string{"title", "artist", "updated", "added", "status", "added_by"} {
		if !strings.Contains(html, "sort="+col) {
			t.Errorf("expected a sort link for column %q, got: %s", col, html)
		}
	}
}

func TestHomeTableShowsStatusAndAddedBy(t *testing.T) {
	html := renderHomeTable(t, sampleSongRows(), "title")
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
	html := renderHomeTable(t, sampleSongRows(), "title")
	if !strings.Contains(html, `<a href="/?sort=artist"`) {
		t.Errorf("expected a real <a href> sort link, got: %s", html)
	}
}
