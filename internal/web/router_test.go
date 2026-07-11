package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markbates/goth"
	"github.com/riverqueue/river"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/jobs"
	"github.com/jhash/tabitha/internal/transcription"
)

// superadminSession creates and promotes a user, returning a session token
// that RequireSuperadmin will accept.
func superadminSession(t *testing.T, q *db.Queries) (token string, user db.User) {
	t.Helper()
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	if _, err := q.PromoteToSuperadmin(ctx, user.Email); err != nil {
		t.Fatalf("PromoteToSuperadmin() error = %v", err)
	}
	token, _, err = auth.CreateSession(ctx, q, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	return token, user
}

func doAdminRequest(r http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func doAdminFormRequest(r http.Handler, method, path, token string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// testJobClient builds a real River client against the shared test
// database, so /admin/tools tests can verify an actual job row lands in
// the queue rather than just trusting the handler called the right
// function. Its own pool is independent of setupTestQueries' — two pools
// against the same database is normal and not a race.
func testJobClient(t *testing.T) *river.Client[pgx.Tx] {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres:///tabitha_test?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := jobs.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("migrating river schema: %v", err)
	}

	// Same shared-fixture-db lock the other setup helpers use (see
	// internal/{auth,web,jobs,db}) — go test -p 1 makes this a formality
	// for this specific table, but it's cheap and keeps the pattern
	// uniform in case that ever changes.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("beginning truncate tx: %v", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(72469)"); err != nil {
		t.Fatalf("acquiring test db truncate lock: %v", err)
	}
	if _, err := tx.Exec(ctx, "TRUNCATE river_job RESTART IDENTITY"); err != nil {
		t.Fatalf("truncating river_job: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("committing truncate tx: %v", err)
	}

	client, err := jobs.NewClient(pool, db.New(pool), config.Config{}, nil)
	if err != nil {
		t.Fatalf("creating river client: %v", err)
	}
	return client
}

func fakeGoogleConfig() config.Config {
	return config.Config{
		AppURL:        "https://tabitha.example.com",
		GoogleKey:     "fake-key",
		GoogleSecret:  "fake-secret",
		SessionSecret: "fake-session-secret-fake-session-secret",
	}
}

func TestAuthRoutesNotMountedWithoutGoogleCredentials(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/google", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /auth/google status = %d, want 404 when Google isn't configured", rec.Code)
	}
}

func TestAuthRoutesMountedWithGoogleCredentialsRedirectsToGoogle(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)

	r := NewRouter(fakeGoogleConfig(), q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/google", nil))

	if rec.Code != http.StatusTemporaryRedirect && rec.Code != http.StatusFound {
		t.Fatalf("GET /auth/google status = %d, want a redirect", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "accounts.google.com") {
		t.Errorf("Location = %q, want it to point at accounts.google.com", loc)
	}
}

func TestAdminRouteRequiresSuperadminSession(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /admin without a session status = %d, want 404", rec.Code)
	}
}

func TestAdminRouteServesPageForSuperadmin(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	if _, err := q.PromoteToSuperadmin(ctx, user.Email); err != nil {
		t.Fatalf("PromoteToSuperadmin() error = %v", err)
	}
	token, _, err := auth.CreateSession(ctx, q, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /admin for superadmin status = %d, want 200", rec.Code)
	}
}

func TestAuthLogoutClearsSessionCookieAndServerSideSession(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	token, _, err := auth.CreateSession(ctx, q, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	r := NewRouter(fakeGoogleConfig(), q, nil)
	req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect && rec.Code != http.StatusFound {
		t.Fatalf("GET /auth/logout status = %d, want a redirect", rec.Code)
	}

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			found = true
			if c.MaxAge >= 0 && c.Expires.IsZero() {
				t.Errorf("logout cookie doesn't look cleared: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("logout response didn't clear the tabitha_session cookie")
	}

	if _, err := auth.CurrentUser(ctx, q, token); err == nil {
		t.Error("session was still valid server-side after logout")
	}
}

func TestAdminUsersRouteServesPageForSuperadmin(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodGet, "/admin/users", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/users status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "jhash147@gmail.com") {
		t.Errorf("expected the page to list the seeded user, got: %s", rec.Body.String())
	}
}

func TestAdminPromoteUserPromotesAndRedirects(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()
	token, _ := superadminSession(t, q)

	target, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jeff@example.com", Name: "Jeff"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodPost, fmt.Sprintf("/admin/users/%d/promote", target.ID), token)

	if rec.Code != http.StatusFound {
		t.Fatalf("POST /admin/users/%d/promote status = %d, want 302", target.ID, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/users" {
		t.Errorf("Location = %q, want /admin/users", loc)
	}

	got, err := q.GetUserByEmail(ctx, "jeff@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if got.Role != db.UserRoleSuperadmin {
		t.Errorf("Role = %v, want superadmin after promotion", got.Role)
	}
}

func TestAdminPromoteUserReturns404ForUnknownID(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodPost, "/admin/users/999999/promote", token)

	if rec.Code != http.StatusNotFound {
		t.Errorf("POST /admin/users/999999/promote status = %d, want 404", rec.Code)
	}
}

func TestAdminToolsRouteServesPageForSuperadmin(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodGet, "/admin/tools", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/tools status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/admin/tools/toc-sync") {
		t.Errorf("expected the toc-sync trigger form, got: %s", rec.Body.String())
	}
}

func TestAdminTriggerTocSyncEnqueuesJobAndRedirects(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)
	jobClient := testJobClient(t)

	r := NewRouter(config.Config{}, q, jobClient)
	rec := doAdminRequest(r, http.MethodPost, "/admin/tools/toc-sync", token)

	if rec.Code != http.StatusFound {
		t.Fatalf("POST /admin/tools/toc-sync status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/tools" {
		t.Errorf("Location = %q, want /admin/tools", loc)
	}

	result, err := jobClient.JobList(context.Background(), river.NewJobListParams().Kinds("toc_sync"))
	if err != nil {
		t.Fatalf("JobList() error = %v", err)
	}
	if len(result.Jobs) != 1 {
		t.Errorf("got %d toc_sync jobs queued, want 1", len(result.Jobs))
	}
}

func TestAdminTriggerDigestSongEnqueuesJobForMatchingTitle(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()
	token, _ := superadminSession(t, q)
	jobClient := testJobClient(t)

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Great Balls of Fire", Artist: "Jerry Lee Lewis"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, jobClient)
	rec := doAdminFormRequest(r, http.MethodPost, "/admin/tools/digest-song", token, url.Values{"title": {"Great Balls of Fire"}})

	if rec.Code != http.StatusFound {
		t.Fatalf("POST /admin/tools/digest-song status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/tools" {
		t.Errorf("Location = %q, want /admin/tools", loc)
	}

	result, err := jobClient.JobList(context.Background(), river.NewJobListParams().Kinds("digest_song"))
	if err != nil {
		t.Fatalf("JobList() error = %v", err)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("got %d digest_song jobs queued, want 1", len(result.Jobs))
	}
	var args jobs.DigestSongArgs
	if err := json.Unmarshal(result.Jobs[0].EncodedArgs, &args); err != nil {
		t.Fatalf("unmarshaling job args: %v", err)
	}
	if args.SongID != song.ID {
		t.Errorf("queued SongID = %d, want %d", args.SongID, song.ID)
	}
}

func TestAdminTriggerDigestSongReturns404ForUnknownTitle(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)
	jobClient := testJobClient(t)

	r := NewRouter(config.Config{}, q, jobClient)
	rec := doAdminFormRequest(r, http.MethodPost, "/admin/tools/digest-song", token, url.Values{"title": {"Not A Real Song"}})

	if rec.Code != http.StatusNotFound {
		t.Errorf("POST /admin/tools/digest-song status = %d, want 404", rec.Code)
	}
}

func TestAdminTriggerDigestBatchEnqueuesJobsForUndigestedSongsOnly(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()
	token, _ := superadminSession(t, q)
	jobClient := testJobClient(t)

	undigested, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Undigested Song", Artist: "Someone"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	digested, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Already Digested Song", Artist: "Someone Else"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: digested.ID, Kind: "primary", Source: "google_doc_scrape", RawText: "x", Content: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: digested.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, jobClient)
	rec := doAdminFormRequest(r, http.MethodPost, "/admin/tools/digest-batch", token, url.Values{"limit": {"50"}})

	if rec.Code != http.StatusFound {
		t.Fatalf("POST /admin/tools/digest-batch status = %d, want 302", rec.Code)
	}

	result, err := jobClient.JobList(context.Background(), river.NewJobListParams().Kinds("digest_song"))
	if err != nil {
		t.Fatalf("JobList() error = %v", err)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("got %d digest_song jobs queued, want 1 (only the undigested song)", len(result.Jobs))
	}
	var args jobs.DigestSongArgs
	if err := json.Unmarshal(result.Jobs[0].EncodedArgs, &args); err != nil {
		t.Fatalf("unmarshaling job args: %v", err)
	}
	if args.SongID != undigested.ID {
		t.Errorf("queued SongID = %d, want %d (the undigested one)", args.SongID, undigested.ID)
	}
}

func TestAdminToolsRouteRequiresSuperadminSession(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/tools", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /admin/tools without a session status = %d, want 404", rec.Code)
	}
}

func TestDevLoginRouteNotMountedByDefault(t *testing.T) {
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dev-login", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /dev-login status = %d, want 404 when DevLoginEnabled is false", rec.Code)
	}
}

func TestDevLoginRouteMintsSuperadminSessionWhenEnabled(t *testing.T) {
	q := setupTestQueries(t)

	r := NewRouter(config.Config{DevLoginEnabled: true}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dev-login", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("GET /dev-login status = %d, want 302", rec.Code)
	}

	var token string
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("expected /dev-login to set a session cookie")
	}

	adminRec := doAdminRequest(r, http.MethodGet, "/admin", token)
	if adminRec.Code != http.StatusOK {
		t.Errorf("GET /admin using the dev-login session status = %d, want 200", adminRec.Code)
	}
}

func TestHealthzRouteIsWiredAndPublic(t *testing.T) {
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want 200", rec.Code)
	}
}

func TestMetricsRouteIsWiredAndPublic(t *testing.T) {
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tabitha_http_requests_total") {
		t.Errorf("expected tabitha_http_requests_total in /metrics output, got: %s", rec.Body.String())
	}
}

func TestRobotsTxtRouteIsWiredAndPublic(t *testing.T) {
	q := setupTestQueries(t)

	r := NewRouter(config.Config{AppURL: "https://tabitha.jakehash.com"}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /robots.txt status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Sitemap: https://tabitha.jakehash.com/sitemap.xml") {
		t.Errorf("expected a Sitemap directive, got: %s", rec.Body.String())
	}
}

func TestSitemapRouteIsWiredAndPublic(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()
	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "africa"}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	r := NewRouter(config.Config{AppURL: "https://tabitha.jakehash.com"}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /sitemap.xml status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "https://tabitha.jakehash.com/songs/africa") {
		t.Errorf("expected the song's URL in the sitemap, got: %s", rec.Body.String())
	}
}

func TestSongEditRouteRequires404ForAnonymousViewer(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/songs/%d/edit", song.ID), nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /songs/%d/edit anonymously status = %d, want 404", song.ID, rec.Code)
	}
}

func TestSongEditRouteServesPageForSuperadmin(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()
	token, _ := superadminSession(t, q)

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodGet, fmt.Sprintf("/songs/%d/edit", song.ID), token)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /songs/%d/edit for superadmin status = %d, want 200", song.ID, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Africa") {
		t.Errorf("expected the edit page to mention the song title, got: %s", rec.Body.String())
	}
}

func TestSongEditRouteServesPageBySlug(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()
	token, _ := superadminSession(t, q)

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "africa"}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodGet, "/songs/africa/edit", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /songs/africa/edit for superadmin status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Africa") {
		t.Errorf("expected the edit page to mention the song title, got: %s", rec.Body.String())
	}
}

func TestSongNewRouteRequires404ForAnonymousViewer(t *testing.T) {
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/songs/new", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /songs/new anonymously status = %d, want 404", rec.Code)
	}
}

func TestSongNewRouteServesFormForSuperadmin(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodGet, "/songs/new", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /songs/new for superadmin status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `name="title"`) {
		t.Errorf("expected a title field, got: %s", rec.Body.String())
	}
}

func TestCreateSongRouteRequires404ForAnonymousViewer(t *testing.T) {
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/songs", strings.NewReader(url.Values{"title": {"Brand New"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("POST /songs anonymously status = %d, want 404", rec.Code)
	}
}

func TestCreateSongRouteRejectsBlankTitle(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminFormRequest(r, http.MethodPost, "/songs", token, url.Values{"artist": {"Someone"}})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /songs with blank title status = %d, want 400", rec.Code)
	}
}

func TestCreateSongRouteCreatesSongAssignsSlugAndRedirectsToEditor(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()
	token, user := superadminSession(t, q)

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminFormRequest(r, http.MethodPost, "/songs", token, url.Values{
		"title":  {"Brand New Song"},
		"artist": {"Some Artist"},
	})

	if rec.Code != http.StatusFound {
		t.Fatalf("POST /songs status = %d, want %d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/songs/") || !strings.HasSuffix(loc, "/edit") {
		t.Fatalf("Location = %q, want /songs/{slug}/edit", loc)
	}

	editRec := doAdminRequest(r, http.MethodGet, loc, token)
	if editRec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", loc, editRec.Code)
	}
	if !strings.Contains(editRec.Body.String(), "Brand New Song") {
		t.Errorf("expected the editor page to mention the new song, got: %s", editRec.Body.String())
	}

	song, err := q.GetSongByTitle(ctx, "Brand New Song")
	if err != nil {
		t.Fatalf("GetSongByTitle() error = %v", err)
	}
	if song.Slug == "" {
		t.Error("expected a slug to be assigned")
	}
	if song.AddedByUserID == nil || *song.AddedByUserID != user.ID {
		t.Errorf("AddedByUserID = %v, want %d", song.AddedByUserID, user.ID)
	}
}

func TestAdminSongsRouteRequiresSuperadminSession(t *testing.T) {
	q := setupTestQueries(t)

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/songs", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /admin/songs without a session status = %d, want 404", rec.Code)
	}
}

func TestAdminSongsRouteServesPageForSuperadmin(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodGet, "/admin/songs", token)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/songs for superadmin status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `href="/songs/new"`) {
		t.Errorf("expected a + Song link, got: %s", rec.Body.String())
	}
}

func TestAdminHomeLinksToManageSongs(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	token, _ := superadminSession(t, q)

	r := NewRouter(config.Config{}, q, nil)
	rec := doAdminRequest(r, http.MethodGet, "/admin", token)

	if !strings.Contains(rec.Body.String(), `href="/admin/songs"`) {
		t.Errorf("expected a Manage songs link, got: %s", rec.Body.String())
	}
}

func TestSongShowRouteRedirectsIDToSlugWhenSlugSet(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "africa"}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/songs/%d", song.ID), nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /songs/%d status = %d, want %d", song.ID, rec.Code, http.StatusMovedPermanently)
	}
	if loc := rec.Header().Get("Location"); loc != "/songs/africa" {
		t.Errorf("Location = %q, want %q", loc, "/songs/africa")
	}
}

func TestSongShowRouteServesBySlugDirectly(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "africa"}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/songs/africa", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /songs/africa status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Africa") {
		t.Errorf("expected the song page to render, got: %s", rec.Body.String())
	}
}

func TestSongShowRouteServesByIDWhenNoSlugSetYet(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}

	r := NewRouter(config.Config{}, q, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/songs/%d", song.ID), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /songs/%d status = %d, want 200 (no slug assigned yet)", song.ID, rec.Code)
	}
}

func TestSongShowRouteShowsEditLinkOnlyToSuperadmin(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto"})
	if err != nil {
		t.Fatalf("UpsertSongFromTOC() error = %v", err)
	}
	editLink := fmt.Sprintf("/songs/%d/edit", song.ID)

	r := NewRouter(config.Config{}, q, nil)

	anonRec := httptest.NewRecorder()
	r.ServeHTTP(anonRec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/songs/%d", song.ID), nil))
	if strings.Contains(anonRec.Body.String(), editLink) {
		t.Error("anonymous viewer should not see the edit link")
	}

	token, _ := superadminSession(t, q)
	adminRec := doAdminRequest(r, http.MethodGet, fmt.Sprintf("/songs/%d", song.ID), token)
	if !strings.Contains(adminRec.Body.String(), editLink) {
		t.Errorf("superadmin viewer should see the edit link, got: %s", adminRec.Body.String())
	}
}

// TestGoldenPathSearchViewEditStatusAndPaginateJobs is a single, broad
// walk through the app end to end via real HTTP requests against a real
// router + real Postgres (no browser/JS involved — a server-rendered,
// htmx-boosted app's "client-side behavior" is just another HTTP request,
// so this covers the same ground a browser-driven e2e test would):
// search finds a song by fuzzy title match, the song page renders its
// transcription, a superadmin edits its status inline, then bulk-edits it
// alongside another song, and the paginated admin jobs page still loads
// after a job's been enqueued.
func TestGoldenPathSearchViewEditStatusAndPaginateJobs(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	song, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Africa", Artist: "Toto", Status: "Pending"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	other, err := q.UpsertSongFromTOC(ctx, db.UpsertSongFromTOCParams{Title: "Yesterday", Artist: "The Beatles", Status: "Pending"})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	content, err := transcription.MarshalDocument([]transcription.Block{{Kind: transcription.TextLine, Text: "x"}})
	if err != nil {
		t.Fatalf("MarshalDocument() error = %v", err)
	}
	version, err := q.CreateTranscriptionVersion(ctx, db.CreateTranscriptionVersionParams{
		SongID: song.ID, Kind: "primary", Source: "manual_edit", RawText: "x", Content: content,
	})
	if err != nil {
		t.Fatalf("CreateTranscriptionVersion() error = %v", err)
	}
	if err := q.SetSongCurrentVersion(ctx, db.SetSongCurrentVersionParams{ID: song.ID, CurrentVersionID: &version.ID}); err != nil {
		t.Fatalf("SetSongCurrentVersion() error = %v", err)
	}
	if err := q.SetSongSlug(ctx, db.SetSongSlugParams{ID: song.ID, Slug: "africa"}); err != nil {
		t.Fatalf("SetSongSlug() error = %v", err)
	}

	jobClient := testJobClient(t)
	r := NewRouter(config.Config{}, q, jobClient)

	// 1. Search finds the song by fuzzy title match.
	searchRec := httptest.NewRecorder()
	r.ServeHTTP(searchRec, httptest.NewRequest(http.MethodGet, "/?search=afric&digested=all", nil))
	if searchRec.Code != http.StatusOK || !strings.Contains(searchRec.Body.String(), "Africa") {
		t.Fatalf("search step: status = %d, want 200 with Africa in body, got: %s", searchRec.Code, searchRec.Body.String())
	}

	// 2. The song page renders (canonical slug URL).
	showRec := httptest.NewRecorder()
	r.ServeHTTP(showRec, httptest.NewRequest(http.MethodGet, "/songs/africa", nil))
	if showRec.Code != http.StatusOK {
		t.Fatalf("show step: status = %d, want 200", showRec.Code)
	}

	// 3. A superadmin edits the song's status inline.
	token, _ := superadminSession(t, q)
	statusRec := doAdminFormRequest(r, http.MethodPost, fmt.Sprintf("/admin/songs/%d/status", song.ID), token, url.Values{"status": {"Done"}})
	if statusRec.Code < 200 || statusRec.Code >= 300 {
		t.Fatalf("inline status step: status = %d, want 2xx", statusRec.Code)
	}
	updated, err := q.GetSongByID(ctx, song.ID)
	if err != nil {
		t.Fatalf("GetSongByID() error = %v", err)
	}
	if updated.Status != "Done" {
		t.Errorf("song status = %q after inline edit, want %q", updated.Status, "Done")
	}

	// 4. Bulk-edit both songs to a different status at once.
	bulkRec := doAdminFormRequest(r, http.MethodPost, "/admin/songs/bulk-status", token, url.Values{
		"status": {"Quality Check"},
		"ids":    {fmt.Sprintf("%d", song.ID), fmt.Sprintf("%d", other.ID)},
	})
	if bulkRec.Code < 200 || bulkRec.Code >= 300 {
		t.Fatalf("bulk status step: status = %d, want 2xx", bulkRec.Code)
	}
	for _, id := range []int64{song.ID, other.ID} {
		s, err := q.GetSongByID(ctx, id)
		if err != nil {
			t.Fatalf("GetSongByID(%d) error = %v", id, err)
		}
		if s.Status != "Quality Check" {
			t.Errorf("song %d status = %q after bulk edit, want %q", id, s.Status, "Quality Check")
		}
	}

	// 5. Enqueue a job and confirm the paginated admin jobs page still loads.
	if err := jobs.EnqueueTocSync(ctx, jobClient); err != nil {
		t.Fatalf("EnqueueTocSync() error = %v", err)
	}
	jobsRec := doAdminRequest(r, http.MethodGet, "/admin/jobs", token)
	if jobsRec.Code != http.StatusOK {
		t.Fatalf("admin jobs step: status = %d, want 200", jobsRec.Code)
	}
}
