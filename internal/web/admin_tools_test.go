package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/jobs"
)

func TestAdminToolsContentHasTocSyncTriggerForm(t *testing.T) {
	var buf bytes.Buffer
	if err := adminToolsContent(nil).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()

	if !strings.Contains(html, `action="/admin/tools/toc-sync"`) {
		t.Errorf("expected a form posting to /admin/tools/toc-sync, got: %s", html)
	}
	if !strings.Contains(html, `method="post"`) {
		t.Errorf("expected the trigger form to POST, got: %s", html)
	}
}

func TestAdminToolsContentHasDigestByTitleForm(t *testing.T) {
	var buf bytes.Buffer
	if err := adminToolsContent(nil).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()

	if !strings.Contains(html, `action="/admin/tools/digest-song"`) {
		t.Errorf("expected a form posting to /admin/tools/digest-song, got: %s", html)
	}
	if !strings.Contains(html, `name="title"`) {
		t.Errorf("expected a title input, got: %s", html)
	}
}

func TestAdminToolsContentHasDigestBatchForm(t *testing.T) {
	var buf bytes.Buffer
	if err := adminToolsContent(nil).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()

	if !strings.Contains(html, `action="/admin/tools/digest-batch"`) {
		t.Errorf("expected a form posting to /admin/tools/digest-batch, got: %s", html)
	}
	if !strings.Contains(html, `name="limit"`) {
		t.Errorf("expected a limit input, got: %s", html)
	}
}

func TestAdminToolsContentShowsNoJobsMessageWhenEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := adminToolsContent(nil).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()

	if !strings.Contains(html, "No jobs run yet.") {
		t.Errorf("expected empty-state message, got: %s", html)
	}
}

func TestAdminToolsContentRendersJobSummaryRow(t *testing.T) {
	var buf bytes.Buffer
	recent := []jobs.JobSummary{
		{ID: 5, Kind: "digest_song", State: "completed", Attempt: 1, Detail: "song_id=132", LastError: ""},
	}
	if err := adminToolsContent(recent).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()

	for _, want := range []string{"digest_song", "completed", "song_id=132"} {
		if !strings.Contains(html, want) {
			t.Errorf("expected job row to contain %q, got: %s", want, html)
		}
	}
}
