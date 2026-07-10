-- Blank status read as "nothing's happened yet," which isn't distinct
-- from Jeff's own "Not Done"/"Not done" workflow label in his sheet.
-- "Pending" is tabitha's own default, applied here and to every future
-- insert via trigger (same pattern as added_by_user_id's Jeff default).
UPDATE songs SET status = 'Pending' WHERE status = '';

CREATE FUNCTION songs_default_status() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = '' THEN
        NEW.status := 'Pending';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Fires on UPDATE too, not just INSERT: UpsertSongFromTOC's ON CONFLICT DO
-- UPDATE sets status = EXCLUDED.status directly on every resync, which
-- would silently reset it back to blank whenever Jeff's cell is blank,
-- bypassing an insert-only trigger.
CREATE TRIGGER songs_default_status BEFORE INSERT OR UPDATE ON songs
FOR EACH ROW EXECUTE FUNCTION songs_default_status();

-- The song's preferred performance key — prefilled at digestion time from
-- the doc's own "Key: X" line when it differs from "Original Y" (see
-- transcription_versions.key), editable later by whoever added the song.
ALTER TABLE songs ADD COLUMN preferred_key TEXT NOT NULL DEFAULT '';
