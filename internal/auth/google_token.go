package auth

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// ValidGoogleToken returns a usable Google access token for background
// jobs (digest_song), refreshing and persisting it first if the stored
// one has expired. endpoint is passed explicitly (rather than hardcoding
// golang.org/x/oauth2/google.Endpoint) so tests can point it at a fake
// token server instead of Google's real one.
func ValidGoogleToken(ctx context.Context, q *db.Queries, cfg config.Config, encryptionKey []byte, endpoint oauth2.Endpoint) (*oauth2.Token, error) {
	row, err := q.GetMostRecentGoogleOAuthToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth: no stored Google OAuth token: %w", err)
	}

	accessToken, err := Decrypt(encryptionKey, row.EncryptedAccessToken)
	if err != nil {
		return nil, fmt.Errorf("auth: decrypting access token: %w", err)
	}
	refreshToken, err := Decrypt(encryptionKey, row.EncryptedRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("auth: decrypting refresh token: %w", err)
	}

	stored := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       row.Expiry.Time,
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GoogleKey,
		ClientSecret: cfg.GoogleSecret,
		Endpoint:     endpoint,
	}

	fresh, err := oauthCfg.TokenSource(ctx, stored).Token()
	if err != nil {
		return nil, fmt.Errorf("auth: refreshing google token: %w", err)
	}
	if fresh.AccessToken == stored.AccessToken {
		return fresh, nil
	}

	// Google's refresh response often omits refresh_token since it hasn't
	// changed — don't overwrite the real stored one with an empty string.
	newRefreshToken := fresh.RefreshToken
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}

	encAccess, err := Encrypt(encryptionKey, fresh.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("auth: encrypting refreshed access token: %w", err)
	}
	encRefresh, err := Encrypt(encryptionKey, newRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("auth: encrypting refreshed refresh token: %w", err)
	}

	err = q.UpsertGoogleOAuthToken(ctx, db.UpsertGoogleOAuthTokenParams{
		UserID:                row.UserID,
		EncryptedAccessToken:  encAccess,
		EncryptedRefreshToken: encRefresh,
		Scope:                 row.Scope,
		Expiry:                pgtype.Timestamptz{Time: fresh.Expiry, Valid: !fresh.Expiry.IsZero()},
	})
	if err != nil {
		return nil, fmt.Errorf("auth: persisting refreshed google token: %w", err)
	}
	return fresh, nil
}
