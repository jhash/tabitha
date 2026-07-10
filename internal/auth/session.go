package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jhash/tabitha/internal/db"
)

// SessionCookieName is the cookie tabitha's own session token travels in —
// distinct from gothic's transient _gothic_session cookie used only during
// the OAuth handshake itself.
const SessionCookieName = "tabitha_session"

const sessionDuration = 30 * 24 * time.Hour

// NewSessionToken generates an unguessable, URL-safe session token.
func NewSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generating session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateSession issues a new session for userID, persisted in Postgres.
func CreateSession(ctx context.Context, q *db.Queries, userID int64) (token string, expiresAt time.Time, err error) {
	token, err = NewSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt = time.Now().Add(sessionDuration)
	err = q.CreateSession(ctx, db.CreateSessionParams{
		Token:     token,
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: creating session: %w", err)
	}
	return token, expiresAt, nil
}

// CurrentUser resolves a session token to its user, or an error if the
// token is unknown, expired, or has been logged out.
func CurrentUser(ctx context.Context, q *db.Queries, token string) (db.User, error) {
	user, err := q.GetSessionUser(ctx, token)
	if err != nil {
		return db.User{}, fmt.Errorf("auth: no valid session: %w", err)
	}
	return user, nil
}
