-- Note: this does not restore the original generated expression from
-- source_url's host — that derivation is gone for good once 0011 runs.
ALTER TABLE songs ALTER COLUMN source_site DROP NOT NULL;
ALTER TABLE songs ALTER COLUMN source_site DROP DEFAULT;
