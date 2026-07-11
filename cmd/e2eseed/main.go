// Command e2eseed seeds a single, predictable test song into whatever
// DATABASE_URL points at, for the Playwright e2e suite (see e2e/) to drive
// against. Idempotent: safe to run before every e2e run.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{
		Title:  "E2E Test Song",
		Artist: "E2E Test Artist",
		Status: "Done",
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "e2e-test-song"}); err != nil {
		log.Fatal(err)
	}

	blocks := []transcription.Block{
		{Kind: transcription.TextLine, Text: "E2E Test Song"},
		{Kind: transcription.TextLine, Text: "As performed by: E2E Test Artist"},
		{Kind: transcription.SectionHeader, Text: "VERSE 1:"},
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "Cm"}, {Text: "Oh baby, baby, how "},
				{Chord: "G/B"}, {Text: "was I supposed to know    That "},
				{Chord: "Eb"}, {Text: "something wasn't "},
				{Chord: "Fm"}, {Text: "right here"},
			},
		},
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Chord: "Cm"}, {Text: "Short line"},
			},
		},
		// Real production content (from "(Everything I Do) I Do It for
		// You") that wraps within the editor's content width — used to
		// catch the gap-between-separate-lines-vs-wrapped-rows spacing
		// bug that the short synthetic lines above don't reproduce.
		{Kind: transcription.SectionHeader, Text: "CHORUS 1:"},
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Text: "Don't "}, {Chord: "Ebm"}, {Text: "tell me it's not worth t"},
				{Chord: "Db"}, {Text: "ryin' fo"}, {Chord: "Ebm"},
				{Text: "r     You can't tell me it's not worth "},
				{Chord: "Db"}, {Text: "dyin' f"}, {Chord: "Ebm"}, {Text: "or"},
			},
		},
		{
			Kind: transcription.ChordLyricPair,
			Tokens: []transcription.Token{
				{Text: "You know it's "}, {Chord: "Db"}, {Text: "true  Everything I "},
				{Chord: "Ab"}, {Text: "do, I do it for y"}, {Chord: "Db"}, {Text: "ou"},
			},
		},
	}
	content, err := transcription.MarshalDocument(blocks)
	if err != nil {
		log.Fatal(err)
	}
	rawText := transcription.Render(blocks)

	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID:  song.ID,
		Kind:    "primary",
		Source:  "manual_edit",
		RawText: rawText,
		Content: content,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		log.Fatal(err)
	}

	// Written for the Playwright suite to read (the /edit route only
	// accepts numeric song IDs, not slugs — see internal/web/song_edit.go).
	if err := os.WriteFile("e2e/.song-id", []byte(fmt.Sprintf("%d", song.ID)), 0o644); err != nil {
		log.Fatal(err)
	}

	log.Printf("seeded song id=%d slug=%s", song.ID, "e2e-test-song")
}
