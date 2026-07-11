package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/jobs"
)

func TestAdminJobsContentRendersJobRows(t *testing.T) {
	page := jobs.JobPage{
		Jobs: []jobs.JobSummary{
			{ID: 5, Kind: "digest_song", State: "completed", Attempt: 1, Detail: "song_id=132"},
		},
	}
	var buf bytes.Buffer
	if err := adminJobsContent(page).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "digest_song") || !strings.Contains(html, "song_id=132") {
		t.Errorf("expected job row to render, got: %s", html)
	}
}

func TestAdminJobsContentShowsNextLinkWhenCursorPresent(t *testing.T) {
	page := jobs.JobPage{NextCursor: "abc123"}
	var buf bytes.Buffer
	if err := adminJobsContent(page).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `href="/admin/jobs?cursor=abc123"`) {
		t.Errorf("expected a Next link carrying the cursor, got: %s", html)
	}
}

func TestAdminJobsContentOmitsNextLinkOnLastPage(t *testing.T) {
	page := jobs.JobPage{Jobs: []jobs.JobSummary{{ID: 1, Kind: "toc_sync", State: "completed"}}}
	var buf bytes.Buffer
	if err := adminJobsContent(page).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()
	if strings.Contains(html, "cursor=") {
		t.Errorf("expected no Next link on the last page, got: %s", html)
	}
}
