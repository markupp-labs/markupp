-- +goose Up
ALTER TABLE users ADD CONSTRAINT users_tenant_id_key UNIQUE (tenant_id, id);

-- Só o hash do token fica no banco. Quem convida é do mesmo tenant do convite.
CREATE TABLE user_invites (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL,
    email        TEXT NOT NULL,
    role         TEXT NOT NULL,
    token_hash   TEXT NOT NULL,
    invited_by   TEXT NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    accepted_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT user_invites_token_hash_key UNIQUE (token_hash),
    CONSTRAINT user_invites_role_check CHECK (role IN ('admin', 'member')),
    CONSTRAINT user_invites_invited_by_fkey FOREIGN KEY (tenant_id, invited_by)
        REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

ALTER TABLE user_invites ENABLE ROW LEVEL SECURITY;
CREATE POLICY user_invites_isolation ON user_invites
    USING (tenant_id = current_setting('markupp.tenant_id', true));

GRANT SELECT, INSERT ON user_invites TO markupp_app;

-- +goose Down
DROP TABLE IF EXISTS user_invites;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_id_key;
