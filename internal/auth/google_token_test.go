package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

func seedGoogleToken(t *testing.T, q *db.Queries, key []byte, userID int64, accessToken, refreshToken string, expiry time.Time) {
	t.Helper()
	ctx := context.Background()

	encAccess, err := Encrypt(key, accessToken)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	encRefresh, err := Encrypt(key, refreshToken)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	err = q.UpsertGoogleOAuthToken(ctx, db.UpsertGoogleOAuthTokenParams{
		UserID:                userID,
		EncryptedAccessToken:  encAccess,
		EncryptedRefreshToken: encRefresh,
		Scope:                 GoogleOAuthScope,
		Expiry:                pgtype.Timestamptz{Time: expiry, Valid: true},
	})
	if err != nil {
		t.Fatalf("UpsertGoogleOAuthToken() error = %v", err)
	}
}

func TestValidGoogleTokenReturnsStoredTokenWithoutRefreshingWhenStillValid(t *testing.T) {
	q := setupTestQueries(t)
	key := testKey(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	seedGoogleToken(t, q, key, user.ID, "still-good-access-token", "refresh-token", time.Now().Add(time.Hour))

	refreshServerHit := false
	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshServerHit = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer refreshServer.Close()

	cfg := config.Config{GoogleKey: "key", GoogleSecret: "secret"}
	token, err := ValidGoogleToken(ctx, q, cfg, key, oauth2.Endpoint{TokenURL: refreshServer.URL})
	if err != nil {
		t.Fatalf("ValidGoogleToken() error = %v", err)
	}
	if token.AccessToken != "still-good-access-token" {
		t.Errorf("AccessToken = %q, want the stored token unchanged", token.AccessToken)
	}
	if refreshServerHit {
		t.Error("refresh endpoint was hit for a token that wasn't expired")
	}
}

func TestValidGoogleTokenRefreshesAndPersistsWhenExpired(t *testing.T) {
	q := setupTestQueries(t)
	key := testKey(t)
	ctx := context.Background()

	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	seedGoogleToken(t, q, key, user.ID, "stale-access-token", "original-refresh-token", time.Now().Add(-time.Hour))

	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "refreshed-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			// Google often omits refresh_token on refresh responses — the
			// original one must survive in storage either way.
		})
	}))
	defer refreshServer.Close()

	cfg := config.Config{GoogleKey: "key", GoogleSecret: "secret"}
	token, err := ValidGoogleToken(ctx, q, cfg, key, oauth2.Endpoint{TokenURL: refreshServer.URL})
	if err != nil {
		t.Fatalf("ValidGoogleToken() error = %v", err)
	}
	if token.AccessToken != "refreshed-access-token" {
		t.Errorf("AccessToken = %q, want the refreshed token", token.AccessToken)
	}

	stored, err := q.GetMostRecentGoogleOAuthToken(ctx)
	if err != nil {
		t.Fatalf("GetMostRecentGoogleOAuthToken() error = %v", err)
	}
	gotAccess, err := Decrypt(key, stored.EncryptedAccessToken)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if gotAccess != "refreshed-access-token" {
		t.Errorf("persisted access token = %q, want the refreshed one", gotAccess)
	}
	gotRefresh, err := Decrypt(key, stored.EncryptedRefreshToken)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if gotRefresh != "original-refresh-token" {
		t.Errorf("persisted refresh token = %q, want the original preserved (Google omitted a new one)", gotRefresh)
	}
}

func TestValidGoogleTokenFailsWithNoStoredToken(t *testing.T) {
	q := setupTestQueries(t)
	key := testKey(t)

	cfg := config.Config{GoogleKey: "key", GoogleSecret: "secret"}
	if _, err := ValidGoogleToken(context.Background(), q, cfg, key, oauth2.Endpoint{}); err == nil {
		t.Error("expected an error when no Google OAuth token has ever been stored")
	}
}
