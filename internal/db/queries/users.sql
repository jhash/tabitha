-- name: FindOrCreateUser :one
INSERT INTO users (email, name, avatar_url)
VALUES ($1, $2, $3)
ON CONFLICT (lower(email)) DO UPDATE SET
    name = EXCLUDED.name,
    avatar_url = EXCLUDED.avatar_url,
    updated_at = now()
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower($1);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: PromoteToSuperadmin :execrows
UPDATE users SET role = 'superadmin', updated_at = now() WHERE lower(email) = lower($1);

-- name: ListUsers :many
SELECT * FROM users ORDER BY lower(email) ASC;
