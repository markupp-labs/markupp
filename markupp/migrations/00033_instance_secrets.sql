-- +goose Up
-- Segredos da instância (ADR-0041): o servidor gera cada um na primeira subida e todas as
-- réplicas leem o mesmo valor. A tabela é da instância, como as do OAuth: sem tenant, sem
-- RLS e fora do alcance dos papéis da aplicação.
CREATE TABLE instance_secrets (
    name       TEXT PRIMARY KEY,
    value      TEXT NOT NULL CHECK (value <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Garante o isolamento mesmo se o banco tiver DEFAULT PRIVILEGES concedendo a PUBLIC ou a
-- qualquer papel criado pelas migrações.
REVOKE ALL ON instance_secrets
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier;

-- +goose Down
DROP TABLE IF EXISTS instance_secrets;
