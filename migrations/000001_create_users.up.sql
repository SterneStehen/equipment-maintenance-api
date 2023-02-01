BEGIN;

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    full_name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_not_empty CHECK (email <> ''),
    CONSTRAINT users_email_normalized CHECK (email = LOWER(BTRIM(email))),
    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_password_hash_not_empty CHECK (password_hash <> ''),
    CONSTRAINT users_full_name_not_empty CHECK (BTRIM(full_name) <> ''),
    CONSTRAINT users_role_valid CHECK (role IN ('admin', 'dispatcher', 'technician', 'viewer'))
);

CREATE INDEX users_role_idx ON users (role);

COMMIT;
