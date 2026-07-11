-- Slugs are computed and deduplicated in Go (internal/slug), not as a
-- generated column — resolving a collision needs to look at other rows'
-- slugs, which a per-row generated expression can't do. Empty string is
-- allowed transiently (existing rows before the one-time backfill runs);
-- the partial unique index only enforces uniqueness on rows that have one.
ALTER TABLE songs ADD COLUMN slug TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX songs_slug_idx ON songs (slug) WHERE slug <> '';
