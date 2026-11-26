-- +goose Up
-- O histórico guarda de onde veio cada evento (ADR-0061). Colunas novas com default não
-- reescrevem as linhas, então o trigger insert-only não dispara.
ALTER TABLE audit_events
    ADD COLUMN client_ip  TEXT NOT NULL DEFAULT '',
    ADD COLUMN user_agent TEXT NOT NULL DEFAULT '';

-- O administrador da instância lê do mais novo para trás.
CREATE INDEX audit_events_occurred_at_idx ON audit_events (occurred_at DESC, id);

-- +goose Down
DROP INDEX IF EXISTS audit_events_occurred_at_idx;
ALTER TABLE audit_events DROP COLUMN IF EXISTS user_agent, DROP COLUMN IF EXISTS client_ip;
