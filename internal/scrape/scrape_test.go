package scrape

import "testing"

func TestForURL(t *testing.T) {
	cases := []struct {
		url      string
		wantName string
		wantErr  bool
	}{
		{"https://tabs.ultimate-guitar.com/tab/oasis/wonderwall-chords-27596", "ultimate_guitar_scrape", false},
		{"https://www.ultimate-guitar.com/tab/oasis/wonderwall-chords-27596", "ultimate_guitar_scrape", false},
		{"https://www.e-chords.com/chords/oasis/wonderwall", "echords_scrape", false},
		{"https://example.com/whatever", "", true},
		{"not a url\x7f", "", true},
	}
	for _, c := range cases {
		p, err := ForURL(c.url)
		if c.wantErr {
			if err == nil {
				t.Errorf("ForURL(%q): expected error, got none", c.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("ForURL(%q): unexpected error: %v", c.url, err)
			continue
		}
		if p.Name() != c.wantName {
			t.Errorf("ForURL(%q).Name() = %q, want %q", c.url, p.Name(), c.wantName)
		}
	}
}
