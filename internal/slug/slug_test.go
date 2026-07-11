package slug

import "testing"

func TestSlugifyStripsApostrophesWithoutInsertingHyphen(t *testing.T) {
	got := Slugify("(I Can't Get No) Satisfaction")
	want := "i-cant-get-no-satisfaction"
	if got != want {
		t.Errorf("Slugify() = %q, want %q", got, want)
	}
}

func TestSlugifyLowercasesAndHyphenatesSpaces(t *testing.T) {
	got := Slugify("Rocket Man")
	want := "rocket-man"
	if got != want {
		t.Errorf("Slugify() = %q, want %q", got, want)
	}
}

func TestSlugifyCollapsesPunctuationToSingleHyphen(t *testing.T) {
	got := Slugify("Yakety Yak!!  (Don't Talk Back)")
	want := "yakety-yak-dont-talk-back"
	if got != want {
		t.Errorf("Slugify() = %q, want %q", got, want)
	}
}

func TestResolveUniqueSlugReturnsBaseWhenAvailable(t *testing.T) {
	got := ResolveUnique("rocket-man", "elton-john", func(string) bool { return false })
	if got != "rocket-man" {
		t.Errorf("ResolveUnique() = %q, want %q", got, "rocket-man")
	}
}

func TestResolveUniqueSlugAppendsArtistSlugOnCollision(t *testing.T) {
	taken := map[string]bool{"rocket-man": true}
	got := ResolveUnique("rocket-man", "elton-john", func(s string) bool { return taken[s] })
	if got != "rocket-man-elton-john" {
		t.Errorf("ResolveUnique() = %q, want %q", got, "rocket-man-elton-john")
	}
}

func TestResolveUniqueSlugFallsBackToNumericSuffixWhenArtistSlugAlsoTaken(t *testing.T) {
	taken := map[string]bool{"rocket-man": true, "rocket-man-elton-john": true}
	got := ResolveUnique("rocket-man", "elton-john", func(s string) bool { return taken[s] })
	if got != "rocket-man-elton-john-2" {
		t.Errorf("ResolveUnique() = %q, want %q", got, "rocket-man-elton-john-2")
	}
}
