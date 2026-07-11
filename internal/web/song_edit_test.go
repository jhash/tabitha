package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/db"
)

func renderSongEdit(t *testing.T, song db.Song, hasVersion bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := songEditContent(song, hasVersion).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return buf.String()
}

func TestSongEditContentMountsProseMirrorEditorIsland(t *testing.T) {
	html := renderSongEdit(t, db.Song{ID: 7, Title: "Africa"}, true)

	if !strings.Contains(html, `id="tabitha-editor-root"`) {
		t.Errorf("expected the ProseMirror editor mount point, got: %s", html)
	}
	if !strings.Contains(html, `data-song-id="7"`) {
		t.Errorf("expected the mount point to carry the song's id, got: %s", html)
	}
	if !strings.Contains(html, `/static/js/editor.js`) {
		t.Errorf("expected the editor bundle to be linked, got: %s", html)
	}
	if !strings.Contains(html, `/static/css/editor.css`) {
		t.Errorf("expected the editor stylesheet to be linked, got: %s", html)
	}
}

func TestSongEditContentShowsPlaceholderWhenNotYetDigested(t *testing.T) {
	html := renderSongEdit(t, db.Song{ID: 7, Title: "Africa"}, false)
	if !strings.Contains(html, "hasn&#39;t been digested") {
		t.Errorf("expected a not-yet-digested message, got: %s", html)
	}
}

func TestSongEditContentLinksBackToTheSongPage(t *testing.T) {
	html := renderSongEdit(t, db.Song{ID: 7, Title: "Africa"}, false)
	if !strings.Contains(html, `href="/songs/7"`) {
		t.Errorf("expected a link back to /songs/7, got: %s", html)
	}
}
