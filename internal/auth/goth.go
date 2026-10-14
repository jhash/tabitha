package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/markbates/goth"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// GoogleConfigured reports whether both Google OAuth credentials are present.
// Both goth provider registration and the login/callback routes stay
// disabled until this is true, so a dev environment without credentials
// doesn't crash on startup.
func GoogleConfigured(cfg config.Config) bool {
	return cfg.GoogleKey != "" && cfg.GoogleSecret != ""
}

// CompleteLogin turns a successful goth callback into a tabitha user and
// session: find-or-create the user from their Google profile, persist their
// encrypted OAuth tokens (used later for read-only Drive/Docs access to
// Jeff's docs), and issue a session.
func CompleteLogin(ctx context.Context, q *db.Queries, encryptionKey []byte, gothUser goth.User) (db.User, string, time.Time, error) {
	user, err := q.FindOrCreateUser(ctx, db.FindOrCreateUserParams{
		Email:     gothUser.Email,
		Name:      gothUser.Name,
		AvatarUrl: gothUser.AvatarURL,
	})
	if err != nil {
		return db.User{}, "", time.Time{}, err
	}

	if gothUser.AccessToken != "" {
		if err := storeGoogleToken(ctx, q, encryptionKey, user.ID, gothUser); err != nil {
			return db.User{}, "", time.Time{}, err
		}
	}

	token, expiresAt, err := CreateSession(ctx, q, user.ID)
	if err != nil {
		return db.User{}, "", time.Time{}, err
	}
	return user, token, expiresAt, nil
}

func storeGoogleToken(ctx context.Context, q *db.Queries, encryptionKey []byte, userID int64, gothUser goth.User) error {
	encryptedAccess, err := Encrypt(encryptionKey, gothUser.AccessToken)
	if err != nil {
		return err
	}
	encryptedRefresh, err := Encrypt(encryptionKey, gothUser.RefreshToken)
	if err != nil {
		return err
	}

	var expiry pgtype.Timestamptz
	if !gothUser.ExpiresAt.IsZero() {
		expiry = pgtype.Timestamptz{Time: gothUser.ExpiresAt, Valid: true}
	}

	return q.UpsertGoogleOAuthToken(ctx, db.UpsertGoogleOAuthTokenParams{
		UserID:                userID,
		EncryptedAccessToken:  encryptedAccess,
		EncryptedRefreshToken: encryptedRefresh,
		Scope:                 GoogleOAuthScope,
		Expiry:                expiry,
	})
}

// GoogleDriveReadonlyScope is the Drive/Docs scope tabitha requests for
// reading Jeff's pre-existing docs — read-only, and stays that way:
// nothing in tabitha ever writes to a doc it didn't create itself. goth.User
// has no field carrying the granted scope back from Google, so this
// records the fixed set tabitha always asks for at the authorization URL.
// See GoogleDriveFileScope for the (separate, deliberately narrower)
// capability that does write.
const GoogleDriveReadonlyScope = "https://www.googleapis.com/auth/drive.readonly"

// GoogleDriveFileScope is a per-file scope: it only grants access to
// files tabitha itself creates via the Drive API (e.g. a new Google Doc
// published from a scraped Ultimate Guitar/e-chords chart), never to
// Jeff's existing docs or anything else already in a user's Drive. This
// is what makes "publish a scraped song to a new Google Doc, then sync
// edits back" (see internal/jobs/publish_song_doc.go) safe to add
// without weakening the read-only guarantee above for every other doc.
const GoogleDriveFileScope = "https://www.googleapis.com/auth/drive.file"

// GoogleOAuthScope is the full space-separated scope string requested for
// every Google login.
const GoogleOAuthScope = "email profile " + GoogleDriveReadonlyScope + " " + GoogleDriveFileScope
