package web

import (
	"bytes"
	"os"
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func renderPage(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	node := Page("Test", "Test page", nil, false)
	if err := node.Render(&buf); err != nil {
		t.Fatalf("failed to render page: %v", err)
	}
	return buf.String()
}

func TestPagePreloadsLoraFont(t *testing.T) {
	html := renderPage(t)
	if !strings.Contains(html, "/static/fonts/Lora-Variable.woff2") {
		t.Error("expected page head to preload /static/fonts/Lora-Variable.woff2")
	}
	if !strings.Contains(html, `rel="preload"`) {
		t.Error("expected font preload to include rel=\"preload\"")
	}
	if !strings.Contains(html, `as="font"`) {
		t.Error("expected font preload to include as=\"font\"")
	}
}

func TestPageDoesNotLoadFontsFromGoogleCDN(t *testing.T) {
	html := renderPage(t)
	if strings.Contains(html, "fonts.googleapis.com") || strings.Contains(html, "fonts.gstatic.com") {
		t.Error("page must not load fonts from Google Fonts CDN — use self-hosted fonts to prevent FOUT")
	}
}

func TestPageLoadsHtmxSelfHostedNotFromCDN(t *testing.T) {
	html := renderPage(t)
	if !strings.Contains(html, "/static/js/htmx.min.js") {
		t.Error("expected page to load self-hosted /static/js/htmx.min.js")
	}
	if strings.Contains(html, "unpkg.com") || strings.Contains(html, "cdn.jsdelivr.net") || strings.Contains(html, "cdn.tailwindcss") {
		t.Error("page must not load htmx (or any script) from a third-party CDN")
	}
}

func TestPageEnablesHtmxBoost(t *testing.T) {
	html := renderPage(t)
	if !strings.Contains(html, `hx-boost="true"`) {
		t.Error(`expected hx-boost="true" so pages work without JS and boosting enhances them`)
	}
}

func TestPageDoesNotDuplicateCharsetOrViewportMeta(t *testing.T) {
	// components.HTML5 already inserts these — a caller adding them again
	// produces invalid duplicate <meta> tags in <head>.
	html := renderPage(t)
	if n := strings.Count(html, `charset="utf-8"`); n != 1 {
		t.Errorf(`charset meta appears %d times, want exactly 1`, n)
	}
	if n := strings.Count(html, `name="viewport"`); n != 1 {
		t.Errorf(`viewport meta appears %d times, want exactly 1`, n)
	}
}

func TestPageOmitsAdminLinkForNonSuperadmin(t *testing.T) {
	html := renderPage(t)
	if strings.Contains(html, "site-admin-link") {
		t.Error("expected no admin link in header for a non-superadmin viewer")
	}
}

func TestPageShowsAdminLinkForSuperadmin(t *testing.T) {
	var buf bytes.Buffer
	if err := Page("Test", "Test page", nil, true).Render(&buf); err != nil {
		t.Fatalf("failed to render page: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `href="/admin"`) {
		t.Error("expected header to link to /admin for a superadmin viewer")
	}
}

func TestPageSetsTitleAndDescription(t *testing.T) {
	var buf bytes.Buffer
	if err := Page("My Song", "A great song", nil, false).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "<title>My Song") {
		t.Errorf("expected <title> to contain %q, got: %s", "My Song", html)
	}
	if !strings.Contains(html, `content="A great song"`) {
		t.Error("expected meta description to contain the page description")
	}
}

func TestPageSetsOpenGraphTags(t *testing.T) {
	var buf bytes.Buffer
	if err := Page("My Song", "A great song", nil, false).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `property="og:title" content="My Song"`) {
		t.Errorf("expected an og:title meta tag, got: %s", html)
	}
	if !strings.Contains(html, `property="og:description" content="A great song"`) {
		t.Errorf("expected an og:description meta tag, got: %s", html)
	}
	if !strings.Contains(html, `property="og:type" content="website"`) {
		t.Errorf("expected an og:type meta tag, got: %s", html)
	}
}

func TestPageOmitsOpenGraphDescriptionWhenBlank(t *testing.T) {
	var buf bytes.Buffer
	if err := Page("My Song", "", nil, false).Render(&buf); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(buf.String(), "og:description") {
		t.Error("expected no og:description meta tag when description is blank")
	}
}

func TestCSSDefinesFontFaceForLora(t *testing.T) {
	css := readRepoFile(t, "static/css/style.css")
	if !strings.Contains(css, "@font-face") {
		t.Error("expected style.css to contain @font-face rules for self-hosted Lora")
	}
	if !strings.Contains(css, "Lora-Variable.woff2") {
		t.Error("expected @font-face to reference the self-hosted Lora woff2 file")
	}
}

func TestCSSFontFaceUsesFontDisplayOptional(t *testing.T) {
	css := readRepoFile(t, "static/css/style.css")
	if !strings.Contains(css, "font-display: optional") {
		t.Error("expected @font-face to use font-display: optional to prevent FOUT")
	}
}

func TestVendoredStaticAssetsExistOnDisk(t *testing.T) {
	// The rendered HTML referencing a path (tested above) doesn't prove the
	// file is actually there — assert both, since a page can link to a
	// self-hosted asset that was never actually placed at that path.
	for _, f := range []string{
		"static/fonts/Lora-Variable.woff2",
		"static/fonts/Lora-Italic-Variable.woff2",
		"static/js/htmx.min.js",
		"static/css/reset.css",
		"static/css/style.css",
	} {
		if _, err := os.Stat(repoPath(t, f)); err != nil {
			t.Errorf("expected vendored asset to exist on disk: %s (%v)", f, err)
		}
	}
}

func TestLoraFontFilesExistAndAreSmallEnoughToPreload(t *testing.T) {
	for _, f := range []string{
		"static/fonts/Lora-Variable.woff2",
		"static/fonts/Lora-Italic-Variable.woff2",
	} {
		info, err := os.Stat(repoPath(t, f))
		if err != nil {
			t.Fatalf("expected font file to exist: %s (%v)", f, err)
		}
		// Sanity budget: a self-hosted, preloaded font this project treats
		// as render-blocking-ish should stay well under 200KB.
		if info.Size() > 200*1024 {
			t.Errorf("%s is %d bytes, expected under 200KB", f, info.Size())
		}
	}
}

func TestPageLinksPWAManifestAndIcons(t *testing.T) {
	html := renderPage(t)
	if !strings.Contains(html, `rel="manifest" href="/static/manifest.webmanifest"`) {
		t.Error("expected page head to link the web app manifest")
	}
	if !strings.Contains(html, `rel="apple-touch-icon" href="/static/icons/apple-touch-icon.png"`) {
		t.Error("expected page head to link an apple-touch-icon")
	}
	if !strings.Contains(html, `name="theme-color"`) {
		t.Error("expected page head to set a theme-color meta tag")
	}
}

func TestPageRegistersServiceWorkerViaOfflineSyncScript(t *testing.T) {
	html := renderPage(t)
	if !strings.Contains(html, "/static/js/offline-db.js") {
		t.Error("expected page to load offline-db.js")
	}
	if !strings.Contains(html, "/static/js/offline-sync.js") {
		t.Error("expected page to load offline-sync.js")
	}
	if !strings.Contains(html, "defer") {
		t.Error("expected offline scripts to be deferred so they never delay first paint")
	}
}

func TestPagePlayAlsoLoadsOfflineSyncScript(t *testing.T) {
	var buf bytes.Buffer
	if err := PagePlay("Test", "Test play page", g.Text("body")).Render(&buf); err != nil {
		t.Fatalf("failed to render play page: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "/static/js/offline-sync.js") {
		t.Error("expected play mode — the actual offline use case — to load offline-sync.js too")
	}
}

func TestVendoredSqlJsExistsOnDisk(t *testing.T) {
	for _, f := range []string{
		"static/js/vendor/sqljs/sql-wasm.js",
		"static/js/vendor/sqljs/sql-wasm.wasm",
	} {
		if _, err := os.Stat(repoPath(t, f)); err != nil {
			t.Errorf("expected vendored asset to exist on disk: %s (%v)", f, err)
		}
	}
}

func TestPWAIconsExistOnDisk(t *testing.T) {
	for _, f := range []string{
		"static/icons/icon-192.png",
		"static/icons/icon-512.png",
		"static/icons/icon-maskable-192.png",
		"static/icons/icon-maskable-512.png",
		"static/icons/apple-touch-icon.png",
	} {
		if _, err := os.Stat(repoPath(t, f)); err != nil {
			t.Errorf("expected PWA icon to exist on disk: %s (%v)", f, err)
		}
	}
}

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	return "../../" + rel
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(repoPath(t, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(data)
}
