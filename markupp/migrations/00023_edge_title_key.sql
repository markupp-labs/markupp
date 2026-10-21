-- +goose Up
-- A menção não ligada ordena pela probabilidade de um título apontar para a nota, que
-- conta as ligações resolvidas com aquele título: a chave citada vai para note_edges.
CREATE OR REPLACE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate, l.title_key
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);

-- Notas ligadas à nota p_note_id em qualquer sentido, que a menção não ligada exclui.
-- Roda com os direitos de quem chama, então o RLS de note_edges vale.
-- +goose StatementBegin
CREATE FUNCTION markupp_linked_notes(p_note_id TEXT) RETURNS SETOF TEXT
LANGUAGE sql STABLE AS $$
    SELECT e.target_id FROM note_edges e WHERE e.source_id = p_note_id
    UNION
    SELECT e.source_id FROM note_edges e WHERE e.target_id = p_note_id
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS markupp_linked_notes(TEXT);
DROP VIEW note_edges;
CREATE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);
GRANT SELECT ON note_edges TO markupp_app;
