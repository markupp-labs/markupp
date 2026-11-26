-- name: ListInstanceAccounts :many
-- O email filtra por parte, sem diferenciar maiúsculas e sem curinga do LIKE.
SELECT id, email, email_verified, created_at, blocked, login_methods, has_last_access, last_access_at
FROM instance_account_summaries
WHERE (sqlc.arg(email)::TEXT = '' OR position(lower(sqlc.arg(email)::TEXT) IN lower(email)) > 0)
  AND (sqlc.arg(status)::TEXT = '' OR (sqlc.arg(status)::TEXT = 'blocked') = blocked)
ORDER BY created_at DESC, id
OFFSET sqlc.arg(row_offset)::INT LIMIT sqlc.arg(row_limit)::INT;

-- name: GetInstanceAccount :one
SELECT id, email, email_verified, created_at, blocked, login_methods, has_last_access, last_access_at
FROM instance_account_summaries
WHERE id = $1;

-- name: ListSoleOwnerTenants :many
-- Os tenants em que a conta é owner e nenhuma outra é: somem com ela.
SELECT t.id, t.name, (SELECT count(*) FROM notes n WHERE n.tenant_id = t.id)::BIGINT AS notes
FROM tenants t
JOIN tenant_memberships m ON m.tenant_id = t.id AND m.user_id = $1 AND m.role = 'owner'
WHERE NOT EXISTS (SELECT 1 FROM tenant_memberships o
                  WHERE o.tenant_id = t.id AND o.role = 'owner' AND o.user_id <> $1)
ORDER BY t.name, t.id;

-- name: SetUserBlocked :execrows
UPDATE users
SET blocked_at = CASE WHEN sqlc.arg(blocked)::BOOLEAN THEN COALESCE(blocked_at, now()) END
WHERE id = sqlc.arg(id);

-- name: RevokeUserOAuthAuthorizations :exec
-- Revoga os apps autorizados da conta e os tokens deles.
WITH revoked AS (
    UPDATE oauth_authorizations a SET revoked_at = now()
    WHERE a.user_id = $1 AND a.revoked_at IS NULL
)
UPDATE oauth_tokens t SET revoked = TRUE
WHERE t.authorization_id IN (SELECT a.id FROM oauth_authorizations a WHERE a.user_id = $1)
  AND t.revoked = FALSE;

-- name: DeleteSoleOwnerTenants :exec
DELETE FROM tenants t
WHERE EXISTS (SELECT 1 FROM tenant_memberships m
              WHERE m.tenant_id = t.id AND m.user_id = $1 AND m.role = 'owner')
  AND NOT EXISTS (SELECT 1 FROM tenant_memberships o
                  WHERE o.tenant_id = t.id AND o.role = 'owner' AND o.user_id <> $1);

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;
