-- source_site is derived from source_url's host, not stored independently:
-- a generated column means it always stays in sync with source_url and
-- backfills every existing row automatically, with no app code needed to
-- populate it. Blank source_url means the song was never linked to an
-- external tab/chord site — "tabitha-spreadsheet" (Jeff's own TOC entry,
-- likely typed or dictated directly into the Google Doc).
ALTER TABLE songs ADD COLUMN source_site TEXT GENERATED ALWAYS AS (
    CASE
        WHEN source_url = '' THEN 'tabitha-spreadsheet'
        ELSE regexp_replace(
            regexp_replace(source_url, '^https?://', ''),
            '^(www\.|tabs\.)?([^/.]+)\..*', '\2'
        )
    END
) STORED;

CREATE INDEX songs_source_site_idx ON songs (source_site);
