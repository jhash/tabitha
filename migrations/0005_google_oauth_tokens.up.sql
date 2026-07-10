-- One stored Google OAuth grant per user. Ingestion jobs run as this
-- identity's readonly Drive/Docs access — see docs/plans design doc.
-- Tokens are encrypted application-side (AES-GCM, key from
-- TOKEN_ENCRYPTION_KEY) before they ever reach a SQL statement, so these
-- columns hold ciphertext, never plaintext.
CREATE TABLE google_oauth_tokens (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    encrypted_access_token BYTEA NOT NULL,
    encrypted_refresh_token BYTEA NOT NULL,
    scope TEXT NOT NULL,
    expiry TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
