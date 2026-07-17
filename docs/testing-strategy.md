# Testing strategy

## Default: Go unit tests

Most behavior — query building (`internal/web/song_query.go`), rendering
(`renderSongShow`, `homeContent`, etc.), business logic — has a pure Go
function underneath a handler. Test that function directly: no HTTP, no
DB, fast. See `song_show_test.go`, `song_query_test.go` for the pattern.

## Full HTTP stack, no browser: `httptest` + `goquery`

When behavior only exists at the handler/router level — auth gating,
routing/redirects (id vs slug), full-page rendering wired through
`NewRouter`, query-param round-trips — test it with a real
`*http.Request`/`httptest.ResponseRecorder` against `NewRouter(...)`, same
as `router_test.go` already does throughout.

For assertions beyond "does this substring appear somewhere in the body"
(row order, row count, which link has which href, which element has an
`hx-*` attribute), parse the response with
[`goquery`](https://github.com/PuerkitoBio/goquery) instead of
`strings.Contains`. goquery gives CSS-selector querying over the returned
HTML — no browser, no JS execution — so it's exactly as fast as any other
Go test, but lets assertions target structure instead of raw text.

```go
rec := httptest.NewRecorder()
r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?sort=title&order=desc", nil))

doc, err := goquery.NewDocumentFromReader(rec.Body)
// ...
titles := doc.Find("table tbody tr td:first-child a").Map(func(_ int, s *goquery.Selection) string {
	return s.Text()
})
// assert titles are in descending order
```

This is the default for anything server-rendered — which is nearly
everything in tabitha (htmx-boosted, not a client-rendered SPA). Prefer it
over spinning up a browser.

## Real browser: Playwright (`e2e/`)

Reserved for pages/features that are genuinely client-rendered — where
the server sends no meaningful HTML and behavior only exists after JS
runs. Today that's exactly the song editor (`/songs/:id/edit`): it mounts
an empty `<div id="tabitha-editor-root">` and a ProseMirror React island
does everything — layout math (chord label positioning, wrap-gap
measurement), drag-select, `contentEditable` typing, click-driven popups.
None of that is observable via a plain HTTP response, so it can only be
tested by actually running the JS in a browser. See
`e2e/tests/song-editor.spec.ts`.

**Before adding a new Playwright test, check whether the page is actually
CSR.** If the server renders the real content (see `SongShowHandler` vs
`SongEditHandler` for the contrast — the show page renders transcription
HTML server-side, the edit page renders an empty mount div), a
`goquery`-based `httptest` test covers it faster and without the
browser/CI overhead. Reach for Playwright only when there's no HTTP
response to inspect that would prove the behavior — i.e. the assertion
inherently depends on JS having run (computed layout, DOM mutations from
event handlers, etc.).
