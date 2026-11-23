-- Recusas de ligação, no papel markupp_app, sob o RLS dos cofres do ator.

-- name: CountVaultNotes :one
SELECT count(*)::int FROM notes WHERE vault_id = sqlc.arg(vault_id) AND id = ANY (sqlc.arg(note_ids)::text[]);

-- O par não tem sentido: a nota de menor id vai em low_id. A recusa repetida fica a que existe.
-- name: CreateLinkRefusal :exec
INSERT INTO link_refusals (vault_id, low_id, high_id, created_by)
VALUES (sqlc.arg(vault_id), least(sqlc.arg(note_a)::text, sqlc.arg(note_b)::text),
        greatest(sqlc.arg(note_a)::text, sqlc.arg(note_b)::text), current_setting('markupp.user_id', true))
ON CONFLICT (low_id, high_id) DO NOTHING;

-- name: GetLinkRefusalByPair :one
SELECT r.id, r.created_at, l.id AS low_id, l.path AS low_path, h.id AS high_id, h.path AS high_path
FROM link_refusals r
JOIN notes l ON l.id = r.low_id
JOIN notes h ON h.id = r.high_id
WHERE r.low_id = least(sqlc.arg(note_a)::text, sqlc.arg(note_b)::text)
  AND r.high_id = greatest(sqlc.arg(note_a)::text, sqlc.arg(note_b)::text);

-- name: ListLinkRefusals :many
SELECT r.id, r.created_at, l.id AS low_id, l.path AS low_path, h.id AS high_id, h.path AS high_path
FROM link_refusals r
JOIN notes l ON l.id = r.low_id
JOIN notes h ON h.id = r.high_id
WHERE r.vault_id = sqlc.arg(vault_id)
ORDER BY r.created_at DESC, r.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: DeleteLinkRefusal :execrows
DELETE FROM link_refusals WHERE vault_id = sqlc.arg(vault_id) AND id = sqlc.arg(id);
