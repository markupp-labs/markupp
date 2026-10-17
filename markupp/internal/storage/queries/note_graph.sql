-- Leituras do grafo de ligações, no papel da aplicação: o RLS recorta o tenant e os
-- cofres do ator, e note_edges resolve as ligações pelos títulos naquele momento.

-- name: VaultVisible :one
SELECT EXISTS (SELECT 1 FROM vaults WHERE id = sqlc.arg(vault_id)) AS visible;

-- name: ListBacklinks :many
SELECT n.id, n.vault_id, n.path, n.updated_at
FROM notes n
WHERE n.id IN (SELECT e.source_id FROM note_edges e WHERE e.target_id = sqlc.arg(note_id))
ORDER BY n.path, n.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListNeighbors :many
WITH v AS (
    SELECT e.target_id AS id, false AS incoming, true AS outgoing FROM note_edges e WHERE e.source_id = sqlc.arg(note_id)
    UNION ALL
    SELECT e.source_id, true, false FROM note_edges e WHERE e.target_id = sqlc.arg(note_id)
)
SELECT n.id, n.vault_id, n.path, n.updated_at, bool_or(v.incoming)::bool AS incoming, bool_or(v.outgoing)::bool AS outgoing
FROM v JOIN notes n ON n.id = v.id
GROUP BY n.id
ORDER BY n.path, n.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- Só a nota já indexada entra: a que ainda está só no registro não tem ligações
-- derivadas e pareceria órfã.
-- name: ListOrphans :many
SELECT n.id, n.vault_id, n.path, n.updated_at
FROM notes n
WHERE n.vault_id = sqlc.arg(vault_id)
  AND EXISTS (SELECT 1 FROM note_search_index s WHERE s.note_id = n.id)
  AND NOT EXISTS (SELECT 1 FROM note_edges e WHERE e.source_id = n.id)
  AND NOT EXISTS (SELECT 1 FROM note_edges e WHERE e.target_id = n.id)
ORDER BY n.path, n.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListUnresolvedLinks :many
SELECT l.note_id, n.path, l.cited_title, l.kind
FROM note_links l
JOIN notes n ON n.id = l.note_id
WHERE l.vault_id = sqlc.arg(vault_id)
  AND NOT EXISTS (SELECT 1 FROM note_titles t WHERE t.vault_id = l.vault_id AND t.title_key = l.title_key)
ORDER BY n.path, l.title_key, l.kind
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
