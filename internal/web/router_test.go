package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/markbates/goth"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

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

	r := NewRouter(config.Config{}, q)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/google", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /auth/google status = %d, want 404 when Google isn't configured", rec.Code)
	}
}

func TestAuthRoutesMountedWithGoogleCredentialsRedirectsToGoogle(t *testing.T) {
	t.Cleanup(goth.ClearProviders)
	q := setupTestQueries(t)

	r := NewRouter(fakeGoogleConfig(), q)
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

	r := NewRouter(config.Config{}, q)
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

	r := NewRouter(config.Config{}, q)
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

	r := NewRouter(fakeGoogleConfig(), q)
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
