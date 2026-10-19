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

-- A ligação não resolvida aparece uma vez, sem destino e com o palpite do segundo
-- estágio. A inferida guarda a primeira grafia citada, então o palpite é a inferida da
-- mesma nota com a grafia de uma ligação escrita com a mesma chave. A view própria
-- deixa a lista das não resolvidas do cofre sem calcular as resolvidas.
CREATE VIEW note_unresolved_link_items WITH (security_invoker = true) AS
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
  AND NOT EXISTS (SELECT 1 FROM note_titles t WHERE t.vault_id = l.vault_id AND t.title_key = l.title_key);

-- A ligação resolvida aparece uma vez por nota com o título, a inferida só como
-- resolvida. As não resolvidas vêm primeiro para o destino nulo ficar no tipo da
-- coluna.
CREATE VIEW note_link_items WITH (security_invoker = true) AS
SELECT * FROM note_unresolved_link_items
UNION ALL
SELECT e.source_id, s.path, e.vault_id, e.target_id, t.path, e.cited_title, e.title_key, e.kind, e.predicate, NULL, NULL
FROM note_edges e
JOIN notes s ON s.id = e.source_id
JOIN notes t ON t.id = e.target_id;

GRANT SELECT ON note_link_items, note_unresolved_link_items TO markupp_app;

-- +goose Down
DROP VIEW IF EXISTS note_link_items;
DROP VIEW IF EXISTS note_unresolved_link_items;
DROP VIEW note_edges;
CREATE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate, l.title_key
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);
GRANT SELECT ON note_edges TO markupp_app, markupp_indexer;
