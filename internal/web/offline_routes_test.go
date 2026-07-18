package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jhash/tabitha/internal/config"
)

func TestOfflineManifestHandlerServesTheCatalogManifest(t *testing.T) {
	q := setupTestQueries(t)
	createDigestedSong(t, q, "Africa", "Toto", "africa")
	r := NewRouter(config.Config{}, q, nil)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/offline/manifest.json", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var manifest OfflineManifest
	if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("unmarshaling response body: %v", err)
	}
	if len(manifest.Songs) != 1 || manifest.Songs[0].Slug != "africa" {
		t.Errorf("songs = %+v, want one song with slug %q", manifest.Songs, "africa")
	}
}

func TestOfflineSongHandlerServesOneRenderedSong(t *testing.T) {
	q := setupTestQueries(t)
	SetAssetVersions(LoadAssetVersions("../../static"))
	createDigestedSong(t, q, "Africa", "Toto", "africa")
	r := NewRouter(config.Config{}, q, nil)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/offline/songs/africa", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var song OfflineSong
	if err := json.Unmarshal(rec.Body.Bytes(), &song); err != nil {
		t.Fatalf("unmarshaling response body: %v", err)
	}
	if song.Slug != "africa" || song.Title != "Africa" {
		t.Errorf("song = %+v, want slug=africa title=Africa", song)
	}
}

func TestOfflineSongHandlerServes404ForUnknownSlug(t *testing.T) {
	q := setupTestQueries(t)
	r := NewRouter(config.Config{}, q, nil)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/offline/songs/does-not-exist", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
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
