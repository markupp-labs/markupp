-- +goose Up
-- Cliente público embutido do markupp-mcp em stdio (ADR-0042): existe de fábrica, sem
-- registro, com redirect só em loopback (a porta é ignorada, RFC 8252) e PKCE.
ALTER TABLE oauth_clients DROP CONSTRAINT oauth_clients_source_check;
ALTER TABLE oauth_clients ADD CONSTRAINT oauth_clients_source_check
    CHECK (source IN ('cimd', 'dcr', 'builtin'));

INSERT INTO oauth_clients (id, source, name, redirect_uris)
VALUES ('markupp-mcp', 'builtin', 'markupp-mcp (terminal)',
        ARRAY['http://127.0.0.1/callback', 'http://[::1]/callback']);

-- +goose Down
DELETE FROM oauth_clients WHERE source = 'builtin';
ALTER TABLE oauth_clients DROP CONSTRAINT oauth_clients_source_check;
ALTER TABLE oauth_clients ADD CONSTRAINT oauth_clients_source_check
    CHECK (source IN ('cimd', 'dcr'));
