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

-- name: UpdateTranscriptionVersionContent :exec
-- Re-derives content from the already-stored raw_text after a parser
-- classification improvement — no re-fetch from Google needed since
-- raw_text itself doesn't change, only how it's parsed into blocks.
UPDATE transcription_versions SET content = $2 WHERE id = $1;

-- name: ListAllCurrentVersionIDsAndRawText :many
SELECT tv.id, tv.raw_text
FROM transcription_versions tv
JOIN songs s ON s.current_version_id = tv.id;
