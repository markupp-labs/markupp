-- +goose Up
-- Ligação tipada: a chave do frontmatter cujo valor é wikilink vira o predicado da
-- ligação, na mesma tabela das outras. Só a ligação tipada tem predicado, e ele vai
-- para note_edges para backlinks e vizinhança dizerem por que chave a nota cita.
ALTER TABLE note_links ADD COLUMN predicate TEXT;
ALTER TABLE note_links ADD CONSTRAINT note_links_predicate_typed
    CHECK ((kind = 'typed') = (predicate IS NOT NULL));

CREATE OR REPLACE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);

-- As notas já indexadas não têm ligações tipadas: todas voltam para a fila.
SELECT markupp_requeue_notes(NULL);

-- +goose Down
DROP VIEW note_edges;
CREATE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);
GRANT SELECT ON note_edges TO markupp_app;
DELETE FROM note_links WHERE kind = 'typed';
ALTER TABLE note_links DROP COLUMN predicate;
