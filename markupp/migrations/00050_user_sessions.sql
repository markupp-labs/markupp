-- +goose Up
-- A sessão é o login que as renovações continuam (ADR-0061). Guarda a origem do último
-- uso; os refresh dela apontam para ela. Refresh de antes das sessões fica sem sessão e
-- ganha uma na próxima renovação.
CREATE TABLE user_sessions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    client_ip    TEXT NOT NULL DEFAULT '',
    user_agent   TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL,
    ended_at     TIMESTAMPTZ
);

CREATE INDEX user_sessions_user_id_idx ON user_sessions (user_id);

ALTER TABLE refresh_tokens
    ADD COLUMN session_id TEXT REFERENCES user_sessions (id) ON DELETE CASCADE;

CREATE INDEX refresh_tokens_session_id_idx ON refresh_tokens (session_id);

-- Como refresh_tokens: só o dono lê e grava.
REVOKE ALL ON user_sessions
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier, markupp_reader;

-- +goose Down
DROP INDEX IF EXISTS refresh_tokens_session_id_idx;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS user_sessions;
