package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOfflineSnapshotHandlerServesASQLiteFile(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	resetSnapshotCache(t)
	createDigestedSong(t, q, "Africa", "Toto", "africa")

	req := httptest.NewRequest(http.MethodGet, "/offline/snapshot.sqlite", nil)
	rec := httptest.NewRecorder()
	OfflineSnapshotHandler(q)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-sqlite3" {
		t.Errorf("Content-Type = %q, want application/x-sqlite3", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected a non-empty SQLite file body")
	}
	// SQLite files start with this fixed 16-byte magic header.
	if !strings.HasPrefix(rec.Body.String(), "SQLite format 3\x00") {
		t.Error("expected the response body to start with the SQLite file header")
	}
}

func TestOfflineSnapshotMetaHandlerServesAVersion(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	resetSnapshotCache(t)
	createDigestedSong(t, q, "Africa", "Toto", "africa")

	req := httptest.NewRequest(http.MethodGet, "/offline/meta", nil)
	rec := httptest.NewRecorder()
	OfflineSnapshotMetaHandler(q)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"version"`) {
		t.Errorf("expected a version field in the response, got: %s", rec.Body.String())
	}
}

func TestServiceWorkerHandlerServesJavaScriptFromRoot(t *testing.T) {
	// ServiceWorkerHandler reads "static/sw.js" relative to the process's
	// cwd, same as router.go's file server — true when the real binary
	// runs from the repo root, not true for `go test`'s per-package cwd.
	t.Chdir("../..")

	req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	rec := httptest.NewRecorder()
	ServiceWorkerHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q, want a javascript type", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("Cache-Control = %q, want no-cache so the browser's SW update check isn't skipped", cc)
	}
}
