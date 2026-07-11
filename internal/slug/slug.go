// Package slug turns song titles/artists into URL-safe, human-readable
// slugs, and resolves collisions between songs that slugify to the same
// string.
package slug

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	apostrophes  = regexp.MustCompile(`['’]`)
	nonAlnumRuns = regexp.MustCompile(`[^a-z0-9]+`)
)

// Slugify lowercases s, strips apostrophes without leaving a gap (so
// "Can't" becomes "cant", not "can-t"), and collapses every other run of
// non-alphanumeric characters into a single hyphen.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = apostrophes.ReplaceAllString(s, "")
	s = nonAlnumRuns.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// ResolveUnique returns baseSlug if it's available (per exists), otherwise
// baseSlug-artistSlug, otherwise that with an incrementing numeric suffix
// — the same fallback chain for both freshly-created and re-synced songs.
func ResolveUnique(baseSlug, artistSlug string, exists func(string) bool) string {
	if !exists(baseSlug) {
		return baseSlug
	}
	withArtist := baseSlug
	if artistSlug != "" {
		withArtist = baseSlug + "-" + artistSlug
	}
	if !exists(withArtist) {
		return withArtist
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", withArtist, i)
		if !exists(candidate) {
			return candidate
		}
	}
}
