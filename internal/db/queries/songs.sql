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

-- name: CreateSong :one
-- Used by the "+ Song" flow (/songs/new): a superadmin creating a song
-- from scratch rather than via toc_sync's spreadsheet upsert. added_by
-- reflects the actual creator instead of falling through to the
-- Jeff-default trigger (see migration 0006).
INSERT INTO songs (
    title, artist, genre, film_show_album, decade, bob_tag, status,
    source_url, notes, transpose_hint, added_by_user_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

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

-- name: SetSongPreferredKey :exec
UPDATE songs SET preferred_key = $2, updated_at = now() WHERE id = $1;

-- name: SetSongDocTimestamps :exec
-- Deliberately does not touch updated_at — these mirror the Google Doc's
-- own createdTime/modifiedTime, not a tabitha-side content change.
UPDATE songs SET doc_created_at = $2, doc_modified_at = $3 WHERE id = $1;

-- name: GetSongBySlug :one
SELECT * FROM songs WHERE slug = $1;

-- name: ListAllSongSlugs :many
-- Loaded once per slug-assignment pass (backfill or per-song on ingest)
-- to answer "is this slug taken" locally rather than one query per
-- candidate — the whole catalog's slugs easily fit in memory.
SELECT id, slug FROM songs WHERE slug <> '';

-- name: SetSongSlug :exec
UPDATE songs SET slug = $2 WHERE id = $1;

-- name: SetSongStatus :exec
UPDATE songs SET status = $2, updated_at = now() WHERE id = $1;

-- name: SetSongsStatusBulk :exec
UPDATE songs SET status = $2, updated_at = now() WHERE id = ANY($1::bigint[]);

-- name: ListSongSlugsForSitemap :many
SELECT slug, updated_at FROM songs WHERE slug <> '' ORDER BY slug;

-- name: ListSongSlugsForOfflineManifest :many
-- Every song that actually has something to show offline (a digested
-- transcription) — feeds the lightweight catalog manifest
-- static/js/offline-sync.js diffs against IndexedDB to figure out which
-- songs it still needs to download. Undigested songs have nothing to
-- render, so they're excluded the same way the home page hides them by
-- default. No transcription content here — that's fetched per song, one
-- at a time, only for slugs the diff says are missing or stale.
--
-- content_hash (not updated_at) is the diff signal: it's derived straight
-- from what actually gets rendered, so it changes exactly when the
-- rendered output would, with no dependency on every write path
-- remembering to bump updated_at, and no risk of a touch-but-don't-change
-- write triggering a needless re-download.
SELECT
    songs.slug,
    md5(transcription_versions.content::text || coalesce(transcription_versions.key, '')) AS content_hash
FROM songs
JOIN transcription_versions ON transcription_versions.id = songs.current_version_id
WHERE songs.current_version_id IS NOT NULL AND songs.slug <> ''
ORDER BY songs.slug;

-- name: GetSongForOfflineSnapshotBySlug :one
-- Renders one song at a time for the offline download queue (see
-- static/js/offline-sync.js) — deliberately not a bulk query, so
-- downloading the catalog is a series of small, individually retryable
-- requests instead of one big all-or-nothing one. content_hash uses the
-- exact same expression as ListSongSlugsForOfflineManifest, so a client's
-- stored hash and the manifest's hash are always byte-for-byte comparable.
SELECT
    sqlc.embed(songs),
    sqlc.embed(transcription_versions),
    md5(transcription_versions.content::text || coalesce(transcription_versions.key, '')) AS content_hash
FROM songs
JOIN transcription_versions ON transcription_versions.id = songs.current_version_id
WHERE songs.slug = $1 AND songs.current_version_id IS NOT NULL;
