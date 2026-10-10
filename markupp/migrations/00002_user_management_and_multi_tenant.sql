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

CREATE TABLE user_invites (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    email       TEXT NOT NULL,
    role        TEXT NOT NULL,
    invited_by  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT user_invites_tenant_email_key UNIQUE (tenant_id, email)
);

ALTER TABLE notes ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE notes ADD COLUMN vault_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE notes DROP CONSTRAINT notes_path_key;
ALTER TABLE notes ADD CONSTRAINT notes_tenant_vault_path_key UNIQUE (tenant_id, vault_id, path);

ALTER TABLE notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE notes FORCE ROW LEVEL SECURITY;

CREATE POLICY notes_tenant_isolation_policy ON notes
    FOR ALL
    USING (tenant_id = NULLIF(current_setting('markupp.current_tenant_id', true), ''))
    WITH CHECK (tenant_id = NULLIF(current_setting('markupp.current_tenant_id', true), ''));

-- +goose Down
DROP POLICY IF EXISTS notes_tenant_isolation_policy ON notes;
ALTER TABLE notes NO FORCE ROW LEVEL SECURITY;
ALTER TABLE notes DISABLE ROW LEVEL SECURITY;

ALTER TABLE notes DROP CONSTRAINT IF EXISTS notes_tenant_vault_path_key;
ALTER TABLE notes ADD CONSTRAINT notes_path_key UNIQUE (path);
ALTER TABLE notes DROP COLUMN IF EXISTS vault_id;
ALTER TABLE notes DROP COLUMN IF EXISTS tenant_id;

DROP TABLE IF EXISTS user_invites;
DROP TABLE IF EXISTS users;
