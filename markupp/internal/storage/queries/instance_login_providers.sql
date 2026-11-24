-- name: ListInstanceLoginProviders :many
SELECT p.id, p.name, p.issuer, p.client_id, p.client_secret_sealed, p.enabled, p.updated_at,
       (SELECT count(DISTINCT i.user_id) FROM user_identities i WHERE i.provider = p.id)::int AS linked_accounts
FROM instance_login_providers p
ORDER BY p.created_at, p.id;

-- name: CreateInstanceLoginProvider :execrows
INSERT INTO instance_login_providers (id, name, issuer, client_id, client_secret_sealed, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO NOTHING;

-- clock_timestamp, e não now, para duas gravações na mesma transação mudarem a revisão.
-- name: UpdateInstanceLoginProvider :execrows
UPDATE instance_login_providers
SET name = $2, issuer = $3, client_id = $4, client_secret_sealed = $5, enabled = $6, updated_at = clock_timestamp()
WHERE id = $1;

-- name: DeleteInstanceLoginProvider :execrows
DELETE FROM instance_login_providers WHERE id = $1;

-- name: DeleteLoginProviderIdentities :execrows
DELETE FROM user_identities WHERE provider = $1;

-- name: DeleteInstanceRegistrationMethod :exec
DELETE FROM instance_registration_methods WHERE method = $1;

-- Novo selo do segredo com a chave de cifra ativa: não muda a revisão.
-- name: ResealInstanceLoginProviderSecret :exec
UPDATE instance_login_providers SET client_secret_sealed = sqlc.arg(fresh)
WHERE id = sqlc.arg(id) AND client_secret_sealed = sqlc.arg(old);
