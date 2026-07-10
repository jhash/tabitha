-- name: UpsertGoogleOAuthToken :exec
INSERT INTO google_oauth_tokens (
    user_id, encrypted_access_token, encrypted_refresh_token, scope, expiry
) VALUES (
    $1, $2, $3, $4, $5
)
ON CONFLICT (user_id) DO UPDATE SET
    encrypted_access_token = EXCLUDED.encrypted_access_token,
    encrypted_refresh_token = EXCLUDED.encrypted_refresh_token,
    scope = EXCLUDED.scope,
    expiry = EXCLUDED.expiry,
    updated_at = now();

-- name: GetGoogleOAuthTokenByUserID :one
SELECT * FROM google_oauth_tokens WHERE user_id = $1;

-- name: GetMostRecentGoogleOAuthToken :one
-- Background jobs (digest_song) have no "current user" — any superadmin's
-- token doubles as the readonly Drive/Docs credential per the design doc,
-- and in practice there's only ever been one. Most-recently-updated wins
-- if that ever changes.
SELECT * FROM google_oauth_tokens ORDER BY updated_at DESC LIMIT 1;
