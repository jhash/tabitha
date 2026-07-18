package web

import (
	"encoding/json"
	"net/http"

	"github.com/jhash/tabitha/internal/db"
)

// OfflineSnapshotHandler serves the background-downloaded JSON export of
// every digested song's rendered page (see offline_snapshot.go) — fetched
// by static/js/offline-sync.js after page load and written straight into
// an IndexedDB object store keyed by slug, so static/sw.js can serve song
// pages offline even if they were never visited while online.
func OfflineSnapshotHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, _, err := GetOfflineSnapshot(r.Context(), q)
		if err != nil {
			http.Error(w, "failed to build offline snapshot", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// Short-lived: the client re-checks /offline/meta before ever
		// re-fetching this, so there's nothing for an intermediate cache to
		// usefully hold onto beyond that.
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	}
}

// OfflineSnapshotMetaHandler serves a small JSON version marker so the
// client can skip re-downloading the (potentially large) snapshot file
// when nothing in the catalog has changed since its last copy.
func OfflineSnapshotMetaHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, version, err := GetOfflineSnapshot(r.Context(), q)
		if err != nil {
			http.Error(w, "failed to build offline snapshot", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_ = json.NewEncoder(w).Encode(OfflineSnapshotMeta{Version: version})
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
