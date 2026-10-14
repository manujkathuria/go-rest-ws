-- Migration: 000001_create_users.up.sql
-- Description: Create app.users table with constraints and permissions
-- Executed as app_migrator assuming role app_owner

\set ON_ERROR_STOP on

SET ROLE app_owner;

CREATE TABLE IF NOT EXISTS app.users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Constraints enforcing agreed data contracts
    CONSTRAINT users_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT users_name_trimmed CHECK (name = btrim(name)),
    CONSTRAINT users_email_length CHECK (char_length(email) BETWEEN 3 AND 254),
    CONSTRAINT users_email_canonical CHECK (email = lower(btrim(email))),
    CONSTRAINT users_email_format CHECK (email LIKE '%@%'),
    CONSTRAINT users_email_unique UNIQUE (email)
);

-- Explicitly ensure grants on existing table (in addition to default privileges)
GRANT SELECT, INSERT ON TABLE app.users TO appuser;

RESET ROLE;
