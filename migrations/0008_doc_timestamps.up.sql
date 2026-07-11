-- The Google Doc's own createdTime/modifiedTime (from the Drive API),
-- fetched during digest_song — distinct from songs.created_at/updated_at,
-- which track when *tabitha* touched the row rather than when Jeff
-- actually wrote or last edited the doc. NULL until a song has been
-- digested at least once; the home page falls back to created_at/
-- updated_at when NULL (manually-added songs never get these set).
ALTER TABLE songs ADD COLUMN doc_created_at TIMESTAMPTZ;
ALTER TABLE songs ADD COLUMN doc_modified_at TIMESTAMPTZ;
