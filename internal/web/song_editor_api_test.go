package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

func TestGetSongEditorContentReturnsCurrentVersionBlocksAsJSON(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto", Status: "Done"})
	if err != nil {
		t.Fatalf("seeding song: %v", err)
	}
	content, _ := transcription.MarshalDocument([]transcription.Block{{Kind: transcription.SectionHeader, Text: "CHORUS:"}})
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "google_doc_scrape", RawText: "CHORUS:", Content: content,
	})
	if err != nil {
		t.Fatalf("seeding version: %v", err)
	}
	if err := q.MarkVersionCurrent(ctx, version.ID); err != nil {
		t.Fatalf("marking current: %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("setting current version: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/songs/{id}/editor-content", GetSongEditorContentHandler(q))

	req := httptest.NewRequest(http.MethodGet, "/songs/"+strconv.FormatInt(song.ID, 10)+"/editor-content", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Blocks []transcription.Block `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshaling response: %v", err)
	}
	if len(got.Blocks) != 1 || got.Blocks[0].Text != "CHORUS:" {
		t.Errorf("got blocks %+v, want one SectionHeader block with text CHORUS:", got.Blocks)
	}
}

func TestGetSongEditorContentReturns404ForUnknownSong(t *testing.T) {
	q := setupTestQueries(t)
	r := chi.NewRouter()
	r.Get("/songs/{id}/editor-content", GetSongEditorContentHandler(q))

	req := httptest.NewRequest(http.MethodGet, "/songs/999999/editor-content", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestPostSongEditorContentSavesNewCurrentVersionAsManualEdit(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto", Status: "Done"})
	if err != nil {
		t.Fatalf("seeding song: %v", err)
	}

	r := chi.NewRouter()
	r.Post("/songs/{id}/editor-content", PostSongEditorContentHandler(q))

	body := `{"blocks":[{"kind":"text_line","text":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/songs/"+strconv.FormatInt(song.ID, 10)+"/editor-content", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	updated, err := q.GetSongByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("reloading song: %v", err)
	}
	if updated.CurrentVersionID == nil {
		t.Fatal("expected song to have a current_version_id set after save")
	}
	row, err := q.GetSongCurrentVersion(ctx, song.ID)
	if err != nil {
		t.Fatalf("loading current version: %v", err)
	}
	if row.TranscriptionVersion.Source != "manual_edit" {
		t.Errorf("source = %q, want manual_edit", row.TranscriptionVersion.Source)
	}
	blocks, err := transcription.UnmarshalDocument(row.TranscriptionVersion.Content)
	if err != nil {
		t.Fatalf("unmarshaling content: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Text != "hello" {
		t.Errorf("got blocks %+v, want one text_line block with text hello", blocks)
	}
}

func TestPostSongEditorContentRejectsInvalidJSON(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()
	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto", Status: "Done"})
	if err != nil {
		t.Fatalf("seeding song: %v", err)
	}

	r := chi.NewRouter()
	r.Post("/songs/{id}/editor-content", PostSongEditorContentHandler(q))

	req := httptest.NewRequest(http.MethodPost, "/songs/"+strconv.FormatInt(song.ID, 10)+"/editor-content", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
