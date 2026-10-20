-- Authorization server OAuth 2.1 (ADR-0046). As tabelas são da instância, sem tenant nem
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
-- supported, sem repetir. Devolve o ID da Autorização criada ou ampliada.
-- name: UpsertOAuthAuthorization :one
INSERT INTO oauth_authorizations (id, user_id, client_id, scopes, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(client_id), sqlc.arg(scopes), sqlc.arg(at), sqlc.arg(at))
ON CONFLICT (user_id, client_id) WHERE revoked_at IS NULL DO UPDATE
SET scopes = ARRAY(
        SELECT s.scope FROM unnest(sqlc.arg(supported)::TEXT[]) WITH ORDINALITY AS s(scope, pos)
        WHERE s.scope = ANY (oauth_authorizations.scopes) OR s.scope = ANY (EXCLUDED.scopes)
        ORDER BY s.pos),
    updated_at = EXCLUDED.updated_at
RETURNING id;

-- name: SetOAuthRequestAuthorization :exec
UPDATE oauth_requests SET authorization_id = sqlc.arg(authorization_id)::TEXT WHERE id = sqlc.arg(id);

-- name: RedeemOAuthCode :one
UPDATE oauth_requests SET code_used_at = sqlc.arg(now)::TIMESTAMPTZ
WHERE code_hash = sqlc.arg(code_hash)::TEXT AND code_used_at IS NULL AND code_expires_at > sqlc.arg(now)::TIMESTAMPTZ
RETURNING *;

-- Marca o primeiro reuso do código já usado, se o pedido foi decidido a partir de
-- decided_since; outro reuso ou um reuso tardio não casa.
-- name: MarkOAuthCodeReused :one
UPDATE oauth_requests SET code_reused_at = sqlc.arg(now)::TIMESTAMPTZ
WHERE code_hash = sqlc.arg(code_hash)::TEXT AND code_used_at IS NOT NULL AND code_reused_at IS NULL
  AND decided_at >= sqlc.arg(decided_since)::TIMESTAMPTZ
RETURNING *;

-- name: GetUsedOAuthCode :one
SELECT * FROM oauth_requests WHERE code_hash = sqlc.arg(code_hash)::TEXT AND code_used_at IS NOT NULL;

-- name: IsUnreusedOAuthCode :one
SELECT EXISTS (
    SELECT 1 FROM oauth_requests
    WHERE id = sqlc.arg(id) AND authorization_id = sqlc.arg(authorization_id)::TEXT AND code_reused_at IS NULL
);

-- name: SaveOAuthToken :exec
INSERT INTO oauth_tokens (token_hash, authorization_id, kind, resource, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: RevokeOAuthAuthorizationTokens :exec
UPDATE oauth_tokens SET revoked = TRUE WHERE authorization_id = $1;

-- Marca o refresh token como rotacionado; só um refresh válido e ainda não usado casa.
-- name: ConsumeOAuthRefreshToken :one
UPDATE oauth_tokens SET rotated_at = sqlc.arg(now)::TIMESTAMPTZ
WHERE token_hash = sqlc.arg(token_hash)::TEXT AND kind = 'refresh' AND NOT revoked
  AND rotated_at IS NULL AND expires_at > sqlc.arg(now)::TIMESTAMPTZ
RETURNING *;

-- name: GetActiveOAuthAuthorizationByID :one
SELECT * FROM oauth_authorizations WHERE id = $1 AND revoked_at IS NULL;

-- Qualquer token, mesmo revogado ou expirado, com a Autorização dona, mesmo revogada.
-- name: GetOAuthTokenWithAuthorization :one
SELECT sqlc.embed(oauth_tokens), sqlc.embed(oauth_authorizations)
FROM oauth_tokens JOIN oauth_authorizations ON oauth_authorizations.id = oauth_tokens.authorization_id
WHERE oauth_tokens.token_hash = $1;

-- name: ListActiveOAuthAuthorizations :many
SELECT sqlc.embed(oauth_authorizations), oauth_clients.name AS client_name
FROM oauth_authorizations JOIN oauth_clients ON oauth_clients.id = oauth_authorizations.client_id
WHERE oauth_authorizations.user_id = $1 AND oauth_authorizations.revoked_at IS NULL
ORDER BY oauth_authorizations.updated_at DESC;

-- name: RevokeOAuthAuthorization :execrows
UPDATE oauth_authorizations SET revoked_at = sqlc.arg(at)::TIMESTAMPTZ
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND revoked_at IS NULL;

-- Trava a Autorização, revogada ou não, para serializar a gravação do par rotacionado
-- com as revogações dela.
-- name: LockOAuthAuthorization :one
SELECT * FROM oauth_authorizations WHERE id = $1 FOR UPDATE;

-- name: IsUnrevokedOAuthToken :one
SELECT EXISTS (
    SELECT 1 FROM oauth_tokens WHERE token_hash = $1 AND authorization_id = $2 AND NOT revoked
);
