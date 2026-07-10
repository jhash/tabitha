-- name: FindOrCreateArtist :one
INSERT INTO artists (name) VALUES ($1)
ON CONFLICT (lower(name)) DO UPDATE SET name = artists.name
RETURNING *;

-- name: FindOrCreateGenre :one
INSERT INTO genres (name) VALUES ($1)
ON CONFLICT (lower(name)) DO UPDATE SET name = genres.name
RETURNING *;

-- name: SetSongArtistID :exec
UPDATE songs SET artist_id = $2, updated_at = now() WHERE id = $1;

-- name: ClearSongGenres :exec
DELETE FROM song_genres WHERE song_id = $1;

-- name: LinkSongGenre :exec
INSERT INTO song_genres (song_id, genre_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListGenresForSong :many
SELECT g.* FROM genres g
JOIN song_genres sg ON sg.genre_id = g.id
WHERE sg.song_id = $1
ORDER BY lower(g.name);
