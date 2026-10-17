-- Authorization server OAuth 2.1 (ADR-0038). As tabelas são da instância, sem tenant nem
-- RLS: as queries rodam no papel da conexão. Códigos e tokens só passam como hash.

-- name: GetOAuthClient :one
SELECT * FROM oauth_clients WHERE id = $1;

-- name: CreateOAuthRequest :exec
INSERT INTO oauth_requests (id, client_id, redirect_uri, scopes, resource, state, code_challenge, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetPendingOAuthRequest :one
SELECT * FROM oauth_requests
WHERE id = sqlc.arg(id) AND decided_at IS NULL AND expires_at > sqlc.arg(now)::TIMESTAMPTZ;

-- name: ApproveOAuthRequest :one
UPDATE oauth_requests
SET decided_at = sqlc.arg(decided_at)::TIMESTAMPTZ, user_id = sqlc.arg(user_id)::TEXT,
    code_hash = sqlc.arg(code_hash)::TEXT, code_expires_at = sqlc.arg(code_expires_at)::TIMESTAMPTZ
WHERE id = sqlc.arg(id) AND decided_at IS NULL AND expires_at > sqlc.arg(decided_at)::TIMESTAMPTZ
RETURNING *;

-- name: DenyOAuthRequest :one
UPDATE oauth_requests SET decided_at = sqlc.arg(decided_at)::TIMESTAMPTZ, user_id = sqlc.arg(user_id)::TEXT
WHERE id = sqlc.arg(id) AND decided_at IS NULL AND expires_at > sqlc.arg(decided_at)::TIMESTAMPTZ
RETURNING *;

-- name: GetActiveOAuthAuthorization :one
SELECT * FROM oauth_authorizations WHERE user_id = $1 AND client_id = $2 AND revoked_at IS NULL;

-- Cria a Autorização ativa ou amplia a existente num único comando, o que serializa as
-- aprovações concorrentes da mesma conta para o mesmo cliente. A união segue a ordem de
-- supported, sem repetir.
-- name: UpsertOAuthAuthorization :exec
INSERT INTO oauth_authorizations (id, user_id, client_id, scopes, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(client_id), sqlc.arg(scopes), sqlc.arg(at), sqlc.arg(at))
ON CONFLICT (user_id, client_id) WHERE revoked_at IS NULL DO UPDATE
SET scopes = ARRAY(
        SELECT s.scope FROM unnest(sqlc.arg(supported)::TEXT[]) WITH ORDINALITY AS s(scope, pos)
        WHERE s.scope = ANY (oauth_authorizations.scopes) OR s.scope = ANY (EXCLUDED.scopes)
        ORDER BY s.pos),
    updated_at = EXCLUDED.updated_at;

-- name: RedeemOAuthCode :one
UPDATE oauth_requests SET code_used_at = sqlc.arg(now)::TIMESTAMPTZ
WHERE code_hash = sqlc.arg(code_hash)::TEXT AND code_used_at IS NULL AND code_expires_at > sqlc.arg(now)::TIMESTAMPTZ
RETURNING *;

-- name: GetOAuthRequestByUsedCode :one
SELECT * FROM oauth_requests WHERE code_hash = sqlc.arg(code_hash)::TEXT AND code_used_at IS NOT NULL;

-- name: SaveOAuthToken :exec
INSERT INTO oauth_tokens (token_hash, authorization_id, kind, resource, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: RevokeOAuthAuthorizationTokens :exec
UPDATE oauth_tokens SET revoked = TRUE WHERE authorization_id = $1;
