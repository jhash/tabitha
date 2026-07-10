-- pg_trgm powers fuzzy (typo-tolerant) search matching on top of Postgres'
-- built-in full-text search, which alone only matches on word stems.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE artists (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX artists_name_norm_idx ON artists (lower(name));

-- songs.artist (free text from Jeff's spreadsheet) stays as the source of
-- truth for the TOC upsert's dedup key — artist_id is a normalized,
-- best-effort link derived from it, not a replacement.
ALTER TABLE songs ADD COLUMN artist_id BIGINT REFERENCES artists (id) ON DELETE SET NULL;

CREATE TABLE genres (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX genres_name_norm_idx ON genres (lower(name));

-- Many genres per song. Jeff's spreadsheet only has one free-text genre
-- column today, but this is built many-to-many since richer genre data
-- (from his Drive folder layout) may get pulled in later.
CREATE TABLE song_genres (
    song_id BIGINT NOT NULL REFERENCES songs (id) ON DELETE CASCADE,
    genre_id BIGINT NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    PRIMARY KEY (song_id, genre_id)
);

-- Backfill artists from the existing free-text songs.artist column.
INSERT INTO artists (name)
SELECT DISTINCT artist FROM songs WHERE artist <> ''
ON CONFLICT (lower(name)) DO NOTHING;

UPDATE songs SET artist_id = artists.id
FROM artists
WHERE songs.artist <> '' AND lower(songs.artist) = lower(artists.name);

-- Backfill genres from the existing free-text songs.genre column.
INSERT INTO genres (name)
SELECT DISTINCT genre FROM songs WHERE genre <> ''
ON CONFLICT (lower(name)) DO NOTHING;

INSERT INTO song_genres (song_id, genre_id)
SELECT s.id, g.id
FROM songs s
JOIN genres g ON s.genre <> '' AND lower(s.genre) = lower(g.name)
ON CONFLICT DO NOTHING;

-- Jeff's TOC spreadsheet has no "added by" column and may never get one —
-- default every song ingested from it to a standing "Jeff" user rather
-- than leaving added_by_user_id NULL. This user never logs in; it's a
-- placeholder identity, not a real account.
INSERT INTO users (email, name)
VALUES ('jeff@tabitha.local', 'Jeff')
ON CONFLICT (lower(email)) DO NOTHING;

UPDATE songs SET added_by_user_id = (SELECT id FROM users WHERE lower(email) = 'jeff@tabitha.local')
WHERE added_by_user_id IS NULL;

-- Defaults added_by_user_id to Jeff for every future insert too (not just
-- this backfill) — the TOC sheet has no "added by" column and may never
-- get one. A trigger covers every insert path (toc_sync's upsert included)
-- without every call site needing to remember to set it.
CREATE FUNCTION songs_default_added_by() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.added_by_user_id IS NULL THEN
        NEW.added_by_user_id := (SELECT id FROM users WHERE lower(email) = 'jeff@tabitha.local');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER songs_default_added_by BEFORE INSERT ON songs
FOR EACH ROW EXECUTE FUNCTION songs_default_added_by();
