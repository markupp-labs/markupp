-- +goose Up
-- A ligação por texto parecido (ADR-0053) não cita título: a lista de ligações a lê
-- com título vazio, como lê o texto das outras.
CREATE OR REPLACE VIEW note_automatic_edges WITH (security_invoker = true) AS
SELECT a.source_id, a.target_id, a.vault_id, a.kind, NULL::text AS predicate,
       coalesce(a.cited_title, '') AS title_key, coalesce(a.cited_title, '') AS cited_title, a.score
FROM note_automatic_links a
WHERE NOT EXISTS (SELECT 1 FROM note_written_edges w
                  WHERE (w.source_id, w.target_id) IN ((a.source_id, a.target_id), (a.target_id, a.source_id)))
  AND NOT EXISTS (SELECT 1 FROM link_refusals r
                  WHERE r.low_id = least(a.source_id, a.target_id) AND r.high_id = greatest(a.source_id, a.target_id))
  AND NOT EXISTS (SELECT 1 FROM note_automatic_links b
                  WHERE (b.source_id, b.target_id) IN ((a.source_id, a.target_id), (a.target_id, a.source_id))
                    AND array_position(ARRAY['mentioned', 'related', 'semantic'], b.kind)
                        < array_position(ARRAY['mentioned', 'related', 'semantic'], a.kind));

-- Nenhum cofre tem ligação por texto parecido ainda.
INSERT INTO vault_second_stage_queue (vault_id) SELECT id FROM vaults
ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');

-- +goose Down
DELETE FROM note_automatic_links WHERE kind = 'related';
CREATE OR REPLACE VIEW note_automatic_edges WITH (security_invoker = true) AS
SELECT a.source_id, a.target_id, a.vault_id, a.kind, NULL::text AS predicate,
       a.cited_title AS title_key, a.cited_title, a.score
FROM note_automatic_links a
WHERE NOT EXISTS (SELECT 1 FROM note_written_edges w
                  WHERE (w.source_id, w.target_id) IN ((a.source_id, a.target_id), (a.target_id, a.source_id)))
  AND NOT EXISTS (SELECT 1 FROM link_refusals r
                  WHERE r.low_id = least(a.source_id, a.target_id) AND r.high_id = greatest(a.source_id, a.target_id))
  AND NOT EXISTS (SELECT 1 FROM note_automatic_links b
                  WHERE (b.source_id, b.target_id) IN ((a.source_id, a.target_id), (a.target_id, a.source_id))
                    AND array_position(ARRAY['mentioned', 'related', 'semantic'], b.kind)
                        < array_position(ARRAY['mentioned', 'related', 'semantic'], a.kind));
