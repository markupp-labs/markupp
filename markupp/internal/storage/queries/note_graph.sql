-- Leituras do grafo de ligações, no papel da aplicação: o RLS recorta o tenant e os
-- cofres do ator, e note_edges resolve as ligações pelos títulos naquele momento.

-- name: VaultVisible :one
SELECT EXISTS (SELECT 1 FROM vaults WHERE id = sqlc.arg(vault_id)) AS visible;

-- Uma linha por nota que liga, com as chaves das ligações tipadas dela, sem repetir,
-- e se ela liga só por ligação inferida.
-- name: ListBacklinks :many
SELECT n.id, n.vault_id, n.path, n.updated_at,
       coalesce(array_agg(DISTINCT e.predicate) FILTER (WHERE e.predicate IS NOT NULL), '{}')::text[] AS predicates,
       bool_and(e.kind = 'inferred')::bool AS inferred
FROM note_edges e JOIN notes n ON n.id = e.source_id
WHERE e.target_id = sqlc.arg(note_id)
GROUP BY n.id
ORDER BY n.path, n.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- inferred diz se as duas notas se ligam só por ligação inferida, em qualquer sentido.
-- name: ListNeighbors :many
WITH v AS (
    SELECT e.target_id AS id, false AS incoming, true AS outgoing, e.predicate, e.kind FROM note_edges e WHERE e.source_id = sqlc.arg(note_id)
    UNION ALL
    SELECT e.source_id, true, false, e.predicate, e.kind FROM note_edges e WHERE e.target_id = sqlc.arg(note_id)
)
SELECT n.id, n.vault_id, n.path, n.updated_at, bool_or(v.incoming)::bool AS incoming, bool_or(v.outgoing)::bool AS outgoing,
       coalesce(array_agg(DISTINCT v.predicate) FILTER (WHERE v.incoming AND v.predicate IS NOT NULL), '{}')::text[] AS incoming_predicates,
       coalesce(array_agg(DISTINCT v.predicate) FILTER (WHERE v.outgoing AND v.predicate IS NOT NULL), '{}')::text[] AS outgoing_predicates,
       bool_and(v.kind = 'inferred')::bool AS inferred
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

-- A ligação inferida não entra: ela é o palpite para a não resolvida, que continua na
-- lista como foi escrita.
-- name: ListUnresolvedLinks :many
SELECT l.note_id, n.path, l.cited_title, l.kind
FROM note_links l
JOIN notes n ON n.id = l.note_id
WHERE l.vault_id = sqlc.arg(vault_id)
  AND l.kind <> 'inferred'
  AND NOT EXISTS (SELECT 1 FROM note_titles t WHERE t.vault_id = l.vault_id AND t.title_key = l.title_key)
ORDER BY n.path, l.title_key, l.kind
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
