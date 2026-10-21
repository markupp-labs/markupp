-- +goose Up
-- O título da nota vem do caminho, então renomear também pede reindexação.
DROP TRIGGER notes_enqueue_on_update ON notes;
CREATE TRIGGER notes_enqueue_on_update AFTER UPDATE OF content, path ON notes
    FOR EACH ROW WHEN (OLD.content_sha256 IS DISTINCT FROM NEW.content_sha256
                       OR OLD.path IS DISTINCT FROM NEW.path)
    EXECUTE FUNCTION markupp_enqueue_note();

-- +goose Down
DROP TRIGGER IF EXISTS notes_enqueue_on_update ON notes;
CREATE TRIGGER notes_enqueue_on_update AFTER UPDATE OF content ON notes
    FOR EACH ROW WHEN (OLD.content_sha256 IS DISTINCT FROM NEW.content_sha256)
    EXECUTE FUNCTION markupp_enqueue_note();
