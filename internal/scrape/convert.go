package scrape

import (
	"regexp"
	"strings"

	"github.com/jhash/tabitha/internal/transcription"
)

var (
	chTagRe       = regexp.MustCompile(`\[/?ch\]`)
	tabTagRe      = regexp.MustCompile(`\[/?tab\]`)
	bracketLineRe = regexp.MustCompile(`^\[([A-Za-z0-9 '()/-]+)\]$`)
)

// convertBracketMarkup turns Ultimate Guitar's (and, after normalization,
// e-chords') bracket-tagged chord markup into tabitha's own plain
// chord-over-lyric text:
//
//   - "[ch]Em[/ch]" -> "Em": UG wraps every chord symbol in [ch] tags so
//     its own renderer can highlight/link it; the column position is
//     already correct in the surrounding plain text, so the tags are
//     pure noise once stripped.
//   - "[tab]...[/tab]": UG groups a chord line + its lyric line (or an
//     instrumental chord-only line) inside one [tab] block. tabitha's
//     own parser pairs chord/lyric lines by shape (see
//     transcription.Parse), not by an explicit wrapper, so these are
//     dropped outright too.
//   - A whole line wrapped in one bracket pair with no [ch]/[/tab]
//     remnants, e.g. "[Verse 1]" or "[Chorus]", is a section label —
//     UG's only convention for one. Converted to "VERSE 1:"/"CHORUS:" to
//     match transcription's own sectionHeaderRe convention.
func convertBracketMarkup(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = chTagRe.ReplaceAllString(content, "")
	content = tabTagRe.ReplaceAllString(content, "")

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if m := bracketLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			lines[i] = strings.ToUpper(m[1]) + ":"
		}
	}
	return strings.Join(lines, "\n")
}

// fromPlainText parses rawText (already in tabitha's chord-over-lyric
// format) into a Song.
func fromPlainText(title, artist, key, rawText string) Song {
	return Song{
		Title:   strings.TrimSpace(title),
		Artist:  strings.TrimSpace(artist),
		Key:     strings.TrimSpace(key),
		RawText: rawText,
		Blocks:  transcription.Parse(rawText),
	}
}
