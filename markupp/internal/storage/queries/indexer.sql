-- O indexador reserva o cofre no papel da conexão, que enxerga todos os tenants, e
-- só então entra no papel markupp_indexer, preso pelo RLS ao cofre reservado.

-- name: ReservePendingVault :one
SELECT r.vault_id, v.tenant_id
FROM vault_index_reservations r
JOIN vaults v ON v.id = r.vault_id
WHERE EXISTS (
    SELECT 1 FROM notes n
    LEFT JOIN note_search_index i ON i.note_id = n.id AND i.revision = n.revision
    WHERE n.vault_id = r.vault_id AND i.note_id IS NULL
)
ORDER BY r.vault_id
LIMIT 1
FOR UPDATE OF r SKIP LOCKED;

-- name: EnterIndexer :exec
SELECT set_config('role', 'markupp_indexer', true),
       set_config('markupp.tenant_id', sqlc.arg(tenant_id)::text, true),
       set_config('markupp.vault_id', sqlc.arg(vault_id)::text, true);

-- name: ListPendingNotes :many
SELECT n.id, n.content, n.revision
FROM notes n
LEFT JOIN note_search_index i ON i.note_id = n.id AND i.revision = n.revision
WHERE i.note_id IS NULL
ORDER BY n.id;

-- name: UpsertSearchDocument :exec
INSERT INTO note_search_index (note_id, tenant_id, vault_id, revision, document)
VALUES (
    sqlc.arg(note_id),
    current_setting('markupp.tenant_id'),
    current_setting('markupp.vault_id'),
    sqlc.arg(revision),
    to_tsvector(sqlc.arg(search_config)::text::regconfig, sqlc.arg(body)::text)
)
ON CONFLICT (note_id) DO UPDATE SET revision = EXCLUDED.revision, document = EXCLUDED.document;

-- Resolve o nome pelo mesmo cast que a gravação usa, com o search_path da conexão.
-- name: ResolveSearchConfig :one
SELECT sqlc.arg(name)::text::regconfig::oid::bigint AS config_oid;
