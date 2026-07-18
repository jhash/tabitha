package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOfflineSnapshotHandlerServesJSON(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	resetSnapshotCache(t)
	createDigestedSong(t, q, "Africa", "Toto", "africa")

	req := httptest.NewRequest(http.MethodGet, "/offline/snapshot.json", nil)
	rec := httptest.NewRecorder()
	OfflineSnapshotHandler(q)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var songs []OfflineSong
	if err := json.Unmarshal(rec.Body.Bytes(), &songs); err != nil {
		t.Fatalf("unmarshaling response body: %v", err)
	}
	if len(songs) != 1 || songs[0].Slug != "africa" {
		t.Errorf("songs = %+v, want one song with slug %q", songs, "africa")
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
