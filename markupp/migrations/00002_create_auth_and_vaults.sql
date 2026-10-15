-- +goose Up
CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    email       TEXT NOT NULL,
    role        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT users_tenant_email_key UNIQUE (tenant_id, email),
    CONSTRAINT users_role_check CHECK (role IN ('admin', 'member'))
);

-- Unicidade global por (provider, provider_user_id) garante que credenciais locais
-- identifiquem uma conta unívoca no sistema.
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

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);

CREATE TABLE audit_events (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    actor_id    TEXT,
    action      TEXT NOT NULL,
    target      TEXT NOT NULL,
    result      TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_events_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'tabela audit_events e insert-only: operacoes de UPDATE ou DELETE sao proibidas';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_events_prevent_mutation
BEFORE UPDATE OR DELETE ON audit_events
FOR EACH ROW EXECUTE FUNCTION audit_events_immutable();

-- +goose Down
DROP TRIGGER IF EXISTS audit_events_prevent_mutation ON audit_events;
DROP FUNCTION IF EXISTS audit_events_immutable();
DROP TABLE IF EXISTS audit_events;
DROP INDEX IF EXISTS refresh_tokens_expires_at_idx;
DROP INDEX IF EXISTS refresh_tokens_user_id_idx;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS vault_members;
DROP TABLE IF EXISTS vaults;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS users;
