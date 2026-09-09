package scrape

import (
	"os"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/transcription"
)

func TestParseUltimateGuitarHTML(t *testing.T) {
	data, err := os.ReadFile("testdata/ultimateguitar_wonderwall.html")
	if err != nil {
		t.Fatal(err)
	}

	song, err := ParseUltimateGuitarHTML(string(data))
	if err != nil {
		t.Fatal(err)
	}

	if song.Title != "Wonderwall" {
		t.Errorf("Title = %q, want %q", song.Title, "Wonderwall")
	}
	if song.Artist != "Oasis" {
		t.Errorf("Artist = %q, want %q", song.Artist, "Oasis")
	}
	if song.Key == "" {
		t.Error("Key is empty, want a tonality")
	}
	if strings.Contains(song.RawText, "[ch]") || strings.Contains(song.RawText, "[tab]") {
		t.Errorf("RawText still has UG markup: %q", song.RawText)
	}
	if !strings.Contains(song.RawText, "VERSE 1:") {
		t.Errorf("RawText missing converted section header, got:\n%s", song.RawText)
	}
	if len(song.Blocks) == 0 {
		t.Error("Blocks is empty")
	}

	var sawChordLyricPair bool
	for _, b := range song.Blocks {
		if b.Kind == transcription.ChordLyricPair {
			sawChordLyricPair = true
			break
		}
	}
	if !sawChordLyricPair {
		t.Error("no ChordLyricPair block parsed out of a real UG chart")
	}
}

func TestParseUltimateGuitarHTML_NoStore(t *testing.T) {
	if _, err := ParseUltimateGuitarHTML("<html><body>nope</body></html>"); err == nil {
		t.Error("expected an error for a page with no js-store data")
	}
}

func TestParseUltimateGuitarHTML_NonChordType(t *testing.T) {
	pageHTML := `<div class="js-store" data-content="{&quot;store&quot;:{&quot;page&quot;:{&quot;data&quot;:{&quot;tab&quot;:{&quot;song_name&quot;:&quot;X&quot;,&quot;artist_name&quot;:&quot;Y&quot;,&quot;type&quot;:&quot;Tab&quot;},&quot;tab_view&quot;:{&quot;wiki_tab&quot;:{&quot;content&quot;:&quot;e|---|&quot;}}}}}}"></div>`
	if _, err := ParseUltimateGuitarHTML(pageHTML); err == nil {
		t.Error("expected an error for a non-Chords page type")
	}
}
