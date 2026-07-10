-- name: UpsertSongFromTOC :one
INSERT INTO songs (
    title, artist, genre, film_show_album, decade, bob_tag, status,
    source_url, notes, transpose_hint
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (lower(title), lower(artist)) DO UPDATE SET
    genre = EXCLUDED.genre,
    film_show_album = EXCLUDED.film_show_album,
    decade = EXCLUDED.decade,
    bob_tag = EXCLUDED.bob_tag,
    status = EXCLUDED.status,
    source_url = EXCLUDED.source_url,
    notes = EXCLUDED.notes,
    transpose_hint = EXCLUDED.transpose_hint,
    updated_at = now()
RETURNING *;

-- name: SetSongGoogleDocID :exec
UPDATE songs SET google_doc_id = $2, updated_at = now() WHERE id = $1;

-- name: SetSongCurrentVersion :exec
UPDATE songs SET current_version_id = $2, updated_at = now() WHERE id = $1;

-- name: GetSongByID :one
SELECT * FROM songs WHERE id = $1;

-- name: GetSongByTitle :one
SELECT * FROM songs WHERE lower(title) = lower($1);

-- name: ListSongIDsWithoutCurrentVersion :many
SELECT id FROM songs WHERE current_version_id IS NULL ORDER BY id ASC LIMIT $1;

-- name: GetSongCurrentVersion :one
SELECT sqlc.embed(songs), sqlc.embed(transcription_versions)
FROM songs
JOIN transcription_versions ON transcription_versions.id = songs.current_version_id
WHERE songs.id = $1;

-- Every ListSongsBy* query below returns the same column set (song columns
-- plus the joined added-by name/email) so the home page table always has
-- every column available no matter which one is sorting it. sqlc can't
-- parametrize ORDER BY, hence one near-identical query per sort column
-- rather than a single dynamic one.

-- name: ListSongsByTitle :many
SELECT songs.*, users.name AS added_by_name, users.email AS added_by_email
FROM songs
LEFT JOIN users ON users.id = songs.added_by_user_id
ORDER BY lower(songs.title) ASC, lower(songs.artist) ASC;

-- name: ListDistinctStatuses :many
SELECT DISTINCT status FROM songs WHERE status <> '' ORDER BY status;

-- name: ListDistinctAddedByUsers :many
SELECT DISTINCT u.name, u.email
FROM songs s
JOIN users u ON u.id = s.added_by_user_id
ORDER BY u.name;
