-- +goose Up
-- Prazo de guarda do histórico e do IP escolhido pelo administrador (ADR-0061): uma linha
-- só, da instância, sem tenant nem RLS. days nulo é sem escolha; last_cleanup_at é a última
-- limpeza de qualquer réplica. Só o dono lê e grava.
CREATE TABLE instance_retention (
    singleton       BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    days            INT CHECK (days BETWEEN 1 AND 3650),
    last_cleanup_at TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO instance_retention DEFAULT VALUES;

REVOKE ALL ON instance_retention
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier, markupp_reader;

-- O histórico continua insert-only; só a limpeza do prazo, que liga
-- markupp.audit_retention na própria transação, apaga.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_events_immutable()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('markupp.audit_retention', true) = 'on' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'tabela audit_events e insert-only: operacoes de UPDATE ou DELETE sao proibidas';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_events_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'tabela audit_events e insert-only: operacoes de UPDATE ou DELETE sao proibidas';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
DROP TABLE IF EXISTS instance_retention;
