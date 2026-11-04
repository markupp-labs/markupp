-- Gestão de cofres, no papel da aplicação: o RLS recorta o tenant e os cofres do ator e
-- barra o viewer nas escritas (00036).

-- name: ListVaults :many
SELECT id, name, created_at FROM vaults
ORDER BY lower(name), id;

-- name: CreateMemberVault :exec
SELECT markupp_create_vault(sqlc.arg(id)::text, sqlc.arg(name)::text, sqlc.arg(created_at)::timestamptz);

-- name: RenameVault :one
UPDATE vaults SET name = sqlc.arg(name)
WHERE id = sqlc.arg(id)
RETURNING id, name, created_at;

-- name: DeleteVault :execrows
DELETE FROM vaults WHERE id = sqlc.arg(id);

-- name: IsTenantAccount :one
SELECT EXISTS (
    SELECT 1 FROM tenant_memberships
    WHERE user_id = sqlc.arg(user_id) AND tenant_id = current_setting('markupp.tenant_id', true)
) AS member;

-- O papel é o do vínculo no tenant da sessão; tenant_memberships só mostra esse tenant.
-- name: ListVaultMembers :many
SELECT vm.user_id, u.email, tm.role, vm.created_at
FROM vault_members vm
JOIN users u ON u.id = vm.user_id
JOIN tenant_memberships tm ON tm.user_id = vm.user_id
WHERE vm.vault_id = sqlc.arg(vault_id)
ORDER BY u.email, vm.user_id;

-- name: GetVaultMember :one
SELECT vm.user_id, u.email, tm.role, vm.created_at
FROM vault_members vm
JOIN users u ON u.id = vm.user_id
JOIN tenant_memberships tm ON tm.user_id = vm.user_id
WHERE vm.vault_id = sqlc.arg(vault_id) AND vm.user_id = sqlc.arg(user_id);

-- name: RemoveVaultMember :execrows
DELETE FROM vault_members WHERE vault_id = sqlc.arg(vault_id) AND user_id = sqlc.arg(user_id);
