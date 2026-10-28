-- name: RecordSongPlayed :exec
INSERT INTO song_last_played (song_id) VALUES ($1)
ON CONFLICT (song_id) DO UPDATE SET played_at = now();

-- name: GetSongLastPlayed :one
SELECT played_at FROM song_last_played WHERE song_id = $1;
