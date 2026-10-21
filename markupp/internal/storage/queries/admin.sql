-- Rodam no papel markupp_app: o RLS recorta tudo ao tenant do administrador.

-- name: ListMembers :many
SELECT u.id, u.email, m.role, m.created_at
FROM tenant_memberships m JOIN users u ON u.id = m.user_id
ORDER BY u.email;

-- name: ListInvites :many
SELECT id, tenant_id, email, role, invited_by, expires_at, accepted_at, created_at
FROM user_invites
ORDER BY created_at DESC, email;

-- Zero linhas quando o email já é de um membro do tenant. O tenant fica explícito para
-- a regra não depender do RLS.
-- name: CreateInvite :execrows
INSERT INTO user_invites (id, tenant_id, email, role, token_hash, invited_by, expires_at, created_at)
SELECT sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(email), sqlc.arg(role), sqlc.arg(token_hash),
       sqlc.arg(invited_by), sqlc.arg(expires_at), sqlc.arg(created_at)
WHERE NOT EXISTS (
    SELECT 1 FROM tenant_memberships m JOIN users u ON u.id = m.user_id
    WHERE m.tenant_id = sqlc.arg(tenant_id) AND u.email = sqlc.arg(email)
);

-- Trava os owners do tenant para que duas mudanças simultâneas não o deixem sem owner.
-- name: LockTenantOwners :many
SELECT user_id FROM tenant_memberships WHERE role = 'owner' FOR UPDATE;

-- name: GetMemberRole :one
SELECT role FROM tenant_memberships WHERE user_id = $1 FOR UPDATE;

-- name: UpdateMemberRole :exec
UPDATE tenant_memberships SET role = $2 WHERE user_id = $1;

-- name: DeleteMember :exec
DELETE FROM tenant_memberships WHERE user_id = $1;

-- Revogar só vale para convite ainda sem resposta.
-- name: RevokeInvite :execrows
DELETE FROM user_invites WHERE id = $1 AND accepted_at IS NULL AND declined_at IS NULL;

-- Só o owner passa pela política. O tenant fica explícito para o apagar não depender só
-- do RLS.
-- name: DeleteTenant :execrows
DELETE FROM tenants WHERE id = $1;
