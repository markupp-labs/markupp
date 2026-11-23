-- +goose Up
-- Escolha do provedor de embedding da instância (ADR-0057): uma linha só, da instância,
-- sem tenant nem RLS, como instance_secrets. O administrador da instância grava a escolha
-- pela API, no papel markupp_app; o indexador de embedding a lê e informa, no papel
-- markupp_indexer, se quem hospeda travou o provedor e o estado do modelo.
CREATE TABLE instance_embedding (
    singleton      BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    provider       TEXT NOT NULL DEFAULT 'none' CHECK (provider IN ('none', 'openai-compat', 'local')),
    url            TEXT NOT NULL DEFAULT '',
    model          TEXT NOT NULL DEFAULT '',
    -- Chave do endpoint remoto, cifrada com AES-256-GCM; vazia quando não há.
    api_key_sealed TEXT NOT NULL DEFAULT '',
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    host_locked    BOOLEAN NOT NULL DEFAULT false,
    host_provider  TEXT NOT NULL DEFAULT '',
    model_state    TEXT NOT NULL DEFAULT '' CHECK (model_state IN ('', 'downloading', 'ready', 'error')),
    model_error    TEXT NOT NULL DEFAULT '',
    model_state_at TIMESTAMPTZ
);
INSERT INTO instance_embedding DEFAULT VALUES;

REVOKE ALL ON instance_embedding
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier, markupp_reader;
GRANT SELECT ON instance_embedding TO markupp_app, markupp_indexer;
GRANT UPDATE (provider, url, model, api_key_sealed, updated_at) ON instance_embedding TO markupp_app;
GRANT UPDATE (host_locked, host_provider, model_state, model_error, model_state_at) ON instance_embedding TO markupp_indexer;

-- +goose Down
DROP TABLE IF EXISTS instance_embedding;
