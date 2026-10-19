-- +goose Up
-- Authorization server OAuth 2.1 (ADR-0039). As tabelas são da instância, como a conta
-- (ADR-0035): sem tenant, sem RLS e fora do alcance dos papéis da aplicação. Códigos e
-- tokens ficam só como hash.
CREATE TABLE oauth_clients (
    id                  TEXT PRIMARY KEY,
    source              TEXT NOT NULL CHECK (source IN ('cimd', 'dcr')),
    name                TEXT NOT NULL,
    redirect_uris       TEXT[] NOT NULL CHECK (cardinality(redirect_uris) > 0),
    metadata_expires_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Só o cliente CIMD tem documento em cache com validade.
    CHECK (source = 'cimd' OR metadata_expires_at IS NULL)
);
-- Os registros DCR varrem os clientes dessa origem para expirar e contar os pendentes.
CREATE INDEX oauth_clients_dcr_created_at_idx ON oauth_clients (created_at) WHERE source = 'dcr';

CREATE TABLE oauth_authorizations (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    client_id  TEXT NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    scopes     TEXT[] NOT NULL CHECK (cardinality(scopes) > 0
        AND scopes <@ ARRAY['notas:leitura', 'notas:escrita', 'administracao']::TEXT[]),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
-- Uma única Autorização ativa por conta e cliente; as revogadas ficam como histórico.
CREATE UNIQUE INDEX oauth_authorizations_active_idx ON oauth_authorizations (user_id, client_id)
    WHERE revoked_at IS NULL;
CREATE INDEX oauth_authorizations_client_id_idx ON oauth_authorizations (client_id);

-- Pedido de autorização: nasce na chegada ao /oauth/authorize, recebe a conta na decisão
-- e, quando aprovado, o hash do código e a Autorização criada ou ampliada por ele.
-- code_reused_at marca a reapresentação do código já usado (RFC 9700, 4.4.2): com ela, o
-- par do primeiro uso não é mais gravado.
CREATE TABLE oauth_requests (
    id               TEXT PRIMARY KEY,
    client_id        TEXT NOT NULL REFERENCES oauth_clients (id) ON DELETE CASCADE,
    redirect_uri     TEXT NOT NULL,
    scopes           TEXT[] NOT NULL CHECK (cardinality(scopes) > 0
        AND scopes <@ ARRAY['notas:leitura', 'notas:escrita', 'administracao']::TEXT[]),
    resource         TEXT NOT NULL,
    state            TEXT,
    code_challenge   TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    decided_at       TIMESTAMPTZ,
    user_id          TEXT REFERENCES users (id) ON DELETE CASCADE,
    code_hash        TEXT UNIQUE,
    code_expires_at  TIMESTAMPTZ,
    code_used_at     TIMESTAMPTZ,
    authorization_id TEXT NULL REFERENCES oauth_authorizations (id) ON DELETE CASCADE,
    code_reused_at   TIMESTAMPTZ NULL
);
CREATE INDEX oauth_requests_expires_at_idx ON oauth_requests (expires_at);
CREATE INDEX oauth_requests_client_id_idx ON oauth_requests (client_id);
CREATE INDEX oauth_requests_user_id_idx ON oauth_requests (user_id);
CREATE INDEX oauth_requests_authorization_id_idx ON oauth_requests (authorization_id);

CREATE TABLE oauth_tokens (
    token_hash       TEXT PRIMARY KEY,
    authorization_id TEXT NOT NULL REFERENCES oauth_authorizations (id) ON DELETE CASCADE,
    kind             TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
    resource         TEXT NOT NULL,
    scopes           TEXT[] NOT NULL CHECK (cardinality(scopes) > 0
        AND scopes <@ ARRAY['notas:leitura', 'notas:escrita', 'administracao']::TEXT[]),
    expires_at       TIMESTAMPTZ NOT NULL,
    revoked          BOOLEAN NOT NULL DEFAULT FALSE,
    rotated_at       TIMESTAMPTZ NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX oauth_tokens_authorization_id_idx ON oauth_tokens (authorization_id);
CREATE INDEX oauth_tokens_expires_at_idx ON oauth_tokens (expires_at);

-- Garante o isolamento mesmo se o banco tiver DEFAULT PRIVILEGES concedendo a PUBLIC ou a
-- qualquer papel criado pelas migrações.
REVOKE ALL ON oauth_clients, oauth_authorizations, oauth_requests, oauth_tokens
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier;

-- +goose Down
DROP TABLE IF EXISTS oauth_tokens;
DROP TABLE IF EXISTS oauth_requests;
DROP TABLE IF EXISTS oauth_authorizations;
DROP TABLE IF EXISTS oauth_clients;
