CREATE TABLE songs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title TEXT NOT NULL,
    artist TEXT NOT NULL DEFAULT '',
    genre TEXT NOT NULL DEFAULT '',
    film_show_album TEXT NOT NULL DEFAULT '',
    decade TEXT NOT NULL DEFAULT '',
    bob_tag TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    transpose_hint TEXT NOT NULL DEFAULT '',
    google_doc_id TEXT NOT NULL DEFAULT '',

    -- FK to transcription_versions added in migration 0003, once that
    -- table exists (avoids a circular forward reference).
    current_version_id BIGINT,

    added_by_user_id BIGINT REFERENCES users (id) ON DELETE SET NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Title/artist clash detection: one canonical song per normalized pair.
CREATE UNIQUE INDEX songs_title_artist_norm_idx ON songs (lower(title), lower(artist));

CREATE INDEX songs_status_idx ON songs (status);
