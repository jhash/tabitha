// Command scrape fetches one Ultimate Guitar or e-chords chord-chart URL,
// converts it to tabitha's own chord-over-lyric format, and prints it. With
// --save it also upserts the song and stores the result as a new current
// transcription version (requires DATABASE_URL) — the same effect as the
// scrape_song background job, run synchronously from the command line.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/scrape"
	"github.com/jhash/tabitha/internal/transcription"
)

func main() {
	save := flag.Bool("save", false, "create/update the song and store a new transcription version in the database (requires DATABASE_URL)")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: scrape [--save] <ultimate-guitar-or-e-chords url>")
		os.Exit(2)
	}
	rawURL := args[0]

	provider, err := scrape.ForURL(rawURL)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	song, err := provider.Fetch(ctx, rawURL)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("# %s — %s (via %s)\n\n", song.Title, song.Artist, provider.Name())
	fmt.Println(transcription.Render(song.Blocks))

	if !*save {
		return
	}

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	q := db.New(pool)

	dbSong, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{
		Title:     song.Title,
		Artist:    song.Artist,
		Status:    "Scraped",
		SourceUrl: rawURL,
	})
	if err != nil {
		log.Fatal(err)
	}

	content, err := transcription.MarshalDocument(song.Blocks)
	if err != nil {
		log.Fatal(err)
	}

	var key *string
	if song.Key != "" {
		key = &song.Key
	}

	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID:  dbSong.ID,
		Kind:    "primary",
		Source:  provider.Name(),
		RawText: song.RawText,
		Content: content,
		Key:     key,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := q.ClearCurrentVersionsForSong(ctx, dbSong.ID); err != nil {
		log.Fatal(err)
	}
	if err := q.MarkVersionCurrent(ctx, version.ID); err != nil {
		log.Fatal(err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: dbSong.ID, CurrentVersionID: &version.ID}); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("saved song id=%d\n", dbSong.ID)
}
