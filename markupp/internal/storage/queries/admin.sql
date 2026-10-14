-- Rodam no papel markupp_app: o RLS recorta tudo ao tenant do administrador.

-- name: ListTenantUsers :many
SELECT * FROM users ORDER BY email;

-- name: ListInvites :many
SELECT id, tenant_id, email, role, invited_by, expires_at, accepted_at, created_at
FROM user_invites
ORDER BY created_at DESC, email;

-- Zero linhas quando o email já é de um usuário do tenant.
-- name: CreateInvite :execrows
INSERT INTO user_invites (id, tenant_id, email, role, token_hash, invited_by, expires_at, created_at)
SELECT sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(email), sqlc.arg(role), sqlc.arg(token_hash),
       sqlc.arg(invited_by), sqlc.arg(expires_at), sqlc.arg(created_at)
WHERE NOT EXISTS (SELECT 1 FROM users WHERE email = sqlc.arg(email));
