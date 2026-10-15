-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CreateUser :exec
INSERT INTO users (id, email, created_at, updated_at)
VALUES ($1, $2, $3, $4);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

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

-- O primeiro usuário da instância nasce junto com o tenant inicial.
-- name: EnsureTenant :exec
INSERT INTO tenants (id, name, created_at) VALUES ($1, $1, $2)
ON CONFLICT DO NOTHING;

-- name: CreateTenant :exec
INSERT INTO tenants (id, name, created_at) VALUES ($1, $2, $3);

-- name: AddMembership :exec
INSERT INTO tenant_memberships (tenant_id, user_id, role, created_at)
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

-- Só roda para o primeiro admin, quando os únicos cofres são os da migração das notas
-- da rc.1: move todos para o tenant dele, e o ON UPDATE CASCADE leva as notas junto.
-- name: AdoptLegacyVaults :exec
WITH moved AS (
    UPDATE vaults SET tenant_id = sqlc.arg(tenant_id) RETURNING id
)
INSERT INTO vault_members (vault_id, user_id, created_at)
SELECT id, sqlc.arg(user_id), sqlc.arg(created_at) FROM moved
ON CONFLICT DO NOTHING;

-- Marca o convite como aceito numa escrita só, e devolve o que a conta herda dele.
-- name: ClaimInvite :one
UPDATE user_invites
SET accepted_at = sqlc.arg(now)
WHERE token_hash = sqlc.arg(token_hash) AND accepted_at IS NULL AND expires_at > sqlc.arg(now)
RETURNING tenant_id, email, role;

-- Serializa a criação de contas para que só o primeiro usuário vire admin.
-- name: LockFirstAdmin :exec
SELECT pg_advisory_xact_lock(sqlc.arg(key)::bigint);

-- Leituras de vínculo rodam antes de existir tenant na sessão, no papel da conexão.
-- name: GetMembership :one
SELECT m.tenant_id, t.name AS tenant_name, m.role
FROM tenant_memberships m JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = $1 AND m.tenant_id = $2;

-- name: ListMemberships :many
SELECT m.tenant_id, t.name AS tenant_name, m.role
FROM tenant_memberships m JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = $1
ORDER BY t.name, m.tenant_id;
