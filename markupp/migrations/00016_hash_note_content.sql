-- +goose Up
-- Hash SHA-256 do conteúdo de cada nota, para o cliente comparar antes de reenviar e
-- para a fila de indexação comparar sem reler o texto inteiro.

-- convert_to é STABLE e coluna gerada exige expressão IMMUTABLE. A conversão para
-- UTF8 não varia numa base UTF8, então o invólucro pode se declarar imutável.
CREATE FUNCTION markupp_content_sha256(content TEXT) RETURNS BYTEA
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
RETURN pg_catalog.sha256(pg_catalog.convert_to(content, 'UTF8'));

ALTER TABLE notes ADD COLUMN content_sha256 BYTEA NOT NULL
    GENERATED ALWAYS AS (markupp_content_sha256(content)) STORED;

-- A coluna gerada só existe no NEW de um trigger AFTER, que é o caso deste.
DROP TRIGGER notes_enqueue_on_update ON notes;
CREATE TRIGGER notes_enqueue_on_update AFTER UPDATE OF content ON notes
    FOR EACH ROW WHEN (OLD.content_sha256 IS DISTINCT FROM NEW.content_sha256)
    EXECUTE FUNCTION markupp_enqueue_note();

-- +goose Down
DROP TRIGGER IF EXISTS notes_enqueue_on_update ON notes;
CREATE TRIGGER notes_enqueue_on_update AFTER UPDATE OF content ON notes
    FOR EACH ROW WHEN (OLD.content IS DISTINCT FROM NEW.content)
    EXECUTE FUNCTION markupp_enqueue_note();
ALTER TABLE notes DROP COLUMN IF EXISTS content_sha256;
DROP FUNCTION IF EXISTS markupp_content_sha256(TEXT);
