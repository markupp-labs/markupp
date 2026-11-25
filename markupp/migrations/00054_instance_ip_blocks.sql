-- +goose Up
-- IPs e faixas que a API recusa antes da autenticação (ADR-0061). Da instância, sem tenant
-- nem RLS. Só o dono lê e grava: a mudança é auditada sem tenant, na mesma transação.
CREATE TABLE instance_ip_blocks (
    id         TEXT PRIMARY KEY,
    cidr       CIDR NOT NULL UNIQUE,
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);

REVOKE ALL ON instance_ip_blocks
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier, markupp_reader;

-- +goose Down
DROP TABLE IF EXISTS instance_ip_blocks;
