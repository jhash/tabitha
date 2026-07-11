package web

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/jhash/tabitha/internal/cloudflare"
	"github.com/jhash/tabitha/internal/db"
)

// purgeHomePage best-effort invalidates the cached home page after a
// status change (its table shows status directly) — never fails the
// request over a purge hiccup, just logs it. cf may be nil.
func purgeHomePage(ctx context.Context, appURL string, cf *cloudflare.Client) {
	if cf == nil || !cf.Configured() {
		return
	}
	if err := cf.PurgeURLs(ctx, []string{appURL + "/"}); err != nil {
		log.Printf("admin_songs: cloudflare purge failed: %v", err)
	}
}

// AdminSetSongStatusHandler updates one song's status — the home page's
// per-row inline status select (see statusCell in home.go) posts here on
// change.
func AdminSetSongStatusHandler(q *db.Queries, appURL string, cf *cloudflare.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := q.SetSongStatus(r.Context(), db.SetSongStatusParams{ID: id, Status: r.FormValue("status")}); err != nil {
			http.Error(w, "failed to update status", http.StatusInternalServerError)
			return
		}
		purgeHomePage(r.Context(), appURL, cf)
		w.WriteHeader(http.StatusNoContent)
	}
}

// AdminBulkSetSongStatusHandler updates every checked song's status at
// once — the home page's bulk-status bar (see bulkStatusBar in home.go)
// posts here, with hx-include gathering the checked checkboxes and the
// bulk status select.
func AdminBulkSetSongStatusHandler(q *db.Queries, appURL string, cf *cloudflare.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		ids := make([]int64, 0, len(r.Form["ids"]))
		for _, raw := range r.Form["ids"] {
			id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if err != nil {
				http.Error(w, "invalid song id", http.StatusBadRequest)
				return
			}
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := q.SetSongsStatusBulk(r.Context(), db.SetSongsStatusBulkParams{Column1: ids, Status: r.FormValue("status")}); err != nil {
			http.Error(w, "failed to update statuses", http.StatusInternalServerError)
			return
		}
		purgeHomePage(r.Context(), appURL, cf)
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusNoContent)
	}
}
