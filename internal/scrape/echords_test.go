package scrape

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/transcription"
)

func TestParseEChordsHTML(t *testing.T) {
	data, err := os.ReadFile("testdata/echords_wonderwall.html")
	if err != nil {
		t.Fatal(err)
	}

	song, err := ParseEChordsHTML(string(data))
	if err != nil {
		t.Fatal(err)
	}

	if song.Title != "Wonderwall" {
		t.Errorf("Title = %q, want %q", song.Title, "Wonderwall")
	}
	if song.Artist != "Oasis" {
		t.Errorf("Artist = %q, want %q", song.Artist, "Oasis")
	}
	if strings.Contains(song.RawText, "[ch]") {
		t.Errorf("RawText still has bracket markup: %q", song.RawText)
	}
	if !strings.Contains(song.RawText, "VERSE 1:") || !strings.Contains(song.RawText, "CHORUS:") {
		t.Errorf("RawText missing converted section headers, got:\n%s", song.RawText)
	}

	var sawChordLyricPair bool
	for _, b := range song.Blocks {
		if b.Kind == transcription.ChordLyricPair {
			sawChordLyricPair = true
			break
		}
	}
	if !sawChordLyricPair {
		t.Errorf("no ChordLyricPair block parsed, got blocks: %+v", song.Blocks)
	}
}

func TestParseEChordsHTML_NoCore(t *testing.T) {
	if _, err := ParseEChordsHTML("<html><body>nope</body></html>"); err == nil {
		t.Error("expected an error for a page with no #core element")
	}
}

func TestEChordsFetch_CloudflareChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html><head><title>Just a moment...</title></head></html>`))
	}))
	defer srv.Close()

	p := &EChords{HTTPClient: srv.Client()}
	_, err := p.Fetch(t.Context(), srv.URL+"/chords/oasis/wonderwall")
	if err == nil || !strings.Contains(err.Error(), "Cloudflare") {
		t.Fatalf("expected ErrBlockedByCloudflare, got: %v", err)
	}
}
