package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

func TestAdminSetSongStatusHandlerUpdatesStatus(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Status: "Pending"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	token, _ := superadminSession(t, q)
	rec := doAdminFormRequest(r, http.MethodPost, fmt.Sprintf("/admin/songs/%d/status", song.ID), token, url.Values{"status": {"Done"}})

	if rec.Code < 200 || rec.Code >= 400 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	updated, err := q.GetSongByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetSongByID() error = %v", err)
	}
	if updated.Status != "Done" {
		t.Errorf("Status = %q, want %q", updated.Status, "Done")
	}
}

func TestAdminSetSongStatusHandlerRequiresSuperadmin(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Status: "Pending"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	form := url.Values{"status": {"Done"}}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/songs/%d/status", song.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("anonymous request status = %d, want 404", rec.Code)
	}
}

func TestAdminBulkSetSongStatusHandlerUpdatesAllSelected(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	first, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Status: "Pending"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	second, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Yesterday", Status: "Pending"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	unrelated, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Untouched", Status: "Pending"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	token, _ := superadminSession(t, q)
	form := url.Values{
		"status": {"Done"},
		"ids":    {fmt.Sprintf("%d", first.ID), fmt.Sprintf("%d", second.ID)},
	}
	rec := doAdminFormRequest(r, http.MethodPost, "/admin/songs/bulk-status", token, form)

	if rec.Code < 200 || rec.Code >= 400 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	for _, id := range []int64{first.ID, second.ID} {
		s, err := q.GetSongByID(ctx, id)
		if err != nil {
			t.Fatalf("GetSongByID(%d) error = %v", id, err)
		}
		if s.Status != "Done" {
			t.Errorf("song %d Status = %q, want %q", id, s.Status, "Done")
		}
	}

	stillPending, err := q.GetSongByID(ctx, unrelated.ID)
	if err != nil {
		t.Fatalf("GetSongByID() error = %v", err)
	}
	if stillPending.Status != "Pending" {
		t.Errorf("unrelated song Status = %q, want unchanged %q", stillPending.Status, "Pending")
	}
}
