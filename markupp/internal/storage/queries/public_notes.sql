-- Leitura sem conta (ADR-0048), no papel markupp_reader: o RLS só deixa ver as notas
-- públicas, e cada consulta recorta pela publicação da nota lida.

-- name: GetPublicScope :one
SELECT note_id::text AS note_id, vault_id::text AS vault_id, publication_id::text AS publication_id,
       kind::text AS kind, COALESCE(folder_path, '')::text AS folder_path, indexable::bool AS indexable,
       COALESCE(vault_name, '')::text AS vault_name
FROM markupp_public_scope(sqlc.arg(public_id)::text);

-- As ligações escritas da nota. O alvo privado não existe para o leitor: a ligação fica
-- não resolvida e sem id público.
-- name: ListPublicNoteLinks :many
SELECT DISTINCT i.cited_title, i.kind, t.public_id, i.target_path
FROM note_link_items i
LEFT JOIN public_notes t ON t.note_id = i.target_id
WHERE i.source_id = sqlc.arg(note_id) AND i.kind <> 'inferred'
ORDER BY i.cited_title, i.kind, t.public_id NULLS FIRST, i.target_path;

-- name: ListPublicTree :many
SELECT pn.public_id, n.path
FROM public_notes pn
JOIN notes n ON n.id = pn.note_id
WHERE pn.publication_id = sqlc.arg(publication_id)
ORDER BY n.path, pn.public_id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountPublicTree :one
SELECT count(*) FROM public_notes WHERE publication_id = sqlc.arg(publication_id);

-- Como SearchNotes, recortada à publicação.
-- name: SearchPublicNotes :many
SELECT pn.public_id, n.path,
       ts_headline(sqlc.arg(search_config)::text::regconfig, n.content, q.query,
                   'StartSel=' || chr(2) || ', StopSel=' || chr(3))::text AS snippet,
       ts_rank(s.document, q.query, 1)::float8 AS rank
FROM websearch_to_tsquery(sqlc.arg(search_config)::text::regconfig, sqlc.arg(query)::text) AS q (query)
JOIN note_search_index s ON s.document @@ q.query
JOIN public_notes pn ON pn.note_id = s.note_id
JOIN notes n ON n.id = s.note_id
WHERE pn.publication_id = sqlc.arg(publication_id)
ORDER BY rank DESC, pn.public_id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountPublicSearch :one
SELECT count(*)
FROM websearch_to_tsquery(sqlc.arg(search_config)::text::regconfig, sqlc.arg(query)::text) AS q (query)
JOIN note_search_index s ON s.document @@ q.query
JOIN public_notes pn ON pn.note_id = s.note_id
WHERE pn.publication_id = sqlc.arg(publication_id);

-- name: ListPublicGraphNodes :many
SELECT pn.public_id, n.path
FROM public_notes pn
JOIN notes n ON n.id = pn.note_id
WHERE pn.publication_id = sqlc.arg(publication_id)
ORDER BY n.path, pn.public_id;

-- name: ListPublicGraphEdges :many
SELECT DISTINCT s.public_id AS source_public_id, t.public_id AS target_public_id, e.kind
FROM note_edges e
JOIN public_notes s ON s.note_id = e.source_id
JOIN public_notes t ON t.note_id = e.target_id
WHERE s.publication_id = sqlc.arg(publication_id) AND t.publication_id = sqlc.arg(publication_id)
ORDER BY 1, 2, 3;
