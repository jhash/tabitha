# React/TanStack SSR Core Rearchitecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace gomponents with a TanStack Start (React, SSR, Bun) app for the home page and song-show page, backed by a new Go `/api/*` JSON surface, while every other Go route keeps working under a new `/old/*` prefix.

**Architecture:** Go stays the backend/system of record (DB, auth, jobs, admin). A new Bun-runtime TanStack Start app (`web/`) does SSR for `/` and `/songs/{idOrSlug}` only, running as an internal-network-only sidecar container that Go's chi router reverse-proxies those two paths to. Start's loaders/server-functions talk to Go exclusively through a new `/api/*` JSON namespace (including a `/api/auth/whoami` endpoint, since Start has no DB access). All existing HTML view handlers (home, show, play, edit, admin, new-song) move to `/old/*`, same handlers, no logic change — their real canonical paths stop resolving.

**Tech Stack:** Go (chi, gomponents — unchanged), TanStack Start + React + TypeScript + Vite (new, in `web/`), Bun runtime, Vitest (new component tests), Playwright (existing e2e, extended), Docker/Docker Swarm + Traefik (existing infra in the sibling `oracle` repo).

**Spec:** `docs/plans/2026-08-21-react-ssr-core-design.md`

## Global Constraints

- Go remains the backend and system of record — no business logic, DB access, or auth logic moves into the Start app or JS. (Design: "Backend language: staying on Go")
- The Bun sidecar joins the docker stack on an internal network only — no Traefik labels, no public route of its own. Go's `tabitha` service keeps sole ownership of `proxy-net`. (Design: "Architecture")
- All Start↔Go calls go through Go's fetch-adapter module in `web/`, never inline `fetch()` calls scattered through route files — keeps a future Cloudflare Workers migration a config change, not a rewrite. (Design: "Architecture")
- Existing Go HTML view handlers move to `/old/*` verbatim — no rendering logic changes for anything not being rebuilt (play, edit, admin, new-song). (Design: "Routes")
- Mutations from the Start app hit Go's `/api/*` JSON endpoints, forwarding the session cookie — never a separate RPC/GraphQL layer, never protobuf. (Design: "Mutations")
- Offline song-show data is structured JSON, not pre-rendered HTML — the same React component renders it for SSR, hydration, and offline. CRDT/offline-editing-sync is explicitly out of scope. (Design: "Offline & data model")
- Straight cutover — no feature flag, no dual-serving. `/old` is a development-time reference only. (Design: "Rollout")

---

## File Structure

**Go side (existing `internal/web` package):**

- `internal/web/router.go` — modify: remount view handlers under `/old`, add `/api` route group, add the reverse proxy for `/` and `/songs/{idOrSlug}`.
- `internal/web/api_auth.go` — create: `GET /api/auth/whoami`.
- `internal/web/api_songs.go` — create: `GET /api/songs` (list), `GET /api/songs/{idOrSlug}` (detail).
- `internal/web/api_song_content.go` — create: the moved editor-content JSON endpoints (was `song_editor_api.go`'s handlers, same logic, new path).
- `internal/web/api_admin_songs.go` — create: `POST /api/admin/songs/{id}/status` (JSON mutation, wraps the same logic `AdminSetSongStatusHandler` uses).
- `internal/web/proxy.go` — create: the reverse-proxy handler for the Start sidecar, config-driven target URL.
- `internal/web/offline_snapshot.go` — modify: `OfflineSong.HTML` (song-show HTML) replaced with structured fields; `PlayHTML` untouched.
- `internal/config/config.go` — modify: add `StartServiceURL` config value.

**New Start app (`web/`):**

- `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json` — create: project scaffold.
- `web/src/lib/api.ts` — create: the single fetch-adapter module every loader/server-function goes through.
- `web/src/components/Transcription.tsx` — create: the shared chord/lyric render component (TS port of `internal/web/transcription_render.go`'s `splitIntoChordWords`/`renderTranscriptionHTML`), used by SSR, hydration, and offline rendering alike.
- `web/src/components/Transcription.test.tsx` — create: Vitest coverage, mirroring the Go tests for the same logic.
- `web/src/routes/index.tsx` — create: home page route (list, search/filter/sort, inline status edit).
- `web/src/routes/songs.$slug.tsx` — create: song show route.
- `web/src/lib/offline-db.ts` — create: thin TS wrapper around the existing IndexedDB schema (`static/js/offline-db.js`'s layout), used for offline reads.
- `web/src/routes/offline-shell.tsx` — create: the precached, client-only route the service worker serves for an offline song-show request; reads slug from URL, reads JSON from IndexedDB, renders via `Transcription.tsx`.

**Static/service worker side:**

- `static/sw.js` — modify: `serveFromOfflineSnapshot`'s show-page branch serves the precached offline-shell HTML instead of `song.html`; `APP_SHELL` gains the shell's built assets.

**Infra (sibling `oracle` repo):**

- `oracle/services/tabitha/stack.yml` — modify: add the `tabitha-web` (Bun/Start) service.
- `Dockerfile` (this repo) — modify: add a Bun build/runtime stage for `web/`.

---

### Task 1: Move existing HTML view routes under `/old`

**Files:**
- Modify: `internal/web/router.go:44-85`
- Test: `internal/web/router_test.go`

**Interfaces:**
- Produces: every existing HTML view handler (`HomeHandler`, `SongShowHandler`, `SongPlayHandler`, `SongEditHandler`, `SongNewHandler`, `CreateSongHandler`, `GetSongEditorContentHandler`, `PostSongEditorContentHandler`, the `/admin/*` group) now answers only under `/old/...`. Their old top-level paths 404.

- [ ] **Step 1: Write the failing test**

Add to `internal/web/router_test.go` (following the existing pattern in that file — build the router with `NewRouter`, hit it with `httptest`):

```go
func TestOldPrefixServesMovedViewRoutes(t *testing.T) {
	r := newTestRouter(t) // existing test helper in router_test.go

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/old/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /old/: want 200, got %d", rec.Code)
	}
}

func TestRealHomePathNoLongerServesGomponentsHome(t *testing.T) {
	r := newTestRouter(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	// Task 8 wires "/" to the reverse proxy; until then it should not be
	// the old HomeHandler still mounted at "/" — this test only asserts
	// the old mount is gone, not what replaces it.
	body := rec.Body.String()
	if strings.Contains(body, "Jeff's music transcription catalog") {
		t.Fatalf("GET /: still serving the old gomponents home page")
	}
}
```

If `newTestRouter` doesn't already exist in `router_test.go`, use whatever existing helper builds a router for tests there (check the file — most tests call `NewRouter(cfg, q, nil)` directly).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run 'TestOldPrefixServesMovedViewRoutes|TestRealHomePathNoLongerServesGomponentsHome' -v`
Expected: `TestOldPrefixServesMovedViewRoutes` FAILs (404, nothing mounted at `/old` yet).

- [ ] **Step 3: Move the routes in router.go**

In `internal/web/router.go`, replace the block from `r.Get("/", HomeHandler(q))` through the `/admin` route group with:

```go
r.Route("/old", func(r chi.Router) {
    r.Get("/", HomeHandler(q))
    r.With(auth.RequireSuperadmin(q)).Get("/songs/new", SongNewHandler())
    r.With(auth.RequireSuperadmin(q)).Post("/songs", CreateSongHandler(q))
    r.Get("/songs/{idOrSlug}", SongShowHandler(q))
    r.Get("/songs/{idOrSlug}/play", SongPlayHandler(q))
    r.With(auth.RequireSuperadmin(q)).Get("/songs/{idOrSlug}/edit", SongEditHandler(q))
    r.With(auth.RequireSuperadmin(q)).Get("/songs/{idOrSlug}/editor-content", GetSongEditorContentHandler(q))
    r.With(auth.RequireSuperadmin(q)).Post("/songs/{idOrSlug}/editor-content", PostSongEditorContentHandler(q))

    r.Route("/admin", func(r chi.Router) {
        r.Use(auth.RequireSuperadmin(q))
        r.Get("/", AdminHomeHandler)
        r.Get("/songs", AdminSongsHandler)
        r.Get("/users", AdminUsersHandler(q))
        r.Post("/users/{id}/promote", AdminPromoteUserHandler(q))
        r.Post("/songs/bulk-status", AdminBulkSetSongStatusHandler(q, cfg.AppURL, cfClient))
        r.Post("/songs/{id}/status", AdminSetSongStatusHandler(q, cfg.AppURL, cfClient))
        r.Get("/tools", AdminToolsHandler(jobClient))
        r.Get("/jobs", AdminJobsHandler(jobClient))
        r.Post("/tools/toc-sync", AdminTriggerTocSyncHandler(jobClient))
        r.Post("/tools/digest-song", AdminTriggerDigestSongHandler(q, jobClient))
        r.Post("/tools/digest-batch", AdminTriggerDigestBatchHandler(q, jobClient))
    })
})
```

Do not add anything back at the top-level `/`, `/songs/*`, `/admin/*` paths yet — that's Task 8 (proxy) and out of scope for play/edit/admin per the design.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/... -v`
Expected: PASS. Note other existing tests referencing top-level `/`, `/songs/{id}`, `/admin/*` paths will now fail — update every such test's request path to the `/old/...` equivalent (`home_test.go`, `home_db_test.go`, `home_goquery_test.go`, `song_show_test.go`, `song_show_db_test.go`, `song_show_goquery_test.go`, `song_play_test.go`, `song_edit_test.go`, `admin_*_test.go`, `song_new.go`'s tests if any). This is mechanical — same assertions, `/old`-prefixed URL.

- [ ] **Step 5: Commit**

```bash
git add internal/web/router.go internal/web/router_test.go internal/web/*_test.go
git commit -m "Move existing HTML view routes under /old"
```

---

### Task 2: `GET /api/auth/whoami`

**Files:**
- Create: `internal/web/api_auth.go`
- Modify: `internal/web/router.go` (add `/api` route group)
- Test: `internal/web/api_auth_test.go`

**Interfaces:**
- Produces: `WhoamiHandler(q *db.Queries) http.HandlerFunc`, response body `{"loggedIn": bool, "role": string}` (role is `""` when not logged in).

- [ ] **Step 1: Write the failing test**

Create `internal/web/api_auth_test.go`:

```go
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWhoamiLoggedOut(t *testing.T) {
	r := newTestRouter(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/whoami", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var body struct {
		LoggedIn bool   `json:"loggedIn"`
		Role     string `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.LoggedIn || body.Role != "" {
		t.Fatalf("want logged-out response, got %+v", body)
	}
}
```

(A logged-in/superadmin case belongs alongside this once the test suite's existing session-seeding helper is identified — follow the pattern `admin_users_test.go` uses to seed a superadmin session cookie, and add `TestWhoamiSuperadmin` asserting `{"loggedIn":true,"role":"superadmin"}`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestWhoamiLoggedOut -v`
Expected: FAIL (404, route doesn't exist).

- [ ] **Step 3: Implement**

Create `internal/web/api_auth.go`:

```go
package web

import (
	"encoding/json"
	"net/http"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/db"
)

// WhoamiHandler tells the Start app's loaders (which have no direct DB
// access) whether the incoming session cookie belongs to a superadmin —
// the only auth state the home/show pages need, to decide whether to
// render superadmin-only affordances. Always 200; "not logged in" is a
// normal response shape, not an error.
func WhoamiHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			LoggedIn bool   `json:"loggedIn"`
			Role     string `json:"role"`
		}{
			LoggedIn: ok,
			Role:     string(user.Role),
		})
	}
}
```

Check `db.User.Role`'s actual type in `internal/db/models.go` — if it's a named string type (e.g. `db.UserRole`), `string(user.Role)` is correct; if it's already `string`, drop the conversion.

In `internal/web/router.go`, add below the `/old` route group (still inside `NewRouter`, before `return r`):

```go
r.Route("/api", func(r chi.Router) {
    r.Get("/auth/whoami", WhoamiHandler(q))
})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/... -run TestWhoami -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web/api_auth.go internal/web/api_auth_test.go internal/web/router.go
git commit -m "Add GET /api/auth/whoami"
```

---

### Task 3: `GET /api/songs` (home listing)

**Files:**
- Create: `internal/web/api_songs.go`
- Modify: `internal/web/router.go`
- Test: `internal/web/api_songs_test.go`

**Interfaces:**
- Consumes: `SongQueryParams` and `ListSongsQuery` (`internal/web/song_query.go`), `parseSongQueryParams` (`internal/web/home.go:101`), `q.ListDistinctStatuses`, `q.ListDistinctAddedByUsers`.
- Produces: `ListSongsAPIHandler(q *db.Queries) http.HandlerFunc`. Response: `{"songs": [...], "statuses": [...], "addedBy": [...]}`, where each song object is `{id, title, artist, status, addedByName, addedByEmail, updatedAt, addedAt, hasVersion, slug}` (RFC3339 timestamps).

- [ ] **Step 1: Write the failing test**

Create `internal/web/api_songs_test.go` (integration test, needs the real test DB per `docs/testing-strategy.md` — follow `home_db_test.go`'s setup/seed pattern):

```go
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListSongsAPIReturnsSeededSongs(t *testing.T) {
	q := setupTestDB(t) // existing helper, see home_db_test.go
	seedSong(t, q, "Eye of the Tiger", "Survivor")

	r := newTestRouterWithQueries(t, q) // or however router_test.go builds a router around a given *db.Queries

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/songs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Songs []struct {
			Title  string `json:"title"`
			Artist string `json:"artist"`
		} `json:"songs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(body.Songs) != 1 || body.Songs[0].Title != "Eye of the Tiger" {
		t.Fatalf("want one seeded song, got %+v", body.Songs)
	}
}
```

Adjust helper names (`setupTestDB`, `seedSong`, `newTestRouterWithQueries`) to whatever `home_db_test.go`/`router_test.go` actually call — read those files first if the exact names aren't already known from Task 1's work.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestListSongsAPIReturnsSeededSongs -v`
Expected: FAIL (404).

- [ ] **Step 3: Implement**

Create `internal/web/api_songs.go`:

```go
package web

import (
	"encoding/json"
	"net/http"

	"github.com/jhash/tabitha/internal/db"
)

type apiSong struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	Status       string `json:"status"`
	AddedByName  string `json:"addedByName"`
	AddedByEmail string `json:"addedByEmail"`
	UpdatedAt    string `json:"updatedAt"`
	AddedAt      string `json:"addedAt"`
	HasVersion   bool   `json:"hasVersion"`
	Slug         string `json:"slug"`
}

// ListSongsAPIHandler is the JSON equivalent of HomeHandler's data —
// same query, same filters, same effective-timestamp logic
// (effectiveUpdatedAt/effectiveAddedAt), shaped as JSON for the Start
// app's home-page loader instead of gomponents.
func ListSongsAPIHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := parseSongQueryParams(r.URL.Query())

		songs, err := ListSongsQuery(r.Context(), q.DB(), params)
		if err != nil {
			http.Error(w, "failed to load songs", http.StatusInternalServerError)
			return
		}
		statuses, err := q.ListDistinctStatuses(r.Context())
		if err != nil {
			http.Error(w, "failed to load statuses", http.StatusInternalServerError)
			return
		}
		addedByUsers, err := q.ListDistinctAddedByUsers(r.Context())
		if err != nil {
			http.Error(w, "failed to load added-by list", http.StatusInternalServerError)
			return
		}

		out := make([]apiSong, len(songs))
		for i, s := range songs {
			out[i] = apiSong{
				ID: s.ID, Title: s.Title, Artist: s.Artist, Status: s.Status,
				AddedByName: s.AddedByName, AddedByEmail: s.AddedByEmail,
				UpdatedAt: effectiveUpdatedAt(s).Format("2006-01-02T15:04:05Z07:00"),
				AddedAt:   effectiveAddedAt(s).Format("2006-01-02T15:04:05Z07:00"),
				HasVersion: s.HasVersion, Slug: s.Slug,
			}
		}
		addedBy := make([]string, len(addedByUsers))
		for i, u := range addedByUsers {
			label := u.Name
			if label == "" {
				label = u.Email
			}
			addedBy[i] = label
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Songs    []apiSong `json:"songs"`
			Statuses []string  `json:"statuses"`
			AddedBy  []string  `json:"addedBy"`
		}{out, statuses, addedBy})
	}
}
```

In `internal/web/router.go`, add to the `/api` group from Task 2:

```go
r.Get("/songs", ListSongsAPIHandler(q))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/... -run TestListSongsAPI -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web/api_songs.go internal/web/api_songs_test.go internal/web/router.go
git commit -m "Add GET /api/songs"
```

---

### Task 4: `GET /api/songs/{idOrSlug}` (song detail)

**Files:**
- Modify: `internal/web/api_songs.go`
- Modify: `internal/web/router.go`
- Test: `internal/web/api_songs_test.go`

**Interfaces:**
- Consumes: `resolveSongByIDOrSlug` (`internal/web/song_show.go:33`), `currentVersionBlocks` (`internal/web/song_show.go:76`), `omitDuplicateHeaderLines` (`internal/web/song_show.go:123`), `songShowHref` (`internal/web/song_show.go:23`).
- Produces: `GetSongAPIHandler(q *db.Queries) http.HandlerFunc`. Response: `{id, title, artist, slug, key, hasVersion, blocks: [...]}` where `blocks` is `transcription.Block` marshaled directly (already JSON-tagged — see `transcription.MarshalDocument`'s usage in `song_editor_api.go`), pre-trimmed via `omitDuplicateHeaderLines`. 404 (not redirect) when the song doesn't resolve — canonicalization by slug is the Start route's job, not this API's.

- [ ] **Step 1: Write the failing test**

Add to `internal/web/api_songs_test.go`:

```go
func TestGetSongAPIReturnsBlocks(t *testing.T) {
	q := setupTestDB(t)
	song := seedDigestedSong(t, q, "Eye of the Tiger", "Survivor") // existing helper — see song_show_db_test.go for the digested-song seeding pattern

	r := newTestRouterWithQueries(t, q)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/songs/"+song.Slug, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Title      string `json:"title"`
		HasVersion bool   `json:"hasVersion"`
		Blocks     []any  `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if body.Title != "Eye of the Tiger" || !body.HasVersion || len(body.Blocks) == 0 {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestGetSongAPIUnknownSlugReturns404(t *testing.T) {
	q := setupTestDB(t)
	r := newTestRouterWithQueries(t, q)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/songs/does-not-exist", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestGetSongAPI -v`
Expected: FAIL (404 route doesn't exist / handler undefined).

- [ ] **Step 3: Implement**

Append to `internal/web/api_songs.go`:

```go
// GetSongAPIHandler is the JSON equivalent of SongShowHandler's data —
// same lookup and duplicate-header trimming, shaped as JSON for the
// Start app's song-show loader instead of gomponents. Unlike the
// gomponents handler, it does not redirect a numeric-ID lookup to the
// slug URL — that canonicalization belongs to the Start route, which
// has the request's own URL to redirect from.
func GetSongAPIHandler(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idOrSlug := chi.URLParam(r, "idOrSlug")

		song, err := resolveSongByIDOrSlug(r, q, idOrSlug)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		blocks, key, hasVersion, err := currentVersionBlocks(r.Context(), q, song)
		if err != nil {
			http.Error(w, "failed to load transcription", http.StatusInternalServerError)
			return
		}
		if hasVersion {
			blocks = omitDuplicateHeaderLines(blocks, song)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			ID         int64                  `json:"id"`
			Title      string                 `json:"title"`
			Artist     string                 `json:"artist"`
			Slug       string                 `json:"slug"`
			Key        string                 `json:"key"`
			HasVersion bool                   `json:"hasVersion"`
			Blocks     []transcription.Block  `json:"blocks"`
		}{song.ID, song.Title, song.Artist, song.Slug, key, hasVersion, blocks})
	}
}
```

Add `"github.com/go-chi/chi/v5"` and `"github.com/jhash/tabitha/internal/transcription"` to the file's imports.

In `internal/web/router.go`'s `/api` group:

```go
r.Get("/songs/{idOrSlug}", GetSongAPIHandler(q))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/... -run TestGetSongAPI -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web/api_songs.go internal/web/api_songs_test.go internal/web/router.go
git commit -m "Add GET /api/songs/{idOrSlug}"
```

---

### Task 5: Move editor-content endpoints to `/api/songs/{id}/content`

**Files:**
- Modify: `internal/web/router.go`
- Modify: `internal/web/song_editor_api_test.go` (update paths)
- Rename references only — `GetSongEditorContentHandler`/`PostSongEditorContentHandler` themselves are untouched; only their mount point changes.

**Interfaces:**
- Consumes: `GetSongEditorContentHandler`, `PostSongEditorContentHandler` (`internal/web/song_editor_api.go`, unchanged).
- Produces: same handlers now also reachable at `/api/songs/{idOrSlug}/content`; the old-path versions under `/old/songs/{idOrSlug}/editor-content` (added in Task 1) stay as-is since the editor page itself hasn't moved.

- [ ] **Step 1: Write the failing test**

Add to `internal/web/song_editor_api_test.go` (copy the shape of the existing GET/POST tests there, new path):

```go
func TestGetSongEditorContentAPIPath(t *testing.T) {
	q := setupTestDB(t)
	song := seedDigestedSong(t, q, "Eye of the Tiger", "Survivor")
	r := newTestRouterWithQueries(t, q)

	req := httptest.NewRequest(http.MethodGet, "/api/songs/"+song.Slug+"/content", nil)
	req.AddCookie(superadminSessionCookie(t, q)) // existing helper for a superadmin session, per e.g. admin_users_test.go
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestGetSongEditorContentAPIPath -v`
Expected: FAIL (404).

- [ ] **Step 3: Implement**

In `internal/web/router.go`'s `/api` group, add (superadmin-gated, matching the existing gate):

```go
r.With(auth.RequireSuperadmin(q)).Get("/songs/{idOrSlug}/content", GetSongEditorContentHandler(q))
r.With(auth.RequireSuperadmin(q)).Post("/songs/{idOrSlug}/content", PostSongEditorContentHandler(q))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/... -run TestGetSongEditorContentAPIPath -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web/router.go internal/web/song_editor_api_test.go
git commit -m "Expose editor content endpoints under /api/songs/{id}/content"
```

---

### Task 6: `POST /api/admin/songs/{id}/status`

**Files:**
- Create: `internal/web/api_admin_songs.go`
- Modify: `internal/web/router.go`
- Test: `internal/web/api_admin_songs_test.go`

**Interfaces:**
- Consumes: whatever `AdminSetSongStatusHandler` (`internal/web/admin_songs.go`) does internally — read that file first; this task wraps the same status-update + Cloudflare-purge logic in a JSON responder rather than reusing the htmx-oriented handler directly, since that one likely returns an HTML fragment / empty 200 for `hx-swap="none"`.
- Produces: `AdminSetSongStatusAPIHandler(q *db.Queries, appURL string, cf *cloudflare.Client) http.HandlerFunc`. Request: `{"status": "..."}` JSON body. Response: `{"ok": true}` on success.

- [ ] **Step 1: Read `admin_songs.go` first**

Before writing code, read `internal/web/admin_songs.go` in full to see exactly what `AdminSetSongStatusHandler` does (which query it calls to persist the status, and how/when it triggers the Cloudflare purge) — this task's handler must do the identical write + purge, just answer JSON instead of an htmx fragment.

- [ ] **Step 2: Write the failing test**

Create `internal/web/api_admin_songs_test.go`:

```go
package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetSongStatusAPIUpdatesStatus(t *testing.T) {
	q := setupTestDB(t)
	song := seedSong(t, q, "Eye of the Tiger", "Survivor")
	r := newTestRouterWithQueries(t, q)

	body, _ := json.Marshal(map[string]string{"status": "ready"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/admin/songs/%d/status", song.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(superadminSessionCookie(t, q))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	updated, err := q.GetSongByID(t.Context(), song.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "ready" {
		t.Fatalf("want status 'ready', got %q", updated.Status)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestSetSongStatusAPIUpdatesStatus -v`
Expected: FAIL (404).

- [ ] **Step 4: Implement**

Create `internal/web/api_admin_songs.go`, mirroring exactly what Step 1's reading found in `AdminSetSongStatusHandler` — same DB write call, same Cloudflare purge call, same superadmin gate — just JSON in/out:

```go
package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/jhash/tabitha/internal/cloudflare"
	"github.com/jhash/tabitha/internal/db"
)

// AdminSetSongStatusAPIHandler is the JSON equivalent of
// AdminSetSongStatusHandler — same write + Cloudflare purge, answering
// {"ok":true}/an error instead of an htmx fragment, for the Start app's
// inline status-edit server function.
func AdminSetSongStatusAPIHandler(q *db.Queries, appURL string, cf *cloudflare.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid song id", http.StatusBadRequest)
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		// TODO(implementer): call the exact same query + purge steps
		// AdminSetSongStatusHandler uses in admin_songs.go, with id and
		// body.Status. Do not invent new query names — reuse what that
		// handler already calls.

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}
```

Replace the `TODO` with the real call once Step 1's reading confirms the exact query/purge function names — this is the one spot in the plan where the concrete call can't be named without having read `admin_songs.go` first, which is why Step 1 is a dedicated read-first step.

In `internal/web/router.go`'s `/api` group:

```go
r.Route("/admin", func(r chi.Router) {
    r.Use(auth.RequireSuperadmin(q))
    r.Post("/songs/{id}/status", AdminSetSongStatusAPIHandler(q, cfg.AppURL, cfClient))
})
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/web/... -run TestSetSongStatusAPIUpdatesStatus -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/web/api_admin_songs.go internal/web/api_admin_songs_test.go internal/web/router.go
git commit -m "Add POST /api/admin/songs/{id}/status"
```

---

### Task 7: Config value for the Start sidecar's internal URL

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `Config.StartServiceURL string` — populated from the `START_SERVICE_URL` env var, e.g. `http://tabitha-web:3000` (Docker service DNS name, matches whatever Task 15's stack.yml service is named). Empty string is valid in dev (Task 8's proxy handler treats empty as "not configured" and 502s clearly rather than panicking).

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`, following that file's existing pattern for other env-var-backed fields:

```go
func TestLoadReadsStartServiceURL(t *testing.T) {
	t.Setenv("START_SERVICE_URL", "http://tabitha-web:3000")
	t.Setenv("DATABASE_URL", "postgres:///doesnotmatter") // whatever else Load() requires — check existing tests for the minimal required set

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StartServiceURL != "http://tabitha-web:3000" {
		t.Fatalf("want http://tabitha-web:3000, got %q", cfg.StartServiceURL)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestLoadReadsStartServiceURL -v`
Expected: FAIL (`cfg.StartServiceURL` undefined field).

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add `StartServiceURL string` to the `Config` struct and populate it in `Load()` the same way every other plain string env var there is read (find the existing pattern — likely `os.Getenv("...")` with no required-check, matching `NTFY_URL`'s optionality).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "Add START_SERVICE_URL config value"
```

---

### Task 8: Reverse proxy `/` and `/songs/{idOrSlug}` to the Start sidecar

**Files:**
- Create: `internal/web/proxy.go`
- Modify: `internal/web/router.go`
- Test: `internal/web/proxy_test.go`

**Interfaces:**
- Consumes: `cfg.StartServiceURL` (Task 7).
- Produces: `StartProxyHandler(targetURL string) http.HandlerFunc` — a `httputil.ReverseProxy` wrapper; 502 with a short plain-text body if `targetURL` is empty or unparseable, instead of panicking.

- [ ] **Step 1: Write the failing test**

Create `internal/web/proxy_test.go`:

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartProxyForwardsToTarget(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("from upstream: " + r.URL.Path))
	}))
	defer upstream.Close()

	handler := StartProxyHandler(upstream.URL)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/songs/some-slug", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if rec.Body.String() != "from upstream: /songs/some-slug" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestStartProxyEmptyTargetReturns502(t *testing.T) {
	handler := StartProxyHandler("")
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestStartProxy -v`
Expected: FAIL (`StartProxyHandler` undefined).

- [ ] **Step 3: Implement**

Create `internal/web/proxy.go`:

```go
package web

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// StartProxyHandler forwards a request to the TanStack Start sidecar —
// the canonical "/" and "/songs/{idOrSlug}" routes only (see router.go).
// targetURL is Config.StartServiceURL; empty or unparseable fails loudly
// with 502 rather than a nil-pointer panic, since an unset value in a
// misconfigured deploy should be obviously visible, not a crash.
func StartProxyHandler(targetURL string) http.HandlerFunc {
	target, err := url.Parse(targetURL)
	if targetURL == "" || err != nil || target.Host == "" {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Start service not configured", http.StatusBadGateway)
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	return proxy.ServeHTTP
}
```

In `internal/web/router.go`, mount this at the top level, replacing what used to be `HomeHandler`/`SongShowHandler`:

```go
r.Get("/", StartProxyHandler(cfg.StartServiceURL))
r.Get("/songs/{idOrSlug}", StartProxyHandler(cfg.StartServiceURL))
```

Place these before the `/old` and `/api` route groups so there's no path-matching ambiguity with `/songs/{idOrSlug}/play` etc., which now only exist under `/old`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/... -v`
Expected: PASS across the package — this is also the point to re-run `TestRealHomePathNoLongerServesGomponentsHome` from Task 1 and confirm it still passes (now for a different reason: `/` is proxied, not just unmounted).

- [ ] **Step 5: Commit**

```bash
git add internal/web/proxy.go internal/web/proxy_test.go internal/web/router.go
git commit -m "Reverse-proxy / and /songs/{idOrSlug} to the Start sidecar"
```

---

### Task 9: Rewrite offline song-show snapshot as structured JSON

**Files:**
- Modify: `internal/web/offline_snapshot.go`
- Test: `internal/web/offline_snapshot_test.go`, `internal/web/offline_routes_test.go`

**Interfaces:**
- Consumes: `transcription.UnmarshalDocument`, `omitDuplicateHeaderLines` (`internal/web/song_show.go:123`) — the offline JSON should carry the same trimmed blocks the live page shows.
- Produces: `OfflineSong` struct changes — `HTML string` removed, replaced with `Key string`, `HasVersion bool`, `Blocks []transcription.Block`. `PlayHTML` unchanged. `RenderOfflineSong` no longer calls `renderOfflineSongPage`.

- [ ] **Step 1: Write the failing test**

Update `internal/web/offline_snapshot_test.go`'s existing test(s) for `RenderOfflineSong` (read the file first for the exact current test names/shape) to assert on the new fields instead of `HTML`:

```go
func TestRenderOfflineSongReturnsStructuredBlocks(t *testing.T) {
	q := setupTestDB(t)
	song := seedDigestedSong(t, q, "Eye of the Tiger", "Survivor")

	got, err := RenderOfflineSong(t.Context(), q, song.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("want non-nil OfflineSong")
	}
	if len(got.Blocks) == 0 {
		t.Fatal("want non-empty Blocks")
	}
	if got.PlayHTML == "" {
		t.Fatal("PlayHTML should be unchanged")
	}
}
```

Remove or update any existing assertion in that file that checks `got.HTML` — it no longer exists.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/web/... -run TestRenderOfflineSongReturnsStructuredBlocks -v`
Expected: FAIL (compile error — `Blocks` field doesn't exist yet).

- [ ] **Step 3: Implement**

In `internal/web/offline_snapshot.go`, change the `OfflineSong` struct:

```go
type OfflineSong struct {
	Slug        string                 `json:"slug"`
	ID          int64                  `json:"id"`
	Title       string                 `json:"title"`
	Artist      string                 `json:"artist"`
	Key         string                 `json:"key"`
	HasVersion  bool                   `json:"hasVersion"`
	Blocks      []transcription.Block  `json:"blocks"`
	PlayHTML    string                 `json:"playHtml"`
	ContentHash string                 `json:"contentHash"`
}
```

And `RenderOfflineSong`:

```go
func RenderOfflineSong(ctx context.Context, q *db.Queries, slug string) (*OfflineSong, error) {
	row, err := q.GetSongForOfflineSnapshotBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	blocks, err := transcription.UnmarshalDocument(row.TranscriptionVersion.Content)
	if err != nil {
		return nil, err
	}
	key := deref(row.TranscriptionVersion.Key)
	playHTML, err := renderOfflinePlayPage(row.Song, blocks, key)
	if err != nil {
		return nil, err
	}

	return &OfflineSong{
		Slug: row.Song.Slug, ID: row.Song.ID, Title: row.Song.Title, Artist: row.Song.Artist,
		Key: key, HasVersion: true,
		Blocks:      omitDuplicateHeaderLines(blocks, row.Song),
		PlayHTML:    playHTML,
		ContentHash: row.ContentHash,
	}, nil
}
```

Delete `renderOfflineSongPage` entirely — nothing calls it anymore.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/web/... -v`
Expected: PASS. `offline_routes_test.go` likely also asserts on the `/offline/songs/{slug}` response body shape — update any `HTML`-field assertions there the same way.

- [ ] **Step 5: Commit**

```bash
git add internal/web/offline_snapshot.go internal/web/offline_snapshot_test.go internal/web/offline_routes_test.go
git commit -m "Offline song-show snapshot: structured JSON instead of rendered HTML"
```

---

### Task 10: Scaffold the TanStack Start app (`web/`)

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/src/router.tsx`, `web/src/routes/__root.tsx`, `web/.gitignore`

**Interfaces:**
- Produces: a runnable Start app with one placeholder route, serving on port 3000, runnable via `bun run dev` and buildable via `bun run build` + `bun run start`.

- [ ] **Step 1: Init the project**

```bash
mkdir -p web/src/routes
cd web
bun init -y
bun add @tanstack/react-start @tanstack/react-router react react-dom
bun add -D @vitejs/plugin-react vite typescript @types/react @types/react-dom vitest @testing-library/react
```

Follow TanStack Start's current scaffold output structure (`vite.config.ts` with the Start Vite plugin, `src/router.tsx` exporting `createRouter`, `src/routes/__root.tsx` as the root layout, file-based routes under `src/routes/`) — this mirrors the setup `editor/vite.config.ts` already does for the ProseMirror bundle, but as a full SSR app rather than a static bundle.

- [ ] **Step 2: Add a placeholder home route**

Create `web/src/routes/index.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/')({
  component: () => <div>tabitha (placeholder)</div>,
})
```

- [ ] **Step 3: Verify it runs**

Run: `cd web && bun run dev`
Expected: dev server starts on `http://localhost:3000`, and `curl http://localhost:3000/` returns HTML containing `tabitha (placeholder)` (confirms SSR is actually rendering server-side, not just shipping an empty shell).

- [ ] **Step 4: Add `.gitignore`**

```
node_modules/
dist/
.output/
.vinxi/
```

- [ ] **Step 5: Commit**

```bash
git add web/
git commit -m "Scaffold TanStack Start app in web/"
```

---

### Task 11: Shared `Transcription` component (TS port of `transcription_render.go`)

**Files:**
- Create: `web/src/components/Transcription.tsx`
- Create: `web/src/components/Transcription.test.tsx`
- Create: `web/src/lib/chordWords.ts`

**Interfaces:**
- Consumes: a `Block[]` shape matching Go's `transcription.Block` JSON tags — read `internal/transcription/blocks.go` for the exact field names/JSON tags before writing the TS type, so `apiSong.blocks` from Task 4 deserializes without a mapping layer.
- Produces: `splitIntoChordWords(tokens: Token[]): ChordWord[]` (`web/src/lib/chordWords.ts`) — direct port of `splitIntoChordWords` (`internal/web/transcription_render.go:46`). `<Transcription blocks={Block[]} />` (`web/src/components/Transcription.tsx`) — direct port of `renderTranscriptionHTML`/`chordLineNode`/`textLineContent`/`lyricWordNode`.

- [ ] **Step 1: Read the Go source and the block schema first**

Read `internal/transcription/blocks.go` (for `Block`/`Token`'s JSON field names and the `Kind` enum's JSON values) and `internal/web/transcription_render.go` in full (already read once during design — re-check exact rune-by-rune logic in `splitIntoChordWords` before porting, it's the trickiest part: chord tokens attach to the *next* word, synthetic tokens are dropped, and word boundaries are found by iterating runes across token boundaries, not `String.split(' ')`, so a chord landing mid-word doesn't fragment it).

- [ ] **Step 2: Write the failing test**

Create `web/src/components/Transcription.test.tsx` — port the Go test cases directly (find them in `internal/web/transcription_render_test.go`, e.g. `TestRenderTranscriptionHTMLEachChordWordIsIndependentlyWrappable`, and the mid-word-chord split case referenced in `splitIntoChordWords`'s doc comment):

```tsx
import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/react'
import { Transcription } from './Transcription'
import type { Block } from '../lib/chordWords'

describe('Transcription', () => {
  it('renders each chord-word as an independently wrappable span', () => {
    const blocks: Block[] = [
      {
        kind: 'chord_lyric_pair',
        tokens: [
          { chord: 'C', text: '' },
          { text: 'Hello ' },
          { chord: 'G', text: '' },
          { text: 'world' },
        ],
      },
    ]
    const { container } = render(<Transcription blocks={blocks} />)
    const words = container.querySelectorAll('.chord-word')
    expect(words).toHaveLength(2)
    expect(words[0].querySelector('.chord')?.textContent).toBe('C')
    expect(words[0].querySelector('.lyric')?.textContent).toBe('Hello')
    expect(words[1].querySelector('.chord')?.textContent).toBe('G')
    expect(words[1].querySelector('.lyric')?.textContent).toBe('world')
  })

  it('does not fragment a word split mid-word by a chord token', () => {
    // Mirrors the Go doc comment's example: "you" stored as "yo"/"u"
    // across adjacent Text tokens with no whitespace between them.
    const blocks: Block[] = [
      {
        kind: 'chord_lyric_pair',
        tokens: [
          { text: 'yo' },
          { chord: 'Am', text: '' },
          { text: 'u' },
        ],
      },
    ]
    const { container } = render(<Transcription blocks={blocks} />)
    const words = container.querySelectorAll('.chord-word')
    expect(words).toHaveLength(1)
    expect(words[0].querySelector('.lyric')?.textContent).toBe('you')
  })
})
```

Adjust the exact `Block`/`Token` field names to whatever Step 1's read of `blocks.go` actually finds (JSON tags there are the ground truth, not this draft).

- [ ] **Step 3: Run test to verify it fails**

Run: `cd web && bun run test -- Transcription`
Expected: FAIL (`./Transcription` module doesn't exist).

- [ ] **Step 4: Implement `chordWords.ts`**

Port `splitIntoChordWords` faithfully — same algorithm as `internal/web/transcription_render.go:46-91`: iterate tokens; a chord token sets `pendingChord`/flushes any prior pending chord; a synthetic token is skipped; a real-text token is walked rune-by-rune (in JS, iterate the string's code points, e.g. `for (const ch of t.text)`), flushing on whitespace and otherwise appending to the current word buffer, capturing the *first* token's bold/italic/underline marks for the word. Match the Go version's `flush()` semantics exactly (emits a word if the buffer is non-empty OR a chord is pending, even with an empty lyric — the "consecutive chords with no lyric between them" case).

- [ ] **Step 5: Implement `Transcription.tsx`**

Port `renderTranscriptionHTML`/`textLineContent`/`chordLineNode`/`lyricWordNode` (`internal/web/transcription_render.go:100-169`) as React components with the same class names (`transcription`, `section-header`, `text-line`, `chord-line`, `chord-word`, `chord`, `lyric`, `annotation`) so `static/css/style.css`'s existing rules apply unchanged.

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd web && bun run test -- Transcription`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add web/src/components/Transcription.tsx web/src/components/Transcription.test.tsx web/src/lib/chordWords.ts
git commit -m "Add shared Transcription render component (TS port)"
```

---

### Task 12: Fetch-adapter module

**Files:**
- Create: `web/src/lib/api.ts`

**Interfaces:**
- Consumes: `TABITHA_API_URL` env var (Go's base URL as reachable from the Bun sidecar — same host Go's own `APP_URL`/internal DNS resolves to; set alongside `START_SERVICE_URL` in the stack, e.g. `http://tabitha:8080`).
- Produces: `tabithaFetch(path: string, init?: RequestInit): Promise<Response>` — the *only* function every loader/server function calls to reach Go, forwarding the incoming request's `Cookie` header when one is passed. `getWhoami(cookie?: string)`, `listSongs(params, cookie?)`, `getSong(slug, cookie?)`, `setSongStatus(id, status, cookie)` as typed wrappers over it.

- [ ] **Step 1: Implement**

```ts
// web/src/lib/api.ts
//
// The single point of contact with Go's /api/* surface — every
// loader/server function goes through here, never a bare fetch() to a
// hardcoded URL. Keeps a future swap to Cloudflare Workers edge SSR
// (calling Go's public API instead of an internal Docker hostname) a
// one-file change.

const BASE_URL = process.env.TABITHA_API_URL ?? 'http://localhost:8080'

export async function tabithaFetch(path: string, init: RequestInit & { cookie?: string } = {}) {
  const { cookie, headers, ...rest } = init
  const res = await fetch(`${BASE_URL}${path}`, {
    ...rest,
    headers: { ...headers, ...(cookie ? { Cookie: cookie } : {}) },
  })
  if (!res.ok) {
    throw new Error(`${path} returned ${res.status}`)
  }
  return res
}

export interface Whoami {
  loggedIn: boolean
  role: string
}

export async function getWhoami(cookie?: string): Promise<Whoami> {
  const res = await tabithaFetch('/api/auth/whoami', { cookie })
  return res.json()
}

export async function listSongs(query: string, cookie?: string) {
  const res = await tabithaFetch(`/api/songs${query}`, { cookie })
  return res.json()
}

export async function getSong(idOrSlug: string, cookie?: string) {
  const res = await tabithaFetch(`/api/songs/${encodeURIComponent(idOrSlug)}`, { cookie })
  return res.json()
}

export async function setSongStatus(id: number, status: string, cookie: string) {
  await tabithaFetch(`/api/admin/songs/${id}/status`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
    cookie,
  })
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "Add Go API fetch adapter"
```

(No test here — this is a thin wrapper exercised end-to-end by Tasks 13/14's route tests, which mock `tabithaFetch`'s callers or hit a real local Go server per the e2e setup in Task 16.)

---

### Task 13: Home route

**Files:**
- Create: `web/src/routes/index.tsx` (replaces Task 10's placeholder)
- Create: `web/src/routes/index.test.tsx`

**Interfaces:**
- Consumes: `getWhoami`, `listSongs`, `setSongStatus` (Task 12).
- Produces: `Route` (TanStack file-based route) rendering the song table with search/status/added-by filters and sort-toggle headers (same query params as today: `search`, `status`, `added_by`, `digested`, `sort`, `order` — see `internal/web/home.go:101-119`'s `parseSongQueryParams` for the exact param names/semantics to preserve), plus an inline status `<select>` for superadmins that calls a server function wrapping `setSongStatus`.

- [ ] **Step 1: Write the failing test**

```tsx
// web/src/routes/index.test.tsx
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { HomePage } from './index'

vi.mock('../lib/api', () => ({
  listSongs: vi.fn().mockResolvedValue({
    songs: [{ id: 1, title: 'Eye of the Tiger', artist: 'Survivor', status: 'ready', hasVersion: true, slug: 'eye-of-the-tiger', addedByName: '', addedByEmail: '', updatedAt: '', addedAt: '' }],
    statuses: ['ready'],
    addedBy: [],
  }),
  getWhoami: vi.fn().mockResolvedValue({ loggedIn: false, role: '' }),
}))

describe('HomePage', () => {
  it('renders the song list', async () => {
    render(<HomePage songs={[{ id: 1, title: 'Eye of the Tiger', artist: 'Survivor', status: 'ready', hasVersion: true, slug: 'eye-of-the-tiger', addedByName: '', addedByEmail: '', updatedAt: '', addedAt: '' }]} statuses={['ready']} addedBy={[]} viewerIsSuperadmin={false} />)
    expect(await screen.findByText('Eye of the Tiger')).toBeInTheDocument()
  })
})
```

Structure `index.tsx` so `HomePage` is an exported, props-driven component separate from the route's `loader` — that's what makes it testable without mocking TanStack Router's route context, and mirrors `homeContent`/`homeTable` being plain functions in `internal/web/home.go` rather than baked into the handler.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && bun run test -- routes/index`
Expected: FAIL (`HomePage` not exported yet).

- [ ] **Step 3: Implement**

Build `web/src/routes/index.tsx` with:
- A `loader` that reads the request's search params, calls `listSongs(queryString, cookie)` and `getWhoami(cookie)` in parallel.
- An exported `HomePage` component taking `{songs, statuses, addedBy, viewerIsSuperadmin}` props, rendering the same table structure as `homeTable` (`internal/web/home.go:248-276`) — title link to `/songs/{slug}`, status column (plain text or inline `<select>` per `viewerIsSuperadmin`), sortable column headers using TanStack Router's `<Link search={...}>` instead of htmx `hx-get`.
- A server function (`createServerFn` from `@tanstack/react-start`) wrapping `setSongStatus`, invoked from the status `<select>`'s `onChange`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bun run test -- routes/index`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/routes/index.tsx web/src/routes/index.test.tsx
git commit -m "Add home page route"
```

---

### Task 14: Song show route

**Files:**
- Create: `web/src/routes/songs.$slug.tsx`
- Create: `web/src/routes/songs.$slug.test.tsx`

**Interfaces:**
- Consumes: `getSong`, `getWhoami` (Task 12), `<Transcription />` (Task 11).
- Produces: `Route` rendering title/byline/key, the `Transcription` component, and (superadmin only) an "Edit" link pointing at `/old/songs/{id}/edit` (edit hasn't moved to React yet — see design's Rollout section) and a "▶ Play" link pointing at `/old/songs/{slug}/play`, matching `songShowContent`'s affordances (`internal/web/song_show.go:93-113`) except those two links now point under `/old` since that's where those pages actually live post-Task-1.

- [ ] **Step 1: Write the failing test**

```tsx
// web/src/routes/songs.$slug.test.tsx
import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SongShowPage } from './songs.$slug'

describe('SongShowPage', () => {
  it('renders title, key, and transcription', () => {
    render(
      <SongShowPage
        song={{ id: 1, title: 'Eye of the Tiger', artist: 'Survivor', slug: 'eye-of-the-tiger', key: 'Cm', hasVersion: true, blocks: [] }}
        viewerIsSuperadmin={false}
      />
    )
    expect(screen.getByText('Eye of the Tiger')).toBeInTheDocument()
    expect(screen.getByText('Cm', { exact: false })).toBeInTheDocument()
  })

  it('shows an Edit link under /old for superadmins only', () => {
    const song = { id: 1, title: 'X', artist: '', slug: 'x', key: '', hasVersion: true, blocks: [] }
    const { rerender } = render(<SongShowPage song={song} viewerIsSuperadmin={false} />)
    expect(screen.queryByText('Edit')).not.toBeInTheDocument()

    rerender(<SongShowPage song={song} viewerIsSuperadmin={true} />)
    expect(screen.getByText('Edit').closest('a')).toHaveAttribute('href', '/old/songs/1/edit')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && bun run test -- songs.\$slug`
Expected: FAIL (module doesn't exist).

- [ ] **Step 3: Implement**

Build `web/src/routes/songs.$slug.tsx` with a `loader` calling `getSong(params.slug, cookie)` + `getWhoami(cookie)`, redirecting (TanStack Router's `redirect()`) if the API 404s, and an exported `SongShowPage` component matching `songShowContent`'s structure (`internal/web/song_show.go:93-113`): `H1` title, byline, key line, conditional Edit/Play links (pointed at `/old/...`), `<TransposeControls key={song.key} />` (Task 16) when `song.hasVersion`, and `<Transcription blocks={song.blocks} />` in place of `renderTranscriptionHTML`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bun run test -- songs.\$slug`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/routes/songs.\$slug.tsx web/src/routes/songs.\$slug.test.tsx
git commit -m "Add song show route"
```

---

### Task 15: Offline data layer + offline shell route

**Files:**
- Create: `web/src/lib/offline-db.ts`
- Create: `web/src/routes/offline-shell.tsx`
- Modify: `static/sw.js`

**Interfaces:**
- Consumes: the existing IndexedDB schema from `static/js/offline-db.js` (`tabitha-offline` DB, `songs`/`meta` stores, `songs` keyed by `slug`) — read that file's exact store/DB names again before writing the TS port so the two agree byte-for-byte (the browser's IndexedDB doesn't care which code wrote to it, but a version/name mismatch would make the TS side unable to see records `offline-sync.js` already wrote, or vice versa).
- Produces: `getOfflineSong(slug: string): Promise<OfflineSongRecord | undefined>` (`web/src/lib/offline-db.ts`). `offline-shell` route: reads `?slug=` from its own URL, calls `getOfflineSong`, renders via `<Transcription />` client-side only (no SSR — this route is only ever reached fully offline).

- [ ] **Step 1: Implement `offline-db.ts`**

```ts
// web/src/lib/offline-db.ts
//
// TS reader for the IndexedDB store static/js/offline-db.js writes —
// same DB name/version/store names, read-only from this side (writes
// stay in offline-sync.js, unchanged by this project). Keep these
// constants in sync with static/js/offline-db.js by hand; they're two
// files by necessity (one runs in the service worker via
// importScripts(), which can't import an ES module).

const OFFLINE_DB_NAME = 'tabitha-offline'
const OFFLINE_DB_VERSION = 1
const OFFLINE_SONGS_STORE = 'songs'

export interface OfflineSongRecord {
  slug: string
  id: number
  title: string
  artist: string
  key: string
  hasVersion: boolean
  blocks: unknown[]
  playHtml: string
  contentHash: string
}

export function getOfflineSong(slug: string): Promise<OfflineSongRecord | undefined> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(OFFLINE_DB_NAME, OFFLINE_DB_VERSION)
    req.onsuccess = () => {
      const tx = req.result.transaction(OFFLINE_SONGS_STORE, 'readonly')
      const getReq = tx.objectStore(OFFLINE_SONGS_STORE).get(slug)
      getReq.onsuccess = () => resolve(getReq.result)
      getReq.onerror = () => reject(getReq.error)
    }
    req.onerror = () => reject(req.error)
  })
}
```

- [ ] **Step 2: Implement the offline-shell route**

```tsx
// web/src/routes/offline-shell.tsx
//
// Precached (see APP_SHELL in static/sw.js) and served by the service
// worker for a song-show request made fully offline (static/sw.js's
// serveFromOfflineSnapshot). Client-only: reads the slug the service
// worker embedded in the query string, pulls the song's JSON from
// IndexedDB, and renders with the exact same Transcription component
// the live SSR page uses — no server round-trip possible here by
// definition (there's no network).
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Transcription } from '../components/Transcription'
import { getOfflineSong, type OfflineSongRecord } from '../lib/offline-db'

export const Route = createFileRoute('/offline-shell')({
  component: OfflineShellPage,
  ssr: false,
})

function OfflineShellPage() {
  const [song, setSong] = useState<OfflineSongRecord | null>(null)
  const [notFound, setNotFound] = useState(false)

  useEffect(() => {
    const slug = new URLSearchParams(window.location.search).get('slug')
    if (!slug) {
      setNotFound(true)
      return
    }
    getOfflineSong(slug).then((record) => {
      if (!record) {
        setNotFound(true)
        return
      }
      setSong(record)
    })
  }, [])

  if (notFound) {
    return <p>You're offline, and this song hasn't been saved for offline viewing yet.</p>
  }
  if (!song) {
    return null
  }
  return (
    <div>
      <h1>{song.title}</h1>
      {song.artist && <p className="byline">As performed by {song.artist}</p>}
      {song.key && <p className="key">Key: <b>{song.key.toUpperCase()}</b></p>}
      <Transcription blocks={song.blocks as never} />
    </div>
  )
}
```

- [ ] **Step 3: Update the service worker**

In `static/sw.js`, change `serveFromOfflineSnapshot`'s show-page branch (currently `showMatch` → `respondFromStoredSong(showMatch[1], "html")`, around line 163-166) to serve the precached offline-shell page instead of a stored HTML field, redirecting the shell to carry the slug:

```js
var showMatch = /^\/songs\/([^/]+)\/?$/.exec(path);
if (showMatch) {
  return caches.match("/offline-shell").then(function (shell) {
    if (!shell) {
      return offlineFallbackResponse();
    }
    return shell;
  });
}
```

The shell itself reads `?slug=` client-side (Step 2), but this fetch is for `/songs/{slug}` with no query string — add a small inline redirect at the very top of `OfflineShellPage`'s effect, or (simpler, no redirect needed) have the shell read the slug from `document.referrer`-independent state by instead reading it from `window.location.pathname` when served *as* `/songs/{slug}` rather than `/offline-shell?slug=...`. Since the service worker controls exactly what's served for that URL, the simplest correct approach: serve the precached shell HTML in response to the `/songs/{slug}` request itself (browser URL bar stays on `/songs/{slug}}`, no redirect), and change `offline-shell.tsx`'s slug-reading to `window.location.pathname.match(/^\/songs\/([^/]+)/)`. Update Step 2's `useEffect` accordingly:

```ts
const slug = /^\/songs\/([^/]+)/.exec(window.location.pathname)?.[1]
```

And drop the `?slug=` framing from this task's description — the shell infers its slug from whatever URL the browser is actually showing, which is always `/songs/{slug}` when the service worker serves it.

Also add the shell's built JS/CSS output path(s) to `APP_SHELL` in `static/sw.js` (exact filenames depend on Task 10's Vite build output — check `web/dist` or wherever Start's client build emits static assets after running `bun run build`, and reference the versioned output the same way `editor.js`/`editor.css` are referenced elsewhere).

- [ ] **Step 4: Manual verification**

Run: `cd web && bun run build && bun run start` (or dev), visit a digested song's page online once (so its JSON is synced into IndexedDB per the existing `offline-sync.js` flow), then use browser devtools to go offline and reload that song's URL. Expected: the offline-shell content renders (title, key, transcription) instead of the "hasn't been saved for offline" fallback.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/offline-db.ts web/src/routes/offline-shell.tsx static/sw.js
git commit -m "Serve song show offline via client-rendered shell + IndexedDB JSON"
```

---

### Task 16: Transpose controls on the new song show page

**Files:**
- Create: `web/src/lib/transpose.ts`
- Create: `web/src/lib/transpose.test.ts`
- Create: `web/src/components/TransposeControls.tsx`
- Modify: `web/src/routes/songs.$slug.tsx`

**Interfaces:**
- Produces: `wrapSemitones(n: number): number`, `transposeChordToken(token: string, semitones: number, useFlats: boolean): string`, `parseKey(text: string): {pc: number, isMinor: boolean}`, `spellKey(key: {pc: number, isMinor: boolean}): string` (`web/src/lib/transpose.ts`, direct TS port of the same-named functions in `static/js/transpose.js:52-134`). `<TransposeControls songKey={string} />` (`web/src/components/TransposeControls.tsx`) — same markup as `transposeControlsNode` (`internal/web/transcription_render.go:179-185`: `.transpose-controls` div, `.transpose-down`/`.transpose-key`/`.transpose-up`), wired to a `useEffect` that re-applies the transpose on mount and on click, operating on `.chord` spans within the same page via direct DOM text mutation (matching the original's approach — this is deliberately not routed through React state/props, since it's a pure post-render string rewrite over already-rendered chord spans, same as the original vanilla-JS version).

Note on duplication: `static/js/transpose.js` stays completely unchanged (still used by `/old`'s show/play pages and Play mode, none of which are rebuilt in this plan) — this task is a parallel TS port for the new React page, not a shared module, because the original is a classic script relying on per-htmx-swap re-execution, which doesn't apply to a React-routed page. A future project (#2, unifying show/play/edit) is the natural point to collapse these back into one implementation once Play mode also moves to React.

- [ ] **Step 1: Write the failing test**

Port the interesting cases directly from `static/js/transpose.js`'s own logic (there's no existing Go/JS test file for this vanilla script today — check `e2e/tests/transpose.spec.ts` first, since a Playwright e2e test for the *existing* transpose feature already exists and is a good source for which behaviors matter):

```ts
// web/src/lib/transpose.test.ts
import { describe, expect, it } from 'vitest'
import { wrapSemitones, transposeChordToken, parseKey, spellKey } from './transpose'

describe('wrapSemitones', () => {
  it('wraps to the shortest path within (-6, 6]', () => {
    expect(wrapSemitones(12)).toBe(0)
    expect(wrapSemitones(13)).toBe(1)
    expect(wrapSemitones(7)).toBe(-5)
    expect(wrapSemitones(-7)).toBe(5)
  })
})

describe('transposeChordToken', () => {
  it('transposes a simple major chord up', () => {
    expect(transposeChordToken('C', 2, false)).toBe('D')
  })
  it('preserves quality/extension text', () => {
    expect(transposeChordToken('Cmaj7', 2, false)).toBe('Dmaj7')
  })
  it('transposes a slash chord bass note too', () => {
    expect(transposeChordToken('C/E', 2, false)).toBe('D/F#')
  })
  it('leaves repeat markers and bar delimiters untouched', () => {
    expect(transposeChordToken('x4', 2, false)).toBe('x4')
    expect(transposeChordToken('|', 2, false)).toBe('|')
  })
  it('transposes bare notes inside a parenthesized group', () => {
    expect(transposeChordToken('(/F /F G)', 2, false)).toBe('(/G /G A)')
  })
})

describe('parseKey / spellKey', () => {
  it('round-trips a flat major key', () => {
    expect(spellKey(parseKey('Bb'))).toBe('Bb')
  })
  it('round-trips a sharp minor key', () => {
    expect(spellKey(parseKey('F#m'))).toBe('F#m')
  })
  it('defaults to C major for an empty/unrecognized key', () => {
    expect(parseKey('')).toEqual({ pc: 0, isMinor: false })
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && bun run test -- transpose`
Expected: FAIL (`./transpose` module doesn't exist).

- [ ] **Step 3: Implement `transpose.ts`**

Direct TS port of `static/js/transpose.js:18-134` (`NOTE_TO_PC`, `SHARP_NAMES`/`FLAT_NAMES`, `FLAT_MAJOR_PCS`/`FLAT_MINOR_PCS`, `CHORD_RE`, `PAREN_NOTE_RE`, `parseNote`, `spellNote`, `transposeNote`, `transposeParenGroup`, `transposeChordToken`, `parseKey`, `keyUsesFlats`, `spellKey`, `wrapSemitones`) — same regexes, same lookup tables, same algorithm, typed. Export every function listed in "Interfaces" above plus `keyUsesFlats` (needed internally by the component in Step 5).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd web && bun run test -- transpose`
Expected: PASS.

- [ ] **Step 5: Implement `TransposeControls.tsx`**

Port `initTransposeControls`/`apply`/the button handlers (`static/js/transpose.js:172-231`) as a React component with a `useEffect`. Drop `updateTransposeLinks` (`static/js/transpose.js:161-170`) — that function exists only to thread the `?t=` param into the Show↔Play navigation links, which don't apply here since Play mode isn't part of this page yet (it's still the separate `/old/songs/{slug}/play` gomponents page, unaffected by this page's transpose state). Keep `TRANSPOSE_PARAM`/URL-param read-on-mount and `history.replaceState` on each step, since that part (deep-linkable/refreshable transpose state) has nothing to do with Show↔Play linking:

```tsx
// web/src/components/TransposeControls.tsx
import { useEffect, useRef, useState } from 'react'
import { parseKey, keyUsesFlats, spellKey, transposeChordToken, wrapSemitones } from '../lib/transpose'

const TRANSPOSE_PARAM = 't'

function readInitialSemitones(): number {
  const raw = new URLSearchParams(window.location.search).get(TRANSPOSE_PARAM)
  const n = parseInt(raw ?? '', 10)
  return Number.isNaN(n) ? 0 : wrapSemitones(n)
}

function withTransposeParam(href: string, semitones: number): string {
  const url = new URL(href, window.location.href)
  if (semitones === 0) {
    url.searchParams.delete(TRANSPOSE_PARAM)
  } else {
    url.searchParams.set(TRANSPOSE_PARAM, String(semitones))
  }
  return url.pathname + url.search + url.hash
}

export function TransposeControls({ songKey }: { songKey: string }) {
  const [semitones, setSemitones] = useState(0)
  const keyElRef = useRef<HTMLSpanElement>(null)
  const baseKey = parseKey(songKey)
  const knownKey = songKey !== ''

  useEffect(() => {
    setSemitones(readInitialSemitones())
  }, [])

  useEffect(() => {
    var chordEls = document.querySelectorAll<HTMLElement>('.chord')
    chordEls.forEach((el) => {
      if (el.dataset.original === undefined) {
        el.dataset.original = el.textContent ?? ''
      }
    })
    if (semitones === 0) {
      chordEls.forEach((el) => {
        el.textContent = el.dataset.original ?? ''
      })
      if (keyElRef.current) keyElRef.current.textContent = knownKey ? songKey : '0'
    } else {
      const current = { pc: baseKey.pc + semitones, isMinor: baseKey.isMinor }
      const useFlats = keyUsesFlats(current)
      chordEls.forEach((el) => {
        el.textContent = transposeChordToken(el.dataset.original ?? '', semitones, useFlats)
      })
      if (keyElRef.current) {
        keyElRef.current.textContent = knownKey ? spellKey(current) : (semitones > 0 ? '+' : '') + semitones
      }
    }
    window.history.replaceState(null, '', withTransposeParam(window.location.href, semitones))
  }, [semitones, songKey])

  return (
    <div className="transpose-controls" data-key={songKey}>
      <button
        type="button"
        className="transpose-down"
        aria-label="Transpose down a semitone"
        onClick={() => setSemitones((s) => wrapSemitones(s - 1))}
      >
        −
      </button>
      <span className="transpose-key" ref={keyElRef}>{knownKey ? songKey : '0'}</span>
      <button
        type="button"
        className="transpose-up"
        aria-label="Transpose up a semitone"
        onClick={() => setSemitones((s) => wrapSemitones(s + 1))}
      >
        +
      </button>
    </div>
  )
}
```

- [ ] **Step 6: Wire it into the song show route**

In `web/src/routes/songs.$slug.tsx`, render `<TransposeControls songKey={song.key} />` when `song.hasVersion`, directly above the `<Transcription />` component — matching `songShowContent`'s ordering (`internal/web/transcription_render.go` / `internal/web/song_show.go:107-109`: transpose controls, then the rendered transcription).

- [ ] **Step 7: Manual verification**

Run: `cd web && bun run dev`, open a digested song's show page, click the `+`/`−` buttons, and confirm the chord spans update and the URL gains/loses `?t=N` as expected. Reload with `?t=2` in the URL and confirm the page opens already transposed.

- [ ] **Step 8: Commit**

```bash
git add web/src/lib/transpose.ts web/src/lib/transpose.test.ts web/src/components/TransposeControls.tsx web/src/routes/songs.\$slug.tsx
git commit -m "Port transpose controls to the new song show page"
```

---

### Task 17: Docker + stack wiring

**Files:**
- Modify: `Dockerfile`
- Modify: `oracle/services/tabitha/stack.yml` (sibling repo — same machine, path `~/Development/articles/oracle`)

**Interfaces:**
- Produces: a `tabitha-web` service in the docker stack, built from a new `web-build`/`web-runtime` stage in this repo's `Dockerfile`, joined to `gojake-net` only (no `proxy-net`, no Traefik labels — per the design's "no public route of its own").

- [ ] **Step 1: Add a Bun build+runtime stage to `Dockerfile`**

Add after the existing `editor-build` stage, before the Go `build` stage:

```dockerfile
FROM oven/bun:1-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/bun.lockb* ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build
```

And a final runtime image for it (separate from the existing Go `alpine:3.20` final stage — this Dockerfile now produces two images; check whether the existing single-`Dockerfile`-two-images approach fits this repo's build tooling, or whether a second `web/Dockerfile` is cleaner given `docker buildx build` in `scripts/deploy.sh` currently only builds one tag — if two images are needed, add `web/Dockerfile` as its own file instead of extending the root one, and update `scripts/deploy.sh` to build+push both tags):

```dockerfile
FROM oven/bun:1-alpine AS web-runtime
WORKDIR /app
COPY --from=web-build /src/web/.output ./.output
EXPOSE 3000
CMD ["bun", "run", ".output/server/index.mjs"]
```

(Exact `.output` path depends on Start's actual Vite build output — confirm against what Task 10's `bun run build` actually emits, adjust the `COPY`/`CMD` paths to match.)

- [ ] **Step 2: Update `scripts/deploy.sh`**

```bash
docker buildx build --target production --push --tag jhash14/tabitha:latest . \
  && docker buildx build -f web/Dockerfile --push --tag jhash14/tabitha-web:latest . \
  && ssh deploy@$(oci-ip) 'docker pull jhash14/tabitha:latest && docker pull jhash14/tabitha-web:latest && docker stack deploy -c /home/deploy/stacks/tabitha/stack.yml tabitha'
```

Adjust `--target production` to whatever the existing single-stage `Dockerfile` actually needs (it may not use named final-stage targeting today — check before assuming).

- [ ] **Step 3: Add the service to `oracle/services/tabitha/stack.yml`**

Add alongside the existing `tabitha` service:

```yaml
  tabitha-web:
    image: jhash14/tabitha-web:latest
    networks: [gojake-net]
    deploy:
      replicas: 1
      restart_policy:
        condition: on-failure
    environment:
      TABITHA_API_URL: "http://tabitha:8080"
```

And add `START_SERVICE_URL: "http://tabitha-web:3000"` to the existing `tabitha` service's environment (or secrets, matching how `APP_URL` is already injected there — check whether plain `environment:` or the `secrets:` file-mount pattern is more consistent with the rest of that file before picking one).

- [ ] **Step 4: Manual verification**

Run: `docker buildx build -t tabitha-web:local -f web/Dockerfile .` locally, then `docker run --rm -p 3000:3000 tabitha-web:local` and confirm `curl http://localhost:3000/` returns rendered HTML. Then bring up the full stack locally (`docker compose`/`docker stack deploy` against a local swarm, per whatever this repo's existing local-multi-service testing approach is — there may not be one yet; if so, note that as a gap rather than skipping verification silently) and confirm `curl http://localhost:8080/` (Go's port) returns the proxied Start response.

- [ ] **Step 5: Commit**

```bash
# In this repo:
git add Dockerfile web/Dockerfile scripts/deploy.sh
git commit -m "Add Bun build/runtime stage and deploy wiring for the Start app"
```

```bash
# In ~/Development/articles/oracle (separate repo, separate commit):
cd ~/Development/articles/oracle
git add services/tabitha/stack.yml
git commit -m "Add tabitha-web service to the tabitha stack"
```

---

### Task 18: Extend Playwright e2e coverage

**Files:**
- Modify: `e2e/setup-server.sh`
- Create: `e2e/tests/home.spec.ts`
- Create: `e2e/tests/song-show.spec.ts`
- Create: `e2e/tests/offline.spec.ts`

**Interfaces:**
- Consumes: `cmd/e2eseed`'s existing seeded song (check `cmd/e2eseed/main.go` for its slug/title, reuse rather than seeding a second one).

- [ ] **Step 1: Update `e2e/setup-server.sh`**

Read the file first — it currently builds the editor bundle + Go binary and starts one server. Add building and starting the Start app too (`cd web && bun install && bun run build && bun run start &`, on a fixed port matching what a test-specific `START_SERVICE_URL` points the Go server at), so Playwright's `webServer` config brings up both processes before tests run.

- [ ] **Step 2: Write `e2e/tests/home.spec.ts`**

```ts
import { test, expect } from '@playwright/test'

test('home page lists the seeded song', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Eye of the Tiger')).toBeVisible() // match cmd/e2eseed's actual seeded title
})
```

- [ ] **Step 3: Write `e2e/tests/song-show.spec.ts`**

```ts
import { test, expect } from '@playwright/test'

test('song show page renders the transcription', async ({ page }) => {
  await page.goto('/')
  await page.getByText('Eye of the Tiger').click()
  await expect(page.locator('.transcription')).toBeVisible()
  await expect(page.locator('.chord-word').first()).toBeVisible()
})
```

- [ ] **Step 4: Write `e2e/tests/offline.spec.ts`**

```ts
import { test, expect } from '@playwright/test'

test('song show page still renders after going offline, once visited online', async ({ page, context }) => {
  await page.goto('/')
  await page.getByText('Eye of the Tiger').click()
  await page.waitForSelector('.transcription')

  // Let the background offline sync (static/js/offline-sync.js) finish
  // writing this song into IndexedDB before cutting the network.
  await page.waitForTimeout(2000)

  await context.setOffline(true)
  await page.reload()
  await expect(page.locator('.transcription')).toBeVisible()
})
```

- [ ] **Step 5: Run the suite**

Run: `cd e2e && npm test`
Expected: PASS. If `offline.spec.ts` is flaky on the sync timing, replace the fixed `waitForTimeout` with polling the app's `#offline-status` element (see `static/js/offline-sync.js`'s `setStatus`) for "1/1 downloaded" before going offline.

- [ ] **Step 6: Commit**

```bash
git add e2e/
git commit -m "Extend e2e coverage to the new home/show pages and offline mode"
```

---

## Self-Review Notes

- **Spec coverage:** Architecture (Tasks 8, 10, 17), Routes (Tasks 1–6), Mutations (Task 6, 12–13), Auth bridging (Task 2, 12), Caching — Cloudflare purge extension is *not* a separate task: it's covered by Task 6 reusing `AdminSetSongStatusHandler`'s existing purge call verbatim, and `digest_song`/`toc_sync` jobs already purge `/` and `/songs/{slug}` by URL string, which don't change. Offline & data model (Tasks 9, 15). Transpose continuity (Task 16). Testing (Tasks 1–18 throughout, plus 18 explicitly). Rollout (Task 1's straight cutover, no flag).
- **Backend-language constraint:** enforced throughout — no task touches `internal/db`, `internal/auth`, or `internal/jobs`'s logic; Tasks 2–6 only add JSON-shaping wrappers around existing Go functions.
- **Known duplication, deliberate:** Task 16 ports transpose logic into a second (TS) implementation rather than sharing `static/js/transpose.js`, since that file's classic-script-per-htmx-swap model doesn't fit a React-routed page. Flagged in Task 16 itself as a natural cleanup point for project #2 (unifying show/play/edit), not a defect to fix here.
