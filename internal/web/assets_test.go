package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAssetVersionsHashesFileContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "style.css"), []byte("body { color: red; }"), 0644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	v := loadAssetVersion(filepath.Join(dir, "style.css"))
	if v == "" {
		t.Fatal("loadAssetVersion() = \"\", want a non-empty hash")
	}
}

func TestLoadAssetVersionChangesWhenContentChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "style.css")
	if err := os.WriteFile(path, []byte("body { color: red; }"), 0644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	before := loadAssetVersion(path)

	if err := os.WriteFile(path, []byte("body { color: blue; }"), 0644); err != nil {
		t.Fatalf("rewriting fixture: %v", err)
	}
	after := loadAssetVersion(path)

	if before == after {
		t.Errorf("hash unchanged after content changed: %q", before)
	}
}

func TestLoadAssetVersionReturnsEmptyForMissingFile(t *testing.T) {
	if v := loadAssetVersion("/nonexistent/path/style.css"); v != "" {
		t.Errorf("loadAssetVersion() for missing file = %q, want empty string", v)
	}
}

func TestLoadAssetVersionsHashesEditorJSAndCSS(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"js", "css"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			t.Fatalf("making %s dir: %v", sub, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "js", "editor.js"), []byte("console.log(1)"), 0644); err != nil {
		t.Fatalf("writing editor.js fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "css", "editor.css"), []byte(".x{}"), 0644); err != nil {
		t.Fatalf("writing editor.css fixture: %v", err)
	}

	v := LoadAssetVersions(dir)
	if v.EditorJS == "" {
		t.Error("LoadAssetVersions().EditorJS is empty, want a hash")
	}
	if v.EditorCSS == "" {
		t.Error("LoadAssetVersions().EditorCSS is empty, want a hash")
	}
}

func TestLoadAssetVersionsHashesTransposeJS(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "js"), 0755); err != nil {
		t.Fatalf("making js dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "js", "transpose.js"), []byte("console.log(1)"), 0644); err != nil {
		t.Fatalf("writing transpose.js fixture: %v", err)
	}

	v := LoadAssetVersions(dir)
	if v.TransposeJS == "" {
		t.Error("LoadAssetVersions().TransposeJS is empty, want a hash")
	}
}

func TestVersionedHrefAppendsVersionQueryParam(t *testing.T) {
	got := versionedHref("/static/css/style.css", "abc123")
	want := "/static/css/style.css?v=abc123"
	if got != want {
		t.Errorf("versionedHref() = %q, want %q", got, want)
	}
}

func TestVersionedHrefOmitsQueryParamWhenVersionEmpty(t *testing.T) {
	got := versionedHref("/static/css/style.css", "")
	want := "/static/css/style.css"
	if got != want {
		t.Errorf("versionedHref() = %q, want %q (no query param when version unknown)", got, want)
	}
}

func noopHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestStaticCacheHeadersSetsLongLivedImmutableWhenVersioned(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/static/css/style.css?v=abc123", nil)
	rec := httptest.NewRecorder()

	staticCacheHeaders(http.HandlerFunc(noopHandler)).ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "immutable") || !strings.Contains(cc, "max-age=31536000") {
		t.Errorf("Cache-Control = %q, want long-lived immutable for a versioned request", cc)
	}
}

func TestStaticCacheHeadersDoesNotCacheAggressivelyWithoutVersion(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil)
	rec := httptest.NewRecorder()

	staticCacheHeaders(http.HandlerFunc(noopHandler)).ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, unversioned requests must not be cached as immutable", cc)
	}
}
