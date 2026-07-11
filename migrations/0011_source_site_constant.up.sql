-- source_site is no longer derived from source_url's host — every song's
-- catalog entry comes from the same place (Jeff's TOC spreadsheet), and
-- "which external tab site the transcription content itself came from" is
-- a separate, per-version concept to be tracked later, not a song-level
-- one. Converting the generated column to a plain one preserves its
-- current stored values (dropped and overwritten below) without needing
-- to also touch the index.
ALTER TABLE songs ALTER COLUMN source_site DROP EXPRESSION;
ALTER TABLE songs ALTER COLUMN source_site SET DEFAULT 'tabitha-spreadsheet';
ALTER TABLE songs ALTER COLUMN source_site SET NOT NULL;
UPDATE songs SET source_site = 'tabitha-spreadsheet';
