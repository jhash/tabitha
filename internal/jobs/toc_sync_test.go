package jobs

import (
	"strings"
	"testing"
)

const tocFixtureCSV = `TITLE,ARTIST,GENRE,FILM/SHOW/ALBUM,DECADE,BOB,STATUS,SCRAPE LINK,Notes,TRANSPOSE
(I Can't Get No) Satisfaction,"Rolling Stones, the",Classic Rock,,1960s,Cruise,Done,https://tabs.ultimate-guitar.com/tab/the-rolling-stones/i-cant-get-no-satisfaction-chords-1046719,,
A Whole New World,Lea Salonga & Brad Kane,Disney/Musical,Aladdin,1990s,,Done,http://www.guitaretab.com/w/walt-disney/186832.html,,
`

func TestParseTOCCSVMapsColumnsByHeaderName(t *testing.T) {
	rows, err := parseTOCCSV(strings.NewReader(tocFixtureCSV))
	if err != nil {
		t.Fatalf("parseTOCCSV() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	first := rows[0]
	if first.Title != "(I Can't Get No) Satisfaction" {
		t.Errorf("Title = %q", first.Title)
	}
	if first.Artist != "Rolling Stones, the" {
		t.Errorf("Artist = %q", first.Artist)
	}
	if first.Genre != "Classic Rock" {
		t.Errorf("Genre = %q", first.Genre)
	}
	if first.Decade != "1960s" {
		t.Errorf("Decade = %q", first.Decade)
	}
	if first.BobTag != "Cruise" {
		t.Errorf("BobTag = %q", first.BobTag)
	}
	if first.Status != "Done" {
		t.Errorf("Status = %q", first.Status)
	}
	if first.SourceUrl != "https://tabs.ultimate-guitar.com/tab/the-rolling-stones/i-cant-get-no-satisfaction-chords-1046719" {
		t.Errorf("SourceUrl = %q", first.SourceUrl)
	}

	second := rows[1]
	if second.Title != "A Whole New World" || second.FilmShowAlbum != "Aladdin" {
		t.Errorf("got %+v", second)
	}
}

func TestParseTOCCSVSkipsBlankRows(t *testing.T) {
	csv := "TITLE,ARTIST,GENRE,FILM/SHOW/ALBUM,DECADE,BOB,STATUS,SCRAPE LINK,Notes,TRANSPOSE\n\nA Thousand Miles,Vanessa Carlton,2000s,,2000s,,Done,,,\n"
	rows, err := parseTOCCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseTOCCSV() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 (blank row skipped)", len(rows))
	}
}

func TestParseTOCCSVSkipsRowsWithoutTitle(t *testing.T) {
	csv := "TITLE,ARTIST,GENRE,FILM/SHOW/ALBUM,DECADE,BOB,STATUS,SCRAPE LINK,Notes,TRANSPOSE\n,No Title Here,,,,,,,,\n"
	rows, err := parseTOCCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parseTOCCSV() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("got %d rows, want 0 (row with no title should be skipped)", len(rows))
	}
}
