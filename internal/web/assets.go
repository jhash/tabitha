package web

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
)

// AssetVersions holds a short content hash per static entrypoint referenced
// directly by the page chrome, computed once at server startup (see
// NewRouter) — not per-request. Appending it as a ?v= query param busts
// Cloudflare's (and browsers') cache on every deploy that actually changes
// the file, without needing a manual cache purge.
type AssetVersions struct {
	Reset        string
	Style        string
	Htmx         string
	LoraVariable string
	EditorJS     string
	EditorCSS    string
	PlayJS       string
	PlayCSS      string
	OfflineDB    string
	OfflineSync  string
}

// assets holds the process-wide asset versions, set once by NewRouter
// before the server starts accepting requests. Zero value (all empty
// strings) is safe — versionedHref just omits the query param.
var assets AssetVersions

// SetAssetVersions installs the asset versions Page/PageWide use when
// rendering static asset URLs. Call once at startup, before serving
// traffic — not safe to call concurrently with request handling.
func SetAssetVersions(v AssetVersions) {
	assets = v
}

// loadAssetVersion hashes a static file's current content on disk, for use
// as a cache-busting query param. Returns "" if the file can't be read
// (e.g. wrong working directory in a test) — dev/test safety, not
// something callers need to check.
func loadAssetVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:10]
}

// LoadAssetVersions hashes every static file the page chrome links to
// directly, rooted at dir (the same directory NewRouter's file server
// serves from).
func LoadAssetVersions(dir string) AssetVersions {
	return AssetVersions{
		Reset:        loadAssetVersion(dir + "/css/reset.css"),
		Style:        loadAssetVersion(dir + "/css/style.css"),
		Htmx:         loadAssetVersion(dir + "/js/htmx.min.js"),
		LoraVariable: loadAssetVersion(dir + "/fonts/Lora-Variable.woff2"),
		EditorJS:     loadAssetVersion(dir + "/js/editor.js"),
		EditorCSS:    loadAssetVersion(dir + "/css/editor.css"),
		PlayJS:       loadAssetVersion(dir + "/js/play.js"),
		PlayCSS:      loadAssetVersion(dir + "/css/play.css"),
		OfflineDB:    loadAssetVersion(dir + "/js/offline-db.js"),
		OfflineSync:  loadAssetVersion(dir + "/js/offline-sync.js"),
	}
}

// versionedHref appends a ?v=<version> cache-busting query param, or
// returns path unchanged when version is empty (unknown asset version
// shouldn't produce a malformed URL).
func versionedHref(path, version string) string {
	if version == "" {
		return path
	}
	return path + "?v=" + version
}

// staticCacheHeaders sets Cache-Control on /static/* responses: requests
// carrying our ?v= cache-busting param are safe to cache forever (the URL
// itself changes whenever the file's content does, so there's nothing to
// invalidate — this is what lets Cloudflare's edge cache serve style.css
// etc. without needing a manual purge after every deploy). Requests
// without it (old bookmarked/linked URLs, direct hits) get a short,
// must-revalidate policy instead of inheriting Cloudflare's own default
// static-extension caching, which is what went stale before this existed.
func staticCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "v=") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
		}
		next.ServeHTTP(w, r)
	})
}
