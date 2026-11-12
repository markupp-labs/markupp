-- +goose Up
-- Ligação automática (ADR-0053): o indexador grava cada candidata por tipo, com o
-- título citado na menção e a pontuação, e note_edges escolhe a que vale. O tenant
-- vem do cofre, pelas políticas, como em note_titles.
CREATE TABLE note_automatic_links (
    source_id   TEXT NOT NULL REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    target_id   TEXT NOT NULL REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    vault_id    TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('mentioned', 'related', 'semantic')),
    cited_title TEXT,
    score       DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (source_id, target_id, kind),
    CHECK (source_id <> target_id),
    CHECK ((kind = 'mentioned') = (cited_title IS NOT NULL))
);
CREATE INDEX note_automatic_links_target_idx ON note_automatic_links (target_id);
CREATE INDEX note_automatic_links_vault_idx ON note_automatic_links (vault_id, kind);

-- Recusa de ligação: o par, sem sentido, em que o servidor não cria ligação automática.
-- É decisão do usuário, então fica no plano de registro; a nota apagada leva a recusa.
CREATE TABLE link_refusals (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    vault_id   TEXT NOT NULL REFERENCES vaults (id) ON DELETE CASCADE,
    low_id     TEXT NOT NULL REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    high_id    TEXT NOT NULL REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (low_id, high_id),
    CHECK (low_id < high_id)
);
CREATE INDEX link_refusals_vault_idx ON link_refusals (vault_id, created_at DESC);

-- As ligações escritas e inferidas, o note_edges de antes, que a automática não repete.
CREATE VIEW note_written_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate, l.title_key, l.cited_title
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);

-- A automática vale quando o par não tem ligação escrita nem inferida, em nenhum sentido,
-- não foi recusado e não tem automática de tipo anterior: menção, texto parecido, sentido.
CREATE VIEW note_automatic_edges WITH (security_invoker = true) AS
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

CREATE OR REPLACE VIEW note_edges WITH (security_invoker = true) AS
SELECT source_id, target_id, vault_id, kind, predicate, title_key, cited_title FROM note_written_edges
UNION ALL
SELECT source_id, target_id, vault_id, kind, predicate, title_key, cited_title FROM note_automatic_edges;

-- Criar ou desfazer recusa muda o grafo do cofre sem passar pelo indexador. SECURITY
-- DEFINER porque a recusa é gravada no papel markupp_app, que não enxerga a marcação.
-- +goose StatementBegin
CREATE FUNCTION markupp_mark_refused_vault() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    INSERT INTO vault_second_stage_queue (vault_id)
    SELECT coalesce(NEW.vault_id, OLD.vault_id) WHERE EXISTS (SELECT 1 FROM vaults v WHERE v.id = coalesce(NEW.vault_id, OLD.vault_id))
    ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');
    RETURN NULL;
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_mark_refused_vault() FROM PUBLIC;
CREATE TRIGGER link_refusals_mark_vault AFTER INSERT OR DELETE ON link_refusals
    FOR EACH ROW EXECUTE FUNCTION markupp_mark_refused_vault();

GRANT SELECT, INSERT, DELETE ON note_automatic_links TO markupp_indexer;
GRANT SELECT ON note_automatic_links TO markupp_app;
GRANT SELECT ON link_refusals TO markupp_indexer;
GRANT SELECT, INSERT, DELETE ON link_refusals TO markupp_app;
GRANT SELECT ON note_written_edges, note_automatic_edges TO markupp_app, markupp_indexer;

ALTER TABLE note_automatic_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_automatic_links_indexer ON note_automatic_links TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true)
           AND EXISTS (SELECT 1 FROM notes n WHERE n.id = source_id));
CREATE POLICY note_automatic_links_read ON note_automatic_links FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

-- Quem não é viewer recusa e desfaz; as duas notas precisam ser do cofre da recusa.
ALTER TABLE link_refusals ENABLE ROW LEVEL SECURITY;
CREATE POLICY link_refusals_indexer ON link_refusals FOR SELECT TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true));
CREATE POLICY link_refusals_read ON link_refusals FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));
CREATE POLICY link_refusals_insert ON link_refusals FOR INSERT TO markupp_app
    WITH CHECK (vault_id IN (SELECT markupp_member_vaults())
                AND current_setting('markupp.role', true) <> 'viewer'
                AND created_by = current_setting('markupp.user_id', true)
                AND (SELECT count(*) FROM notes n WHERE n.id IN (low_id, high_id) AND n.vault_id = link_refusals.vault_id) = 2);
CREATE POLICY link_refusals_delete ON link_refusals FOR DELETE TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults())
           AND current_setting('markupp.role', true) <> 'viewer');

-- Nenhum cofre tem ligação por menção ainda.
INSERT INTO vault_second_stage_queue (vault_id) SELECT id FROM vaults
ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');

-- +goose Down
CREATE OR REPLACE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind, l.predicate, l.title_key, l.cited_title
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);
DROP VIEW IF EXISTS note_automatic_edges;
DROP VIEW IF EXISTS note_written_edges;
DROP TRIGGER IF EXISTS link_refusals_mark_vault ON link_refusals;
DROP FUNCTION IF EXISTS markupp_mark_refused_vault();
DROP TABLE IF EXISTS link_refusals;
DROP TABLE IF EXISTS note_automatic_links;
