-- name: CreateTranscriptionVersion :one
INSERT INTO transcription_versions (
    song_id, kind, source, raw_text, content, key, capo, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: ClearCurrentVersionsForSong :exec
UPDATE transcription_versions SET is_current = false WHERE song_id = $1 AND is_current;

-- name: MarkVersionCurrent :exec
UPDATE transcription_versions SET is_current = true WHERE id = $1;

-- name: GetTranscriptionVersion :one
SELECT * FROM transcription_versions WHERE id = $1;

-- name: ListVersionsForSong :many
SELECT * FROM transcription_versions WHERE song_id = $1 ORDER BY created_at DESC;
