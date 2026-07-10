package db

import (
	"context"
	"testing"
)

func TestUpsertSongFromTOCCreatesNewSong(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, UpsertSongFromTOCParams{
		Title:  "(I Can't Get No) Satisfaction",
		Artist: "Rolling Stones, the",
		Status: "Done",
	})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if song.Title != "(I Can't Get No) Satisfaction" {
		t.Errorf("Title = %q", song.Title)
	}
	if song.Status != "Done" {
		t.Errorf("Status = %q, want Done", song.Status)
	}
}

func TestUpsertSongFromTOCDedupsByNormalizedTitleArtist(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	first, err := q.UpsertSongFromTOC(ctx, UpsertSongFromTOCParams{
		Title:  "Yesterday",
		Artist: "The Beatles",
		Status: "Done",
	})
	if err != nil {
		t.Fatalf("first UpsertSongFromTOC() error = %v", err)
	}

	// Same song, different case and an updated status — must update the
	// existing row, not create a title/artist clash.
	second, err := q.UpsertSongFromTOC(ctx, UpsertSongFromTOCParams{
		Title:  "YESTERDAY",
		Artist: "the beatles",
		Status: "In Progress",
	})
	if err != nil {
		t.Fatalf("second UpsertSongFromTOC() error = %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("dedup failed: got a new song ID %d, want the same ID %d as the first upsert", second.ID, first.ID)
	}
	if second.Status != "In Progress" {
		t.Errorf("Status = %q, want the updated value In Progress", second.Status)
	}

	all, err := q.ListSongsByTitle(ctx)
	if err != nil {
		t.Fatalf("ListSongsByTitle() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("got %d songs, want exactly 1 (no duplicate row)", len(all))
	}
}

func TestGetSongByIDRoundTrips(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	created, err := q.UpsertSongFromTOC(ctx, UpsertSongFromTOCParams{
		Title:  "Africa",
		Artist: "Toto",
		Genre:  "Classic Rock",
		Decade: "1980s",
	})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	fetched, err := q.GetSongByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSongByID() error = %v", err)
	}
	if fetched.Title != "Africa" || fetched.Genre != "Classic Rock" {
		t.Errorf("got %+v", fetched)
	}
}

func TestPromoteToSuperadminUpdatesRole(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, FindOrCreateUserParams{
		Email: "jhash147@gmail.com",
		Name:  "Jake",
	})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	if user.Role != UserRoleUser {
		t.Fatalf("new user Role = %v, want %v", user.Role, UserRoleUser)
	}

	rows, err := q.PromoteToSuperadmin(ctx, "JHash147@gmail.com") // case-insensitive email match
	if err != nil {
		t.Fatalf("PromoteToSuperadmin() error = %v", err)
	}
	if rows != 1 {
		t.Fatalf("PromoteToSuperadmin() affected %d rows, want 1", rows)
	}

	promoted, err := q.GetUserByEmail(ctx, "jhash147@gmail.com")
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if promoted.Role != UserRoleSuperadmin {
		t.Errorf("Role = %v, want %v", promoted.Role, UserRoleSuperadmin)
	}
}

func TestPromoteToSuperadminReturnsZeroRowsForUnknownEmail(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	rows, err := q.PromoteToSuperadmin(ctx, "nobody@example.com")
	if err != nil {
		t.Fatalf("PromoteToSuperadmin() error = %v", err)
	}
	if rows != 0 {
		t.Errorf("affected %d rows, want 0 for an email with no matching user", rows)
	}
}
