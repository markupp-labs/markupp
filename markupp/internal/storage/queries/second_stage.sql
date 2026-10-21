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

-- Os títulos que cada nota do cofre cita, fora as ligações inferidas, com a grafia.
-- name: ListVaultCitations :many
SELECT note_id, cited_title, title_key
FROM note_links
WHERE vault_id = sqlc.arg(vault_id) AND kind <> 'inferred'
ORDER BY note_id, title_key, cited_title;

-- Para cada título, as notas do cofre que o têm no corpo (peso D), como a menção não
-- ligada conta as ocorrências da âncora, e se ele é comum, em mais de max(5% das
-- notas, 5).
-- name: CountTitleOccurrences :many
SELECT k.title_key::text AS title_key, (o.occurrences IS NULL)::bool AS common,
       coalesce(o.occurrences, 0)::int AS occurrences
FROM unnest(sqlc.arg(title_keys)::text[]) AS k (title_key)
CROSS JOIN LATERAL (
    SELECT markupp_title_occurrences(sqlc.arg(vault_id), sqlc.arg(search_config)::text::regconfig, k.title_key) AS occurrences
) o;

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

-- A versão do grafo é a revisão da marcação que o segundo estágio leu.
-- name: SaveGraphVersion :exec
INSERT INTO vault_graph_versions (vault_id, version) VALUES (sqlc.arg(vault_id), sqlc.arg(version))
ON CONFLICT (vault_id) DO UPDATE SET version = EXCLUDED.version;
