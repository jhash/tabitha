package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/transcription"
)

// GetSongEditorContentHandler returns a song's current transcription as the
// {"blocks": [...]} JSON shape the ProseMirror editor's blocksToDoc()
// converts into a document. Behind RequireSuperadmin in the router.
func GetSongEditorContentHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		song, err := q.GetSongByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		blocks, _, _, err := currentVersionBlocks(r.Context(), q, song)
		if err != nil {
			http.Error(w, "failed to load transcription", http.StatusInternalServerError)
			return
		}
		body, err := transcription.MarshalDocument(blocks)
		if err != nil {
			http.Error(w, "failed to marshal transcription", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}

// PostSongEditorContentHandler saves the ProseMirror editor's edited blocks
// as a new "manual_edit" transcription_versions row and marks it current.
func PostSongEditorContentHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		song, err := q.GetSongByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		var body struct {
			Blocks []transcription.Block `json:"blocks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		content, err := transcription.MarshalDocument(body.Blocks)
		if err != nil {
			http.Error(w, "failed to marshal transcription", http.StatusInternalServerError)
			return
		}
		rawText := transcription.Render(body.Blocks)

		var createdBy *int64
		if user, ok := auth.UserFromContext(r.Context()); ok {
			createdBy = &user.ID
		}

		version, err := q.CreateTranscriptionVersion(r.Context(), db.CreateTranscriptionVersionParams{
			SongID:    song.ID,
			Kind:      "primary",
			Source:    "manual_edit",
			RawText:   rawText,
			Content:   content,
			CreatedBy: createdBy,
		})
		if err != nil {
			http.Error(w, "failed to save transcription", http.StatusInternalServerError)
			return
		}
		if err := q.ClearCurrentVersionsForSong(r.Context(), song.ID); err != nil {
			http.Error(w, "failed to save transcription", http.StatusInternalServerError)
			return
		}
		if err := q.MarkVersionCurrent(r.Context(), version.ID); err != nil {
			http.Error(w, "failed to save transcription", http.StatusInternalServerError)
			return
		}
		if err := q.SetSongCurrentVersion(r.Context(), db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
			http.Error(w, "failed to save transcription", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}
