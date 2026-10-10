-- name: CreateUser :exec
INSERT INTO users (id, tenant_id, email, role, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: CountUsersByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1;

-- name: GetUserByID :one
SELECT id, tenant_id, email, role, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByTenantAndEmail :one
SELECT id, tenant_id, email, role, created_at, updated_at
FROM users
WHERE tenant_id = $1 AND email = $2;

-- name: ListUsersByTenant :many
SELECT id, tenant_id, email, role, created_at, updated_at
FROM users
WHERE tenant_id = $1
ORDER BY email;

-- name: CreateUserInvite :exec
INSERT INTO user_invites (id, tenant_id, email, role, invited_by, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetUserInviteByTenantAndEmail :one
SELECT id, tenant_id, email, role, invited_by, status, created_at
FROM user_invites
WHERE tenant_id = $1 AND email = $2;

-- name: ListUserInvitesByTenant :many
SELECT id, tenant_id, email, role, invited_by, status, created_at
FROM user_invites
WHERE tenant_id = $1
ORDER BY created_at DESC;

-- name: UpdateUserInviteStatus :execrows
UPDATE user_invites
SET status = $3
WHERE tenant_id = $1 AND email = $2;
