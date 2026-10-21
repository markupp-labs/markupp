-- Leituras do grafo para desenhar, no papel da aplicação, sob RLS. note_ids nulo lê o
-- cofre inteiro; senão só as notas dadas (ou as ligações que tocam nelas).

-- Notas com a organização derivada. A nota sem centralidade ainda não passou pelo
-- segundo estágio e não conta como órfã.
-- name: ListGraphNotes :many
SELECT n.id, n.path, c.pagerank, c.in_degree, c.out_degree, m.community_id
FROM notes n
LEFT JOIN note_centrality c ON c.note_id = n.id
LEFT JOIN note_communities m ON m.note_id = n.id
WHERE n.vault_id = sqlc.arg(vault_id)
  AND (sqlc.narg(note_ids)::text[] IS NULL OR n.id = ANY (sqlc.narg(note_ids)::text[]))
  AND (sqlc.narg(community_id)::bigint IS NULL OR m.community_id = sqlc.narg(community_id)::bigint)
  AND NOT (sqlc.arg(hide_orphans)::bool AND COALESCE(c.in_degree + c.out_degree = 0, false))
ORDER BY n.path, n.id;

-- Ligações resolvidas, uma por par de notas, tipo e predicado.
-- name: ListGraphEdges :many
SELECT DISTINCT source_id, target_id, kind, predicate
FROM note_edges
WHERE vault_id = sqlc.arg(vault_id)
  AND (sqlc.narg(note_ids)::text[] IS NULL OR source_id = ANY (sqlc.narg(note_ids)::text[]) OR target_id = ANY (sqlc.narg(note_ids)::text[]))
  AND NOT (sqlc.arg(hide_inferred)::bool AND kind = 'inferred')
ORDER BY source_id, target_id, kind, predicate NULLS FIRST;

-- Ligações não resolvidas, uma por nota, chave de título, tipo e predicado, com a
-- primeira grafia citada; em cada chave, a menor grafia vem primeiro.
-- name: ListGraphCitations :many
SELECT l.note_id, l.title_key, min(l.cited_title)::text AS cited_title, l.kind, l.predicate
FROM note_links l
WHERE l.vault_id = sqlc.arg(vault_id)
  AND l.kind <> 'inferred'
  AND (sqlc.narg(note_ids)::text[] IS NULL OR l.note_id = ANY (sqlc.narg(note_ids)::text[]))
  AND NOT EXISTS (SELECT 1 FROM note_titles t WHERE t.vault_id = l.vault_id AND t.title_key = l.title_key)
GROUP BY l.note_id, l.title_key, l.kind, l.predicate
ORDER BY l.title_key, cited_title, l.note_id, l.kind, l.predicate NULLS FIRST;

-- Versão do grafo do cofre, zero antes do primeiro segundo estágio.
-- name: GetGraphVersion :one
SELECT COALESCE((SELECT version FROM vault_graph_versions WHERE vault_id = sqlc.arg(vault_id)), 0)::bigint AS version;
