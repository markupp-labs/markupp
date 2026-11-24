-- +goose Up
-- Provedores de login externo escolhidos pelo administrador da instância (ADR-0060). Da
-- instância, sem tenant nem RLS. A API grava no papel markupp_app; o login, que não tem
-- ator, lê como o dono. O segredo do cliente fica selado como a chave do embedding.
CREATE TABLE instance_login_providers (
    id                   TEXT PRIMARY KEY CHECK (id ~ '^[a-z0-9-]+$' AND id <> 'local'),
    name                 TEXT NOT NULL,
    issuer               TEXT NOT NULL DEFAULT '',
    client_id            TEXT NOT NULL,
    client_secret_sealed TEXT NOT NULL,
    enabled              BOOLEAN NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

REVOKE ALL ON instance_login_providers
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier, markupp_reader;
GRANT SELECT, INSERT, UPDATE ON instance_login_providers TO markupp_app;

-- +goose Down
DROP TABLE IF EXISTS instance_login_providers;
