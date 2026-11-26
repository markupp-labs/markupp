-- +goose Up
-- A Autorização guarda a origem do último uso, como a sessão de login (ADR-0061): o
-- /oauth/token e a troca do MCP a atualizam. last_used_at nulo é Autorização sem uso
-- desde então.
ALTER TABLE oauth_authorizations
    ADD COLUMN client_ip    TEXT NOT NULL DEFAULT '',
    ADD COLUMN user_agent   TEXT NOT NULL DEFAULT '',
    ADD COLUMN last_used_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE oauth_authorizations
    DROP COLUMN IF EXISTS last_used_at,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS client_ip;
