CREATE TABLE transcription_versions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    song_id BIGINT NOT NULL REFERENCES songs (id) ON DELETE CASCADE,

    -- Free text, not an enum: 'primary' by default, 'alternate' or similar
    -- later without a migration. See design doc — deliberately not
    -- over-modeled upfront.
    kind TEXT NOT NULL DEFAULT 'primary',

    source TEXT NOT NULL, -- 'google_doc_scrape' | 'manual_edit'

    raw_text TEXT NOT NULL DEFAULT '',
    content JSONB NOT NULL DEFAULT '{"blocks": []}',

    key TEXT,
    capo INTEGER,

    is_current BOOLEAN NOT NULL DEFAULT false,
    created_by BIGINT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX transcription_versions_song_id_idx ON transcription_versions (song_id);

-- At most one current version per song.
CREATE UNIQUE INDEX transcription_versions_one_current_idx
    ON transcription_versions (song_id)
    WHERE is_current;

ALTER TABLE songs
    ADD CONSTRAINT songs_current_version_id_fkey
    FOREIGN KEY (current_version_id) REFERENCES transcription_versions (id)
    ON DELETE SET NULL;
