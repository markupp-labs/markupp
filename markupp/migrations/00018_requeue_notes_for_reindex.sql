-- +goose Up
-- Reenfileira as notas do cofre target_vault, ou da instalação inteira com NULL, e
-- devolve quantas entraram na fila. Serve à migração que muda o documento do índice e
-- ao subcomando reindex, que rodam no papel dono do banco. A revisão nova faz o
-- indexador que já leu a nota deixá-la na fila. Nenhum papel de tenant ou do KEDA
-- executa: a chamada atravessa todos os tenants.
-- +goose StatementBegin
CREATE FUNCTION markupp_requeue_notes(target_vault TEXT) RETURNS BIGINT
LANGUAGE sql VOLATILE SET search_path = pg_catalog, public, pg_temp AS $$
    WITH requeued AS (
        INSERT INTO note_index_queue (note_id)
        SELECT id FROM notes WHERE target_vault IS NULL OR vault_id = target_vault
        ON CONFLICT (note_id) DO UPDATE SET revision = nextval('note_revision_seq')
        RETURNING 1
    )
    SELECT count(*) FROM requeued
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_requeue_notes(TEXT) FROM PUBLIC;

-- +goose Down
DROP FUNCTION IF EXISTS markupp_requeue_notes(TEXT);
