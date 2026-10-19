-- Leituras do grafo de ligações, no papel da aplicação: o RLS recorta o tenant e os
-- cofres do ator, e note_edges e note_link_items resolvem as ligações pelos títulos
-- naquele momento.

-- name: VaultVisible :one
SELECT EXISTS (SELECT 1 FROM vaults WHERE id = sqlc.arg(vault_id)) AS visible;

-- As listas de ligações saem na mesma ordem: nota que liga, título citado, tipo,
-- predicado e nota a que liga.

-- name: ListOutgoingLinks :many
SELECT * FROM note_link_items
WHERE source_id = sqlc.arg(note_id)
  AND (sqlc.arg(kind)::text = '' OR kind = sqlc.arg(kind)::text)
ORDER BY source_path, source_id, title_key, kind, predicate NULLS FIRST, target_path NULLS FIRST, target_id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListIncomingLinks :many
SELECT * FROM note_link_items
WHERE target_id = sqlc.arg(note_id)::text
  AND (sqlc.arg(kind)::text = '' OR kind = sqlc.arg(kind)::text)
ORDER BY source_path, source_id, title_key, kind, predicate NULLS FIRST, target_path NULLS FIRST, target_id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListVaultLinks :many
SELECT * FROM note_link_items
WHERE vault_id = sqlc.arg(vault_id)
  AND (sqlc.arg(kind)::text = '' OR kind = sqlc.arg(kind)::text)
ORDER BY source_path, source_id, title_key, kind, predicate NULLS FIRST, target_path NULLS FIRST, target_id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- O destino nulo das não resolvidas é constante, então o filtro descarta a parte
-- delas já no plano.
-- name: ListVaultResolvedLinks :many
SELECT * FROM note_link_items
WHERE vault_id = sqlc.arg(vault_id)
  AND target_id IS NOT NULL
  AND (sqlc.arg(kind)::text = '' OR kind = sqlc.arg(kind)::text)
ORDER BY source_path, source_id, title_key, kind, predicate NULLS FIRST, target_path NULLS FIRST, target_id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListVaultUnresolvedLinks :many
SELECT * FROM note_unresolved_link_items
WHERE vault_id = sqlc.arg(vault_id)
  AND (sqlc.arg(kind)::text = '' OR kind = sqlc.arg(kind)::text)
ORDER BY source_path, source_id, title_key, kind, predicate NULLS FIRST
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- Chaves de título da nota, para achar quem a menciona.
-- name: ListNoteTitleKeys :many
SELECT title_key FROM note_titles WHERE note_id = sqlc.arg(note_id) ORDER BY title_key;

-- Notas do cofre, fora a própria e as ligadas a ela, cujo corpo (peso D) tem alguma
-- das frases. O GIN acha as candidatas; a aplicação confere título inteiro, fronteira
-- de palavra e texto fora de ligação. Frase só de stopwords não entra na consulta.
-- name: ListMentioningNotes :many
WITH q AS (
    SELECT string_agg('(' || phraseto_tsquery(sqlc.arg(search_config)::text::regconfig, p)::text || ')', ' | ')::tsquery AS query
    FROM unnest(sqlc.arg(phrases)::text[]) AS p
    WHERE numnode(phraseto_tsquery(sqlc.arg(search_config)::text::regconfig, p)) > 0
)
SELECT n.id, n.vault_id, n.path, n.updated_at, n.content
FROM q
JOIN note_search_index s ON s.document @@ q.query
JOIN notes n ON n.id = s.note_id
WHERE n.vault_id = sqlc.arg(vault_id)
  AND n.id <> sqlc.arg(note_id)
  AND ts_filter(s.document, '{d}') @@ q.query
  AND n.id NOT IN (SELECT markupp_linked_notes(sqlc.arg(note_id)));

-- Títulos das outras notas do cofre que não se ligam à nota: o dicionário do que ela
-- pode mencionar.
-- name: ListMentionableTitles :many
SELECT n.id, n.vault_id, n.path, n.updated_at, t.title_key
FROM note_titles t
JOIN notes n ON n.id = t.note_id
WHERE t.vault_id = sqlc.arg(vault_id)
  AND t.note_id <> sqlc.arg(note_id)
  AND t.note_id NOT IN (SELECT markupp_linked_notes(sqlc.arg(note_id)));

-- Para cada par (título, nota com o título): se o título é stopword em português ou
-- inglês, as notas do cofre com o título no corpo (ocorrências), as notas que ligam à
-- nota por esse título e as notas do cofre com o título (candidatas).
-- name: MentionAnchorStats :many
SELECT a.title_key::text AS title_key, a.target_id::text AS target_id,
       (to_tsvector('pg_catalog.portuguese', a.title_key) = ''::tsvector
        OR to_tsvector('pg_catalog.english', a.title_key) = ''::tsvector)::bool AS stopword,
       (SELECT count(*) FROM note_search_index s
        JOIN notes n ON n.id = s.note_id
        CROSS JOIN phraseto_tsquery(sqlc.arg(search_config)::text::regconfig, a.title_key) AS q (query)
        WHERE n.vault_id = sqlc.arg(vault_id) AND s.document @@ q.query AND ts_filter(s.document, '{d}') @@ q.query)::int AS occurrences,
       (SELECT count(DISTINCT e.source_id) FROM note_edges e
        WHERE e.target_id = a.target_id AND e.title_key = a.title_key)::int AS anchor_links,
       (SELECT count(*) FROM note_titles t
        WHERE t.vault_id = sqlc.arg(vault_id) AND t.title_key = a.title_key)::int AS candidates
FROM (SELECT unnest(sqlc.arg(title_keys)::text[]) AS title_key, unnest(sqlc.arg(target_ids)::text[]) AS target_id) a;

-- name: ListStructuralSuggestions :many
SELECT n.id, n.vault_id, n.path, n.updated_at, r.score::float8 AS score
FROM markupp_resource_allocation(sqlc.arg(note_id)) AS r (note_id, score)
JOIN notes n ON n.id = r.note_id
ORDER BY r.score DESC, n.path, n.id;

-- name: ListLexicalSuggestions :many
SELECT n.id, n.vault_id, n.path, n.updated_at, c.score::float8 AS score
FROM markupp_lexical_similarity(sqlc.arg(vault_id), sqlc.arg(note_id)) AS c (note_id, score)
JOIN notes n ON n.id = c.note_id
ORDER BY c.score DESC, n.path, n.id;

-- Uma linha por comunidade, das maiores para as menores, com a nota de maior PageRank.
-- name: ListCommunities :many
WITH ranked AS (
    SELECT m.community_id, n.id, n.path,
           count(*) OVER (PARTITION BY m.community_id) AS size,
           row_number() OVER (PARTITION BY m.community_id ORDER BY c.pagerank DESC NULLS LAST, n.path, n.id) AS position
    FROM note_communities m
    JOIN notes n ON n.id = m.note_id
    LEFT JOIN note_centrality c ON c.note_id = m.note_id
    WHERE m.vault_id = sqlc.arg(vault_id)
      AND (sqlc.narg(community_id)::bigint IS NULL OR m.community_id = sqlc.narg(community_id)::bigint)
)
SELECT community_id, size::int AS size, id AS hub_id, path AS hub_path
FROM ranked
WHERE position = 1
ORDER BY size DESC, community_id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
