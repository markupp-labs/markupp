-- +goose Up
-- Cada ligação como item, um por nota a que ela liga, para as ligações de saída, de
-- entrada e do cofre. note_edges passa a levar o título citado como escrito.
CREATE OR REPLACE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate, l.title_key, l.cited_title
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);

-- A ligação resolvida aparece uma vez por nota com o título; a não resolvida, uma vez,
-- sem destino e com o palpite do segundo estágio. A inferida guarda a primeira grafia
-- citada, então o palpite é a inferida da mesma nota com a grafia de uma ligação
-- escrita com a mesma chave. A inferida aparece só como ligação resolvida. A não
-- resolvida vem primeiro para o destino nulo ficar no tipo da coluna.
CREATE VIEW note_link_items WITH (security_invoker = true) AS
SELECT i.source_id, s.path AS source_path, i.vault_id, i.target_id, t.path AS target_path,
       i.cited_title, i.title_key, i.kind, i.predicate, i.inferred_target_id, f.path AS inferred_target_path
FROM (
    SELECT l.note_id AS source_id, l.vault_id, NULL::text AS target_id, l.cited_title, l.title_key, l.kind, l.predicate,
           g.target_id AS inferred_target_id
    FROM note_links l
    LEFT JOIN LATERAL (
        SELECT g.target_id FROM note_edges g
        WHERE g.source_id = l.note_id AND g.kind = 'inferred'
          AND g.cited_title IN (SELECT w.cited_title FROM note_links w
                                WHERE w.note_id = l.note_id AND w.title_key = l.title_key AND w.kind <> 'inferred')
        ORDER BY g.target_id LIMIT 1
    ) g ON true
    WHERE l.kind <> 'inferred'
      AND NOT EXISTS (SELECT 1 FROM note_titles t WHERE t.vault_id = l.vault_id AND t.title_key = l.title_key)
    UNION ALL
    SELECT e.source_id, e.vault_id, e.target_id, e.cited_title, e.title_key, e.kind, e.predicate, NULL
    FROM note_edges e
) i
JOIN notes s ON s.id = i.source_id
LEFT JOIN notes t ON t.id = i.target_id
LEFT JOIN notes f ON f.id = i.inferred_target_id;

GRANT SELECT ON note_link_items TO markupp_app;

-- +goose Down
DROP VIEW IF EXISTS note_link_items;
DROP VIEW note_edges;
CREATE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate, l.title_key
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);
GRANT SELECT ON note_edges TO markupp_app, markupp_indexer;
