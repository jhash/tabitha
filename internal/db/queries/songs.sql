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

-- name: GetSongCurrentVersion :one
SELECT sqlc.embed(songs), sqlc.embed(transcription_versions)
FROM songs
JOIN transcription_versions ON transcription_versions.id = songs.current_version_id
WHERE songs.id = $1;

-- name: ListSongsByTitle :many
SELECT * FROM songs ORDER BY lower(title) ASC, lower(artist) ASC;

-- name: ListSongsByArtist :many
SELECT * FROM songs ORDER BY lower(artist) ASC, lower(title) ASC;

-- name: ListSongsByStatus :many
SELECT * FROM songs ORDER BY lower(status) ASC, lower(title) ASC;

-- name: ListSongsByLastUpdated :many
SELECT * FROM songs ORDER BY updated_at DESC;

-- name: ListSongsByRecentlyAdded :many
SELECT * FROM songs ORDER BY created_at DESC;

-- name: ListSongsByAddedBy :many
SELECT songs.*, users.name AS added_by_name, users.email AS added_by_email
FROM songs
LEFT JOIN users ON users.id = songs.added_by_user_id
ORDER BY lower(users.email) ASC NULLS LAST, lower(songs.title) ASC;
