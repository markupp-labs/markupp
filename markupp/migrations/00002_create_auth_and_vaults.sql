-- +goose Up
CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    email       TEXT NOT NULL,
    role        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT users_tenant_email_key UNIQUE (tenant_id, email)
);

CREATE TABLE user_identities (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_user_id  TEXT NOT NULL,
    password_hash     TEXT,
    created_at        TIMESTAMPTZ NOT NULL,
    CONSTRAINT user_identities_provider_user_key UNIQUE (provider, provider_user_id)
);

CREATE TABLE vaults (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE vault_members (
    vault_id    TEXT NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (vault_id, user_id)
);

CREATE TABLE refresh_tokens (
    token_hash  TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE audit_events (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    actor_id    TEXT,
    action      TEXT NOT NULL,
    target      TEXT NOT NULL,
    result      TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS vault_members;
DROP TABLE IF EXISTS vaults;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS users;
