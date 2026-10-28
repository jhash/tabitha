package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRecordSongPlayedUpsertsOneRowPerSong(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	if _, err := q.GetSongLastPlayed(ctx, song.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetSongLastPlayed() before any play error = %v, want ErrNoRows", err)
	}

	if err := q.RecordSongPlayed(ctx, song.ID); err != nil {
		t.Fatalf("first RecordSongPlayed() error = %v", err)
	}
	first, err := q.GetSongLastPlayed(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetSongLastPlayed() error = %v", err)
	}

	if err := q.RecordSongPlayed(ctx, song.ID); err != nil {
		t.Fatalf("second RecordSongPlayed() error = %v", err)
	}
	second, err := q.GetSongLastPlayed(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetSongLastPlayed() error = %v", err)
	}
	if second.Time.Before(first.Time) {
		t.Errorf("second play %v is before first %v", second.Time, first.Time)
	}
}

func TestSongLastPlayedIsDeletedWithItsSong(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if err := q.RecordSongPlayed(ctx, song.ID); err != nil {
		t.Fatalf("RecordSongPlayed() error = %v", err)
	}
	if _, err := q.db.Exec(ctx, "DELETE FROM songs WHERE id = $1", song.ID); err != nil {
		t.Fatalf("deleting song: %v", err)
	}
	if _, err := q.GetSongLastPlayed(ctx, song.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetSongLastPlayed() after delete error = %v, want ErrNoRows", err)
	}
}
