-- +goose Up
-- Cadastro aberto por método (ADR-0059): uma linha por método que o administrador da
-- instância escolheu; método sem linha está aberto. Da instância, sem tenant nem RLS. A API
-- grava no papel markupp_app; o cadastro, que não tem ator, lê como o dono, como faz ao
-- criar a conta.
CREATE TABLE instance_registration_methods (
    method     TEXT PRIMARY KEY CHECK (method ~ '^[a-z0-9-]+$'),
    open       BOOLEAN NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

REVOKE ALL ON instance_registration_methods
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier, markupp_reader;
GRANT SELECT, INSERT, UPDATE ON instance_registration_methods TO markupp_app;

-- +goose Down
DROP TABLE IF EXISTS instance_registration_methods;
