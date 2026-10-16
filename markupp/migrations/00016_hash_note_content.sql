-- +goose NO TRANSACTION
-- +goose Up
-- Hash SHA-256 do conteúdo de cada nota, para o cliente comparar antes de reenviar e
-- para a fila de indexação comparar sem reler o texto inteiro.
-- Sem transação para o preenchimento confirmar lote a lote. Cada passo pode rodar de
-- novo, então a migração interrompida é retomada pelo próximo upgrade.

-- convert_to é STABLE; a conversão para UTF8 não varia numa base UTF8, então o
-- invólucro pode se declarar imutável.
CREATE OR REPLACE FUNCTION markupp_content_sha256(content TEXT) RETURNS BYTEA
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
RETURN pg_catalog.sha256(pg_catalog.convert_to(content, 'UTF8'));

-- Coluna comum em vez de gerada: coluna gerada STORED reescreve a tabela inteira.
ALTER TABLE notes ADD COLUMN IF NOT EXISTS content_sha256 BYTEA;

-- O trigger calcula o hash a cada escrita do conteúdo ou do próprio hash, então o
-- cliente nunca grava um hash que não seja o do conteúdo.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION markupp_hash_note_content() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    NEW.content_sha256 := markupp_content_sha256(NEW.content);
    RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE OR REPLACE TRIGGER notes_hash_content BEFORE INSERT OR UPDATE OF content, content_sha256 ON notes
    FOR EACH ROW EXECUTE FUNCTION markupp_hash_note_content();

-- Preenche as notas antigas em lotes pela chave, confirmando cada lote para não segurar
-- a trava das linhas até o fim. Atualizar só o hash não dispara o trigger da fila.
-- +goose StatementBegin
DO $$
DECLARE
    ultimo_id TEXT := '';
BEGIN
    LOOP
        WITH lote AS (
            SELECT id FROM notes WHERE id > ultimo_id ORDER BY id LIMIT 1000
        ), preenchidas AS (
            UPDATE notes n SET content_sha256 = markupp_content_sha256(n.content)
            FROM lote WHERE n.id = lote.id AND n.content_sha256 IS NULL
        )
        SELECT max(id) INTO ultimo_id FROM lote;
        EXIT WHEN ultimo_id IS NULL;
        COMMIT;
    END LOOP;
END
$$;
-- +goose StatementEnd

-- A restrição validada sem trava exclusiva deixa o SET NOT NULL dispensar a varredura.
ALTER TABLE notes DROP CONSTRAINT IF EXISTS notes_content_sha256_not_null;
ALTER TABLE notes ADD CONSTRAINT notes_content_sha256_not_null
    CHECK (content_sha256 IS NOT NULL) NOT VALID;
ALTER TABLE notes VALIDATE CONSTRAINT notes_content_sha256_not_null;
ALTER TABLE notes ALTER COLUMN content_sha256 SET NOT NULL;
ALTER TABLE notes DROP CONSTRAINT notes_content_sha256_not_null;

-- O trigger BEFORE já preencheu o hash quando este AFTER compara.
CREATE OR REPLACE TRIGGER notes_enqueue_on_update AFTER UPDATE OF content ON notes
    FOR EACH ROW WHEN (OLD.content_sha256 IS DISTINCT FROM NEW.content_sha256)
    EXECUTE FUNCTION markupp_enqueue_note();

-- +goose Down
CREATE OR REPLACE TRIGGER notes_enqueue_on_update AFTER UPDATE OF content ON notes
    FOR EACH ROW WHEN (OLD.content IS DISTINCT FROM NEW.content)
    EXECUTE FUNCTION markupp_enqueue_note();
DROP TRIGGER IF EXISTS notes_hash_content ON notes;
DROP FUNCTION IF EXISTS markupp_hash_note_content();
ALTER TABLE notes DROP COLUMN IF EXISTS content_sha256;
DROP FUNCTION IF EXISTS markupp_content_sha256(TEXT);
