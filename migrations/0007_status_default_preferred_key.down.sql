ALTER TABLE songs DROP COLUMN preferred_key;
DROP TRIGGER songs_default_status ON songs;
DROP FUNCTION songs_default_status();
