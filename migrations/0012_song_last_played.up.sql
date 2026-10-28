CREATE TABLE song_last_played (
    song_id BIGINT PRIMARY KEY REFERENCES songs (id) ON DELETE CASCADE,
    played_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
