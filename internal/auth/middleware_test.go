package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jhash/tabitha/internal/db"
)

func newAuthedRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	}
	return req
}

func TestRequireSuperadminAllowsSuperadminThrough(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	if _, err := q.PromoteToSuperadmin(ctx, user.Email); err != nil {
		t.Fatalf("PromoteToSuperadmin() error = %v", err)
	}
	token, _, err := CreateSession(ctx, q, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	called := false
	handler := RequireSuperadmin(q)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got, ok := UserFromContext(r.Context())
		if !ok || got.ID != user.ID {
			t.Errorf("UserFromContext() = %v, %v; want user %d in context", got, ok, user.ID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAuthedRequest(t, token))

	if !called {
		t.Fatal("next handler was not called for a valid superadmin session")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireSuperadminBlocksNonSuperadminWith404(t *testing.T) {
	q := setupTestQueries(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "someone@example.com", Name: "Someone"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	token, _, err := CreateSession(ctx, q, user.ID)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	called := false
	handler := RequireSuperadmin(q)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAuthedRequest(t, token))

	if called {
		t.Error("next handler was called for a non-superadmin user")
	}
	// 404, not 403 — don't reveal that /admin exists to non-superadmins.
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (not 403 — shouldn't reveal the route exists)", rec.Code)
	}
}

func TestRequireSuperadminBlocksMissingSessionWith404(t *testing.T) {
	q := setupTestQueries(t)

	called := false
	handler := RequireSuperadmin(q)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAuthedRequest(t, ""))

	if called {
		t.Error("next handler was called with no session cookie at all")
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
