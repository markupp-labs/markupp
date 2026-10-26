-- +goose Up
-- Cliente público embutido do mu, o CLI de terminal (emenda do ADR-0042): existe de fábrica,
-- sem registro, com redirect só em loopback (a porta é ignorada, RFC 8252) e PKCE.
INSERT INTO oauth_clients (id, source, name, redirect_uris)
VALUES ('markupp-cli', 'builtin', 'markupp-cli (terminal)',
        ARRAY['http://127.0.0.1/callback', 'http://[::1]/callback']);

-- +goose Down
DELETE FROM oauth_clients WHERE id = 'markupp-cli';
