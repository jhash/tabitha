-- A real enum (not a boolean) because the role set is small, ours to
-- define, and rarely changes — unlike songs.status, which is free text
-- because it's sourced from Jeff's spreadsheet and its vocabulary isn't
-- ours to predict. 'user' is scaffolded now for the future general-login
-- work even though only 'superadmin' is reachable today.
CREATE TYPE user_role AS ENUM ('user', 'superadmin');

CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    role user_role NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_norm_idx ON users (lower(email));
