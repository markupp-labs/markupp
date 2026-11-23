-- +goose Up
-- markupp_note_path_exists respondia para qualquer cofre e caminho, inclusive de outro
-- tenant. A substituta só responde no cofre de uma nota pública de origem: nota de
-- origem que não é pública, ou caminho fora do cofre dela, dá false. Quem enxerga o
-- cofre inteiro (aplicação e indexador) resolve pelo EXISTS em notes; a função cobre o
-- que a RLS da leitura pública esconde. Os três papéis leem as views, e o Postgres
-- exige EXECUTE de quem consulta a view mesmo quando a função não muda o resultado.
-- +goose StatementBegin
CREATE FUNCTION markupp_public_note_path_exists(p_source_note_id TEXT, p_path TEXT) RETURNS BOOLEAN
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT EXISTS (SELECT 1 FROM public_notes pn JOIN notes n ON n.vault_id = pn.vault_id
                   WHERE pn.note_id = p_source_note_id AND n.path = p_path)
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_public_note_path_exists(TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_public_note_path_exists(TEXT, TEXT) TO markupp_app, markupp_indexer, markupp_reader;

CREATE OR REPLACE VIEW note_written_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate, l.title_key, l.cited_title
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR (NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path)
       AND NOT markupp_public_note_path_exists(l.note_id, l.target_path));

-- Sem destino também para quem lê a nota pública: o link para caminho oculto aparece
-- como não resolvido, como o link para título só de nota privada.
CREATE OR REPLACE VIEW note_unresolved_link_items WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, s.path AS source_path, l.vault_id, NULL::text AS target_id, NULL::text AS target_path,
       l.cited_title, l.title_key, l.kind, l.predicate, g.target_id AS inferred_target_id, f.path AS inferred_target_path
FROM note_links l
JOIN notes s ON s.id = l.note_id
LEFT JOIN LATERAL (
    SELECT g.target_id FROM note_edges g
    WHERE g.source_id = l.note_id AND g.kind = 'inferred'
      AND g.cited_title IN (SELECT w.cited_title FROM note_links w
                            WHERE w.note_id = l.note_id AND w.title_key = l.title_key AND w.kind <> 'inferred')
    ORDER BY g.target_id LIMIT 1
) g ON true
LEFT JOIN notes f ON f.id = g.target_id
WHERE l.kind <> 'inferred'
  AND (NOT EXISTS (SELECT 1 FROM note_titles t WHERE t.vault_id = l.vault_id AND t.title_key = l.title_key)
       OR (markupp_public_note_path_exists(l.note_id, l.target_path)
           AND NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path)));

DROP FUNCTION markupp_note_path_exists(TEXT, TEXT);

-- +goose Down
-- +goose StatementBegin
CREATE FUNCTION markupp_note_path_exists(p_vault_id TEXT, p_path TEXT) RETURNS BOOLEAN
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT EXISTS (SELECT 1 FROM notes WHERE vault_id = p_vault_id AND path = p_path)
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_note_path_exists(TEXT, TEXT) FROM PUBLIC;
-- Os papéis que leem as views de ligação, que são security_invoker.
GRANT EXECUTE ON FUNCTION markupp_note_path_exists(TEXT, TEXT) TO markupp_app, markupp_indexer, markupp_reader;

CREATE OR REPLACE VIEW note_written_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate, l.title_key, l.cited_title
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT markupp_note_path_exists(l.vault_id, l.target_path);

-- Sem destino também para quem lê a nota pública: o link para caminho oculto aparece
-- como não resolvido, como o link para título só de nota privada.
CREATE OR REPLACE VIEW note_unresolved_link_items WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, s.path AS source_path, l.vault_id, NULL::text AS target_id, NULL::text AS target_path,
       l.cited_title, l.title_key, l.kind, l.predicate, g.target_id AS inferred_target_id, f.path AS inferred_target_path
FROM note_links l
JOIN notes s ON s.id = l.note_id
LEFT JOIN LATERAL (
    SELECT g.target_id FROM note_edges g
    WHERE g.source_id = l.note_id AND g.kind = 'inferred'
      AND g.cited_title IN (SELECT w.cited_title FROM note_links w
                            WHERE w.note_id = l.note_id AND w.title_key = l.title_key AND w.kind <> 'inferred')
    ORDER BY g.target_id LIMIT 1
) g ON true
LEFT JOIN notes f ON f.id = g.target_id
WHERE l.kind <> 'inferred'
  AND (NOT EXISTS (SELECT 1 FROM note_titles t WHERE t.vault_id = l.vault_id AND t.title_key = l.title_key)
       OR (markupp_note_path_exists(l.vault_id, l.target_path)
           AND NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path)));

DROP FUNCTION IF EXISTS markupp_public_note_path_exists(TEXT, TEXT);
