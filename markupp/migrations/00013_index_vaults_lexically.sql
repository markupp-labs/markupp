-- +goose Up
-- Primeira estrutura do plano de recuperação (ADR-0012, ADR-0017): o índice léxico de
-- cada nota, mantido pelo indexador que o KEDA dispara por cofre (ADR-0032).

-- Toda escrita de nota ganha uma revisão nova. A nota está pendente enquanto o índice
-- não guardar a revisão atual dela, comparada nota a nota: um cursor único por cofre
-- perderia a escrita que pega revisão antes e confirma depois de outra.
CREATE SEQUENCE note_revision_seq;
ALTER TABLE notes ADD COLUMN revision BIGINT NOT NULL DEFAULT nextval('note_revision_seq');
ALTER SEQUENCE note_revision_seq OWNED BY notes.revision;
GRANT USAGE ON SEQUENCE note_revision_seq TO markupp_app;

-- +goose StatementBegin
CREATE FUNCTION markupp_bump_note_revision() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.revision := nextval('note_revision_seq');
    RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER notes_bump_revision BEFORE UPDATE ON notes
    FOR EACH ROW EXECUTE FUNCTION markupp_bump_note_revision();

CREATE TABLE note_search_index (
    note_id   TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE,
    tenant_id TEXT NOT NULL,
    vault_id  TEXT NOT NULL,
    revision  BIGINT NOT NULL,
    document  TSVECTOR NOT NULL
);
CREATE INDEX note_search_index_document_idx ON note_search_index USING GIN (document);
CREATE INDEX note_search_index_vault_idx ON note_search_index (vault_id);

-- Uma linha por cofre, só para o Job reservar o cofre com FOR UPDATE SKIP LOCKED sem
-- travar a linha do cofre no registro.
CREATE TABLE vault_index_reservations (
    vault_id TEXT PRIMARY KEY REFERENCES vaults (id) ON DELETE CASCADE ON UPDATE CASCADE
);
INSERT INTO vault_index_reservations (vault_id) SELECT id FROM vaults;

-- SECURITY DEFINER porque o cofre nasce no papel da aplicação, que não enxerga a tabela.
-- +goose StatementBegin
CREATE FUNCTION markupp_add_vault_reservation() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    INSERT INTO vault_index_reservations (vault_id) VALUES (NEW.id);
    RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER vaults_add_index_reservation AFTER INSERT ON vaults
    FOR EACH ROW EXECUTE FUNCTION markupp_add_vault_reservation();

-- Consulta do scaler do KEDA: um Job por cofre com nota pendente.
-- +goose StatementBegin
CREATE FUNCTION markupp_pending_vaults() RETURNS BIGINT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT count(DISTINCT n.vault_id) FROM notes n
    LEFT JOIN note_search_index i ON i.note_id = n.id AND i.revision = n.revision
    WHERE i.note_id IS NULL
$$;
-- +goose StatementEnd

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
GRANT SELECT, INSERT, UPDATE ON note_search_index TO markupp_indexer;
-- A política notes_isolation chama a função para qualquer papel; sem usuário na sessão
-- ela não devolve cofre nenhum.
GRANT EXECUTE ON FUNCTION markupp_member_vaults() TO markupp_indexer;

CREATE POLICY notes_indexer ON notes FOR SELECT TO markupp_indexer
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND vault_id = current_setting('markupp.vault_id', true));

ALTER TABLE note_search_index ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_search_index_indexer ON note_search_index TO markupp_indexer
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND vault_id = current_setting('markupp.vault_id', true));

-- +goose Down
DROP POLICY IF EXISTS notes_indexer ON notes;
REVOKE EXECUTE ON FUNCTION markupp_member_vaults() FROM markupp_indexer;
REVOKE ALL ON notes FROM markupp_indexer;
REVOKE USAGE ON SCHEMA public FROM markupp_indexer;
DROP FUNCTION IF EXISTS markupp_pending_vaults();
DROP TRIGGER IF EXISTS vaults_add_index_reservation ON vaults;
DROP FUNCTION IF EXISTS markupp_add_vault_reservation();
DROP TABLE IF EXISTS vault_index_reservations;
DROP TABLE IF EXISTS note_search_index;
DROP TRIGGER IF EXISTS notes_bump_revision ON notes;
DROP FUNCTION IF EXISTS markupp_bump_note_revision();
ALTER TABLE notes DROP COLUMN IF EXISTS revision;
