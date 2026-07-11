package jobs

import (
	"regexp"
	"strings"
)

// keyLineRe matches Jeff's "Key: X, Original Y" convention (see
// music/template-song.txt), tolerating whatever internal spacing his docs
// happen to have. Group 1 is the performance key (what the chords in this
// section are actually written in), group 2 is the original key.
var keyLineRe = regexp.MustCompile(`(?i)^Key:\s*([^,]+?)\s*,\s*Original\s+(.+?)\s*$`)

// parseKeyLine finds Jeff's "Key: X, Original Y" line in a digested doc's
// raw text and returns the performance key and original key. false if no
// such line is found — older or nonstandard docs shouldn't fail digestion
// over missing key metadata.
func parseKeyLine(rawText string) (performanceKey, originalKey string, ok bool) {
	for _, line := range strings.Split(rawText, "\n") {
		m := keyLineRe.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil {
			return m[1], m[2], true
		}
	}
	return "", "", false
}
