-- Leituras e gravações do segundo estágio sobre o grafo do cofre inteiro, no papel
-- markupp_indexer, preso pelo RLS ao cofre reservado.

-- name: DeleteInferredLinks :exec
DELETE FROM note_links WHERE vault_id = sqlc.arg(vault_id) AND kind = 'inferred';

-- name: ListVaultNoteIDs :many
SELECT id FROM notes WHERE vault_id = sqlc.arg(vault_id) ORDER BY id;

-- A ligação automática pesa menos no PageRank e nas comunidades (ADR-0053).
-- name: ListVaultEdges :many
SELECT source_id, target_id, (kind IN ('mentioned', 'related', 'semantic'))::bool AS automatic
FROM note_edges WHERE vault_id = sqlc.arg(vault_id) ORDER BY source_id, target_id;

-- name: ListVaultNoteContents :many
SELECT id, content FROM notes WHERE vault_id = sqlc.arg(vault_id) ORDER BY id;

-- name: DeleteVaultMentionLinks :exec
DELETE FROM note_automatic_links WHERE vault_id = sqlc.arg(vault_id) AND kind = 'mentioned';

-- name: DeleteVaultRelatedLinks :exec
DELETE FROM note_automatic_links WHERE vault_id = sqlc.arg(vault_id) AND kind = 'related';

-- name: DeleteVaultSemanticLinks :exec
DELETE FROM note_automatic_links WHERE vault_id = sqlc.arg(vault_id) AND kind = 'semantic';

-- O HNSW filtra o cofre depois de achar os vizinhos; a busca iterativa continua até
-- achar os k do cofre, na ordem exata. Vale até o fim da transação.
-- shortcut: a busca para em hnsw.max_scan_tuples (20 mil); um cofre pequeno numa tabela
-- grande pode ficar com menos de 5 vizinhos, e então vale subir o teto ou filtrar antes.
-- name: AllowFilteredVectorScan :exec
SELECT set_config('hnsw.iterative_scan', 'strict_order', true);

-- Ligação por sentido: o par em que cada nota está entre as 5 de maior cosseno da
-- outra (kNN mútuo), sem mínimo, gravado nos dois sentidos com o cosseno.
-- name: SaveVaultSemanticLinks :exec
WITH knn AS (
    SELECT e.note_id AS source_id, n.note_id AS target_id, n.similarity
    FROM note_embeddings e
    CROSS JOIN LATERAL (
        SELECT o.note_id, 1 - (o.embedding <=> e.embedding) AS similarity
        FROM note_embeddings o
        WHERE o.vault_id = e.vault_id AND o.note_id <> e.note_id
        ORDER BY o.embedding <=> e.embedding
        LIMIT 5
    ) n
    WHERE e.vault_id = sqlc.arg(vault_id)
)
INSERT INTO note_automatic_links (source_id, target_id, vault_id, kind, score)
SELECT a.source_id, a.target_id, sqlc.arg(vault_id), 'semantic', a.similarity
FROM knn a
JOIN knn b ON b.source_id = a.target_id AND b.target_id = a.source_id;

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
