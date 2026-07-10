package web

import (
	"bytes"
	"strings"
	"testing"
)

func TestAdminToolsContentHasTocSyncTriggerForm(t *testing.T) {
	var buf bytes.Buffer
	if err := adminToolsContent().Render(&buf); err != nil {
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
	if err := adminToolsContent().Render(&buf); err != nil {
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
