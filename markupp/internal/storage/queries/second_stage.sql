-- Leituras e gravações do segundo estágio sobre o grafo do cofre inteiro, no papel
-- markupp_indexer, preso pelo RLS ao cofre reservado.

-- name: DeleteInferredLinks :exec
DELETE FROM note_links WHERE vault_id = sqlc.arg(vault_id) AND kind = 'inferred';

-- name: ListVaultNoteIDs :many
SELECT id FROM notes WHERE vault_id = sqlc.arg(vault_id) ORDER BY id;

-- name: ListVaultEdges :many
SELECT source_id, target_id FROM note_edges WHERE vault_id = sqlc.arg(vault_id) ORDER BY source_id, target_id;

-- name: ListVaultTitleKeys :many
SELECT note_id, title_key FROM note_titles WHERE vault_id = sqlc.arg(vault_id) ORDER BY note_id, title_key;

-- Cada título sem nota citado por uma nota, uma vez por nota, com a primeira grafia.
-- name: ListUnresolvedCitations :many
SELECT DISTINCT ON (l.note_id, l.title_key) l.note_id, l.cited_title, l.title_key
FROM note_links l
WHERE l.vault_id = sqlc.arg(vault_id)
  AND l.kind <> 'inferred'
  AND NOT EXISTS (SELECT 1 FROM note_titles t WHERE t.vault_id = l.vault_id AND t.title_key = l.title_key)
ORDER BY l.note_id, l.title_key, l.cited_title;

-- A ligação inferida guarda o título citado como escrito, a chave do título da nota
-- escolhida e o caminho dela, que a faz ligar só àquela nota. Nota apagada some no JOIN.
-- name: SaveInferredLinks :exec
INSERT INTO note_links (note_id, vault_id, cited_title, title_key, target_path, kind)
SELECT s.id, s.vault_id, d.cited_title, d.title_key, t.path, 'inferred'
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(cited_titles)::text[]) AS cited_title,
             unnest(sqlc.arg(title_keys)::text[]) AS title_key, unnest(sqlc.arg(target_ids)::text[]) AS target_id) d
JOIN notes s ON s.id = d.note_id
JOIN notes t ON t.id = d.target_id;

-- name: DeleteNoteCentrality :exec
DELETE FROM note_centrality WHERE vault_id = sqlc.arg(vault_id);

-- O cofre vem da nota. Nota apagada depois da leitura some no JOIN.
-- name: SaveNoteCentrality :exec
INSERT INTO note_centrality (note_id, vault_id, pagerank, in_degree, out_degree)
SELECT n.id, n.vault_id, d.pagerank, d.in_degree, d.out_degree
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(pageranks)::float8[]) AS pagerank,
             unnest(sqlc.arg(in_degrees)::int[]) AS in_degree, unnest(sqlc.arg(out_degrees)::int[]) AS out_degree) d
JOIN notes n ON n.id = d.note_id;

-- name: ListNoteCommunities :many
SELECT note_id, community_id FROM note_communities WHERE vault_id = sqlc.arg(vault_id) ORDER BY note_id;

-- name: DeleteNoteCommunities :exec
DELETE FROM note_communities WHERE vault_id = sqlc.arg(vault_id);

-- name: NextCommunityIDs :many
SELECT nextval('note_community_id_seq')::bigint AS community_id FROM generate_series(1, sqlc.arg(count)::int);

-- name: SaveNoteCommunities :exec
INSERT INTO note_communities (note_id, vault_id, community_id)
SELECT n.id, n.vault_id, d.community_id
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(community_ids)::bigint[]) AS community_id) d
JOIN notes n ON n.id = d.note_id;
