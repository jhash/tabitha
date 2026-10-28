package web

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

func renderHomeTableDoc(t *testing.T, songs []SongRow, params SongQueryParams) *goquery.Document {
	t.Helper()
	var buf bytes.Buffer
	if err := homeTable(songs, params, nil, false).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	doc, err := goquery.NewDocumentFromReader(&buf)
	if err != nil {
		t.Fatalf("parsing HTML: %v", err)
	}
	return doc
}

func azSampleRows() []SongRow {
	return []SongRow{
		{ID: 1, Title: "1999"},
		{ID: 2, Title: "Africa"},
		{ID: 3, Title: "All Along the Watchtower"},
		{ID: 4, Title: "(I Can't Get No) Satisfaction"},
		{ID: 5, Title: "Imagine"},
	}
}

func TestIndexLetter(t *testing.T) {
	cases := map[string]string{
		"Africa":                        "A",
		"africa":                        "A",
		"(I Can't Get No) Satisfaction": "I",
		"1999":                          "#",
		"Ébène":                         "#",
		"":                              "#",
		"...":                           "#",
	}
	for title, want := range cases {
		if got := indexLetter(title); got != want {
			t.Errorf("indexLetter(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestHomeTableAnchorsFirstRowOfEachLetterWhenSortedByTitle(t *testing.T) {
	doc := renderHomeTableDoc(t, azSampleRows(), SongQueryParams{})

	want := map[string]string{"az-num": "1999", "az-A": "Africa", "az-I": "(I Can't Get No) Satisfaction"}
	for id, title := range want {
		row := doc.Find("tr#" + id)
		if row.Length() != 1 {
			t.Fatalf("expected exactly one row with id %q, got %d", id, row.Length())
		}
		if got := row.Find("td a").First().Text(); got != title {
			t.Errorf("row %q title = %q, want %q", id, got, title)
		}
	}
	if n := doc.Find("tbody tr[id]").Length(); n != 3 {
		t.Errorf("anchored rows = %d, want 3 (one per letter present)", n)
	}
}

func TestHomeTableAzIndexLinksPresentLettersAndDimsEmptyOnes(t *testing.T) {
	doc := renderHomeTableDoc(t, azSampleRows(), SongQueryParams{Sort: "title", Order: "desc"})

	nav := doc.Find("nav.az-index")
	if nav.Length() != 1 {
		t.Fatalf("expected one az-index nav, got %d", nav.Length())
	}
	if n := nav.Find(".az-letter").Length(); n != 27 {
		t.Errorf("letters = %d, want 27 (# plus A–Z)", n)
	}
	if href, _ := nav.Find("a.az-letter").First().Attr("href"); href != "#az-num" {
		t.Errorf("first link href = %q, want #az-num", href)
	}
	if nav.Find(`a[href="#az-I"]`).Length() != 1 {
		t.Error("expected a link to the I anchor")
	}
	if nav.Find("span.az-empty").Length() != 24 {
		t.Errorf("empty letters = %d, want 24", nav.Find("span.az-empty").Length())
	}
	if doc.Find(`script[src^="/static/js/az-index.js"]`).Length() != 1 {
		t.Error("expected the az-index script to be included")
	}
}

func TestHomeTableOmitsAzIndexWhenNotSortedByTitle(t *testing.T) {
	for _, params := range []SongQueryParams{
		{Sort: "artist"},
		{Sort: "played"},
		{Search: "africa"},
	} {
		doc := renderHomeTableDoc(t, azSampleRows(), params)
		if doc.Find("nav.az-index").Length() != 0 {
			t.Errorf("params %+v: expected no az-index", params)
		}
		if doc.Find("tbody tr[id]").Length() != 0 {
			t.Errorf("params %+v: expected no anchored rows", params)
		}
	}
}

func TestHomeTableOmitsAzIndexWhenNoSongs(t *testing.T) {
	doc := renderHomeTableDoc(t, nil, SongQueryParams{})
	if doc.Find("nav.az-index").Length() != 0 {
		t.Error("expected no az-index for an empty list")
	}
}

func TestTimeAgo(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{2 * 24 * time.Hour, "2d ago"},
		{15 * 24 * time.Hour, "2w ago"},
		{95 * 24 * time.Hour, "3mo ago"},
		{800 * 24 * time.Hour, "2y ago"},
	}
	for _, c := range cases {
		if got := timeAgo(now.Add(-c.ago), now); got != c.want {
			t.Errorf("timeAgo(-%v) = %q, want %q", c.ago, got, c.want)
		}
	}
}

func TestHomeTableShowsLastPlayedAsRelativeTime(t *testing.T) {
	played := time.Now().Add(-3 * time.Hour)
	songs := []SongRow{{ID: 1, Title: "Africa", LastPlayedAt: &played}, {ID: 2, Title: "Yesterday"}}
	doc := renderHomeTableDoc(t, songs, SongQueryParams{})

	if !strings.Contains(doc.Find("thead").Text(), "Last Played") {
		t.Error("expected a Last Played header")
	}
	cell := doc.Find("tbody tr").First().Find("td").Last().Find("time")
	if got := cell.Text(); got != "3h ago" {
		t.Errorf("last played text = %q, want 3h ago", got)
	}
	if dt, _ := cell.Attr("datetime"); dt != played.UTC().Format(time.RFC3339) {
		t.Errorf("datetime = %q", dt)
	}
	if text := doc.Find("tbody tr").Last().Find("td").Last().Text(); text != "" {
		t.Errorf("never-played cell = %q, want empty", text)
	}
}

func TestPlayRouteRecordsLastPlayedAndHomeShowsIt(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("seeding song: %v", err)
	}
	blocks := []transcription.Block{{Kind: transcription.SectionHeader, Text: "CHORUS:"}}
	content, err := transcription.MarshalDocument(blocks)
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "manual_edit", RawText: transcription.Render(blocks), Content: content,
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)

	before := getDoc(t, r, httptest.NewRequest("GET", "/", nil))
	if got := before.Find("tbody tr td time").Length(); got != 0 {
		t.Fatalf("expected no last-played time before playing, got %d", got)
	}

	getDoc(t, r, httptest.NewRequest("GET", "/songs/"+strconv.FormatInt(song.ID, 10)+"/play", nil))

	rows, err := ListSongsQuery(ctx, q.DB(), SongQueryParams{HideUndigested: true})
	if err != nil {
		t.Fatalf("ListSongsQuery() error = %v", err)
	}
	if len(rows) != 1 || rows[0].LastPlayedAt == nil {
		t.Fatalf("rows = %+v, want Africa with LastPlayedAt set", rows)
	}

	after := getDoc(t, r, httptest.NewRequest("GET", "/", nil))
	if got := after.Find("tbody tr td time").Text(); got != "just now" {
		t.Errorf("last played after playing = %q, want just now", got)
	}
}
