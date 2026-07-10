package auth

import (
	"context"
	"testing"
	"time"

	"github.com/markbates/goth"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

func TestGoogleConfiguredRequiresBothKeyAndSecret(t *testing.T) {
	tests := []struct {
		name   string
		cfg    config.Config
		wantOK bool
	}{
		{"both set", config.Config{GoogleKey: "k", GoogleSecret: "s"}, true},
		{"missing secret", config.Config{GoogleKey: "k"}, false},
		{"missing key", config.Config{GoogleSecret: "s"}, false},
		{"neither set", config.Config{}, false},
	}
	for _, tt := range tests {
		if got := GoogleConfigured(tt.cfg); got != tt.wantOK {
			t.Errorf("%s: GoogleConfigured() = %v, want %v", tt.name, got, tt.wantOK)
		}
	}
}

func TestCompleteLoginCreatesUserAndSession(t *testing.T) {
	q := setupTestQueries(t)
	key := testKey(t)

	gothUser := goth.User{
		Email:     "jhash147@gmail.com",
		Name:      "Jake",
		AvatarURL: "https://example.com/avatar.png",
	}

	user, token, expiresAt, err := CompleteLogin(context.Background(), q, key, gothUser)
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}
	if user.Email != "jhash147@gmail.com" {
		t.Errorf("user.Email = %q", user.Email)
	}
	if token == "" {
		t.Error("expected a non-empty session token")
	}
	if expiresAt.IsZero() {
		t.Error("expected a non-zero session expiry")
	}

	// The session must actually resolve back to this user.
	got, err := CurrentUser(context.Background(), q, token)
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("CurrentUser() = %d, want %d", got.ID, user.ID)
	}
}

func TestCompleteLoginStoresEncryptedTokenWhenAccessTokenPresent(t *testing.T) {
	q := setupTestQueries(t)
	key := testKey(t)
	ctx := context.Background()

	gothUser := goth.User{
		Email:        "jhash147@gmail.com",
		Name:         "Jake",
		AccessToken:  "real-looking-access-token",
		RefreshToken: "real-looking-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	user, _, _, err := CompleteLogin(ctx, q, key, gothUser)
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}

	stored, err := q.GetGoogleOAuthTokenByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetGoogleOAuthTokenByUserID() error = %v", err)
	}

	// Ciphertext, never plaintext, must be what's in the DB.
	if string(stored.EncryptedAccessToken) == gothUser.AccessToken {
		t.Fatal("access token was stored as plaintext")
	}

	decrypted, err := Decrypt(key, stored.EncryptedAccessToken)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != gothUser.AccessToken {
		t.Errorf("decrypted access token = %q, want %q", decrypted, gothUser.AccessToken)
	}
}

func TestCompleteLoginSkipsTokenStorageWhenNoAccessToken(t *testing.T) {
	// Defensive: don't write a bogus empty-ciphertext token row if goth
	// somehow returns a user with no access token.
	q := setupTestQueries(t)
	key := testKey(t)
	ctx := context.Background()

	gothUser := goth.User{Email: "jhash147@gmail.com", Name: "Jake"}
	user, _, _, err := CompleteLogin(ctx, q, key, gothUser)
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}

	if _, err := q.GetGoogleOAuthTokenByUserID(ctx, user.ID); err == nil {
		t.Error("expected no token row when goth.User has no AccessToken, but one was found")
	}
}

func TestCompleteLoginPromotesExistingSuperadminSeamlessly(t *testing.T) {
	// Logging in again shouldn't touch role — only the CLI/UI promote path does.
	q := setupTestQueries(t)
	key := testKey(t)
	ctx := context.Background()

	first, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}
	if _, err := q.PromoteToSuperadmin(ctx, first.Email); err != nil {
		t.Fatalf("PromoteToSuperadmin() error = %v", err)
	}

	user, _, _, err := CompleteLogin(ctx, q, key, goth.User{Email: "jhash147@gmail.com", Name: "Jake"})
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}
	if user.Role != db.UserRoleSuperadmin {
		t.Errorf("Role = %v, want superadmin to survive a subsequent login", user.Role)
	}
}
