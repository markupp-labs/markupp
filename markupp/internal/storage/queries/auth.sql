-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateUser :exec
INSERT INTO users (id, tenant_id, email, role, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetUserByID :one
SELECT id, tenant_id, email, role, created_at, updated_at
FROM users
WHERE id = $1;

-- name: CreateUserIdentity :exec
INSERT INTO user_identities (id, user_id, provider, provider_user_id, password_hash, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetUserIdentity :one
SELECT id, user_id, provider, provider_user_id, password_hash, created_at
FROM user_identities
WHERE provider = $1 AND provider_user_id = $2;

-- name: CreateVault :exec
INSERT INTO vaults (id, tenant_id, name, created_at)
VALUES ($1, $2, $3, $4);

-- name: AddVaultMember :exec
INSERT INTO vault_members (vault_id, user_id, created_at)
VALUES ($1, $2, $3);

-- name: SaveRefreshToken :exec
INSERT INTO refresh_tokens (token_hash, user_id, expires_at, revoked, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetRefreshToken :one
SELECT token_hash, user_id, expires_at, revoked, created_at
FROM refresh_tokens
WHERE token_hash = $1;

-- name: ConsumeRefreshToken :one
UPDATE refresh_tokens
SET revoked = TRUE
WHERE token_hash = $1 AND revoked = FALSE AND expires_at > now()
RETURNING user_id;

-- name: RevokeRefreshToken :execrows
UPDATE refresh_tokens
SET revoked = TRUE
WHERE token_hash = $1 AND revoked = FALSE;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens
SET revoked = TRUE
WHERE user_id = $1;

-- name: CreateAuditEvent :exec
INSERT INTO audit_events (id, tenant_id, actor_id, action, target, result, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- clock_timestamp, e não o instante da conta, deixa o cofre criado com o usuário
-- como o mais antigo dele, que é o padrão das notas sem cofre informado.
-- name: JoinTenantVaults :exec
INSERT INTO vault_members (vault_id, user_id, created_at)
SELECT id, sqlc.arg(user_id), clock_timestamp() FROM vaults
WHERE tenant_id = sqlc.arg(tenant_id)
ON CONFLICT DO NOTHING;

-- Marca o convite como aceito numa escrita só, e devolve o que a conta herda dele.
-- name: ClaimInvite :one
UPDATE user_invites
SET accepted_at = sqlc.arg(now)
WHERE token_hash = sqlc.arg(token_hash) AND accepted_at IS NULL AND expires_at > sqlc.arg(now)
RETURNING tenant_id, email, role;
