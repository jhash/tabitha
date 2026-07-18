package web

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/jhash/tabitha/internal/db"
)

// OfflineManifestHandler serves the lightweight catalog manifest (see
// offline_snapshot.go) — every digested song's slug and last-updated time,
// no HTML. static/js/offline-sync.js fetches this on every page load and
// diffs it against IndexedDB to build its one-song-at-a-time download
// queue, rather than ever shipping the whole catalog in one request.
func OfflineManifestHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := GetOfflineManifest(r.Context(), q)
		if err != nil {
			http.Error(w, "failed to build offline manifest", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	}
}

// OfflineSongHandler serves one song's rendered page as JSON — what
// static/js/offline-sync.js's download queue fetches one slug at a time,
// written straight into an IndexedDB object store keyed by slug so
// static/sw.js can serve that song's page offline even if it was never
// visited while online.
func OfflineSongHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		song, err := RenderOfflineSong(r.Context(), q, slug)
		if err != nil || song == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_ = json.NewEncoder(w).Encode(song)
	}
}

// ServiceWorkerHandler serves static/sw.js from the site root rather than
// under /static/ — a service worker can only control paths at or below its
// own script URL, and the whole site needs to be in scope for offline
// navigation fallback to work on every page, not just /static/*.
// no-cache (rather than the long-lived immutable caching /static/* assets
// get) keeps the browser's built-in update check — which compares this
// file byte-for-byte on every navigation — from being skipped due to a
// stale HTTP cache.
func ServiceWorkerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, "static/sw.js")
	}
}
