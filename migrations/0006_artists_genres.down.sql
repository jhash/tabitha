DROP TRIGGER songs_default_added_by ON songs;
DROP FUNCTION songs_default_added_by();
DROP TABLE song_genres;
DROP TABLE genres;
ALTER TABLE songs DROP COLUMN artist_id;
DROP TABLE artists;
DROP EXTENSION IF EXISTS pg_trgm;
