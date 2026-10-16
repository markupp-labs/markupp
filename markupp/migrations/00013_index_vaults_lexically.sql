-- +goose Up
-- Primeira estrutura do plano de recuperação (ADR-0012, ADR-0017): o índice léxico de
-- cada nota, mantido pelo indexador que o KEDA dispara por cofre (ADR-0032).

-- Fila de pendências: uma linha por nota cujo conteúdo mudou desde a última indexação,
-- com a revisão da escrita. O indexador tira a nota da fila só se a revisão ainda for a
-- que ele leu, então a escrita confirmada durante a indexação continua pendente. Contar
-- e reservar custa o tamanho da fila, não o da base.
CREATE SEQUENCE note_revision_seq;
CREATE TABLE note_index_queue (
    note_id  TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    revision BIGINT NOT NULL DEFAULT nextval('note_revision_seq')
);
ALTER SEQUENCE note_revision_seq OWNED BY note_index_queue.revision;
INSERT INTO note_index_queue (note_id) SELECT id FROM notes;

-- SECURITY DEFINER porque a nota é escrita no papel markupp_app, que não enxerga a fila.
-- +goose StatementBegin
CREATE FUNCTION markupp_enqueue_note() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    INSERT INTO note_index_queue (note_id) VALUES (NEW.id)
    ON CONFLICT (note_id) DO UPDATE SET revision = nextval('note_revision_seq');
    RETURN NULL;
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_enqueue_note() FROM PUBLIC;
CREATE TRIGGER notes_enqueue_on_insert AFTER INSERT ON notes
    FOR EACH ROW EXECUTE FUNCTION markupp_enqueue_note();
-- O índice não guarda caminho nem cofre, então só o conteúdo novo pede reindexação.
CREATE TRIGGER notes_enqueue_on_update AFTER UPDATE OF content ON notes
    FOR EACH ROW WHEN (OLD.content IS DISTINCT FROM NEW.content)
    EXECUTE FUNCTION markupp_enqueue_note();

-- O cofre e o tenant da linha vêm de notes, pela política, e nunca ficam velhos aqui.
CREATE TABLE note_search_index (
    note_id  TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    document TSVECTOR NOT NULL
);
CREATE INDEX note_search_index_document_idx ON note_search_index USING GIN (document);

-- Consulta do scaler do KEDA: um Job por cofre com nota na fila.
-- +goose StatementBegin
CREATE FUNCTION markupp_pending_vaults() RETURNS BIGINT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT count(DISTINCT n.vault_id) FROM note_index_queue q JOIN notes n ON n.id = q.note_id
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_pending_vaults() FROM PUBLIC;

-- Papel do scaler: só conta os cofres pendentes. O deploy dá LOGIN e senha a ele, para o
-- KEDA não guardar a credencial do dono do banco.
-- +goose StatementBegin
DO $$
BEGIN
    CREATE ROLE markupp_scaler NOLOGIN NOSUPERUSER NOBYPASSRLS;
EXCEPTION WHEN duplicate_object OR unique_violation THEN
    NULL;
END
$$;
-- +goose StatementEnd
GRANT USAGE ON SCHEMA public TO markupp_scaler;
GRANT EXECUTE ON FUNCTION markupp_pending_vaults() TO markupp_scaler;

-- Papel do indexador: lê as notas e escreve o índice de um cofre só, o da sessão, e
-- nunca escreve no registro.
-- +goose StatementBegin
DO $$
BEGIN
    CREATE ROLE markupp_indexer NOLOGIN NOSUPERUSER NOBYPASSRLS;
EXCEPTION WHEN duplicate_object OR unique_violation THEN
    NULL;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT (SELECT rolsuper FROM pg_roles WHERE rolname = current_user) THEN
        EXECUTE format('GRANT markupp_indexer TO %I', current_user);
    END IF;
END
$$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO markupp_indexer;
GRANT SELECT ON notes TO markupp_indexer;
GRANT SELECT, DELETE ON note_index_queue TO markupp_indexer;
GRANT SELECT, INSERT, UPDATE ON note_search_index TO markupp_indexer;
-- A política notes_read vale para qualquer papel e chama a função; sem usuário na
-- sessão ela não devolve cofre nenhum.
GRANT EXECUTE ON FUNCTION markupp_member_vaults() TO markupp_indexer;

CREATE POLICY notes_indexer ON notes FOR SELECT TO markupp_indexer
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND vault_id = current_setting('markupp.vault_id', true));

-- Fila e índice herdam o recorte de notes: a subconsulta passa pela política acima.
ALTER TABLE note_index_queue ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_index_queue_indexer ON note_index_queue TO markupp_indexer
    USING (EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));

ALTER TABLE note_search_index ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_search_index_indexer ON note_search_index TO markupp_indexer
    USING (EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));

-- +goose Down
DROP POLICY IF EXISTS notes_indexer ON notes;
REVOKE EXECUTE ON FUNCTION markupp_member_vaults() FROM markupp_indexer;
REVOKE ALL ON notes FROM markupp_indexer;
REVOKE USAGE ON SCHEMA public FROM markupp_indexer, markupp_scaler;
DROP FUNCTION IF EXISTS markupp_pending_vaults();
DROP TABLE IF EXISTS note_search_index;
DROP TRIGGER IF EXISTS notes_enqueue_on_update ON notes;
DROP TRIGGER IF EXISTS notes_enqueue_on_insert ON notes;
DROP FUNCTION IF EXISTS markupp_enqueue_note();
DROP TABLE IF EXISTS note_index_queue;
