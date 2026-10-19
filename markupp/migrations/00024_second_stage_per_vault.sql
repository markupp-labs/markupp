-- +goose Up
-- Segundo estágio do indexador: roda no mesmo Job do cofre, depois do lote, quando as
-- ligações ou os títulos do cofre mudaram. Nesta versão ele recalcula a frequência de
-- documento (df) de cada termo do cofre, que a similaridade léxica usa no idf.

-- Cofres marcados para o segundo estágio. Sem chave estrangeira para vaults: apagar o
-- cofre apaga as notas em cascata, e o trigger de exclusão não pode marcar um cofre que
-- já sumiu. A revisão faz o Job que já leu o cofre deixar marcada a mudança posterior.
CREATE SEQUENCE vault_second_stage_revision_seq;
CREATE TABLE vault_second_stage_queue (
    vault_id TEXT PRIMARY KEY,
    revision BIGINT NOT NULL DEFAULT nextval('vault_second_stage_revision_seq')
);
ALTER SEQUENCE vault_second_stage_revision_seq OWNED BY vault_second_stage_queue.revision;

-- df de cada termo do cofre, no analisador da busca. O tenant vem do cofre, pelas
-- políticas, como em note_titles.
CREATE TABLE vault_term_frequencies (
    vault_id TEXT NOT NULL,
    term     TEXT NOT NULL,
    df       INTEGER NOT NULL,
    PRIMARY KEY (vault_id, term)
);

-- Apagar nota muda títulos e ligações do cofre sem passar pelo indexador. SECURITY
-- DEFINER porque a nota é apagada no papel markupp_app, que não enxerga a marcação.
-- +goose StatementBegin
CREATE FUNCTION markupp_mark_deleted_notes_vaults() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    INSERT INTO vault_second_stage_queue (vault_id)
    SELECT DISTINCT d.vault_id FROM deleted_notes d WHERE EXISTS (SELECT 1 FROM vaults v WHERE v.id = d.vault_id)
    ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');
    RETURN NULL;
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_mark_deleted_notes_vaults() FROM PUBLIC;
CREATE TRIGGER notes_mark_vault_on_delete AFTER DELETE ON notes
    REFERENCING OLD TABLE AS deleted_notes
    FOR EACH STATEMENT EXECUTE FUNCTION markupp_mark_deleted_notes_vaults();

-- O KEDA passa a contar também o cofre só marcado.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION markupp_pending_vaults() RETURNS BIGINT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT count(*) FROM (
        SELECT n.vault_id FROM note_index_queue q JOIN notes n ON n.id = q.note_id
        UNION
        SELECT s.vault_id FROM vault_second_stage_queue s JOIN vaults v ON v.id = s.vault_id
    ) pendentes
$$;
-- +goose StatementEnd

-- Reindexar pode trocar a configuração de texto, e com ela os termos do df: o cofre
-- reenfileirado também fica marcado.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION markupp_requeue_notes(target_vault TEXT) RETURNS BIGINT
LANGUAGE plpgsql VOLATILE SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE
    requeued BIGINT;
BEGIN
    IF target_vault IS NOT NULL AND NOT EXISTS (SELECT 1 FROM vaults WHERE id = target_vault) THEN
        RAISE EXCEPTION 'cofre % inexistente', target_vault USING ERRCODE = 'no_data_found';
    END IF;
    INSERT INTO note_index_queue (note_id)
    SELECT id FROM notes WHERE target_vault IS NULL OR vault_id = target_vault
    ON CONFLICT (note_id) DO UPDATE SET revision = nextval('note_revision_seq');
    GET DIAGNOSTICS requeued = ROW_COUNT;
    INSERT INTO vault_second_stage_queue (vault_id)
    SELECT id FROM vaults WHERE target_vault IS NULL OR id = target_vault
    ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');
    RETURN requeued;
END
$$;
-- +goose StatementEnd

-- O indexador marca, lê e desmarca só o cofre da sessão, que veio do cofre reservado.
GRANT SELECT, INSERT, UPDATE, DELETE ON vault_second_stage_queue TO markupp_indexer;
GRANT USAGE ON SEQUENCE vault_second_stage_revision_seq TO markupp_indexer;
GRANT SELECT, INSERT, DELETE ON vault_term_frequencies TO markupp_indexer;
GRANT SELECT ON vault_term_frequencies TO markupp_app;

ALTER TABLE vault_second_stage_queue ENABLE ROW LEVEL SECURITY;
CREATE POLICY vault_second_stage_queue_indexer ON vault_second_stage_queue TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true));

ALTER TABLE vault_term_frequencies ENABLE ROW LEVEL SECURITY;
CREATE POLICY vault_term_frequencies_indexer ON vault_term_frequencies TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true));
CREATE POLICY vault_term_frequencies_read ON vault_term_frequencies FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

-- Nenhum cofre tem df ainda.
INSERT INTO vault_second_stage_queue (vault_id) SELECT id FROM vaults;

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION markupp_requeue_notes(target_vault TEXT) RETURNS BIGINT
LANGUAGE plpgsql VOLATILE SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE
    requeued BIGINT;
BEGIN
    IF target_vault IS NOT NULL AND NOT EXISTS (SELECT 1 FROM vaults WHERE id = target_vault) THEN
        RAISE EXCEPTION 'cofre % inexistente', target_vault USING ERRCODE = 'no_data_found';
    END IF;
    INSERT INTO note_index_queue (note_id)
    SELECT id FROM notes WHERE target_vault IS NULL OR vault_id = target_vault
    ON CONFLICT (note_id) DO UPDATE SET revision = nextval('note_revision_seq');
    GET DIAGNOSTICS requeued = ROW_COUNT;
    RETURN requeued;
END
$$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION markupp_pending_vaults() RETURNS BIGINT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT count(DISTINCT n.vault_id) FROM note_index_queue q JOIN notes n ON n.id = q.note_id
$$;
-- +goose StatementEnd
DROP TRIGGER IF EXISTS notes_mark_vault_on_delete ON notes;
DROP FUNCTION IF EXISTS markupp_mark_deleted_notes_vaults();
DROP TABLE IF EXISTS vault_term_frequencies;
DROP TABLE IF EXISTS vault_second_stage_queue;
