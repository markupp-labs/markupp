-- +goose Up
-- Ligação inferida: o segundo estágio escolhe, para cada ligação não resolvida, a nota
-- mais provável e grava a ligação do tipo inferred em note_links, com o caminho dela.
-- Para isso ele lê o grafo resolvido do cofre, como a aplicação já lê.
GRANT SELECT ON note_edges TO markupp_indexer;

-- Nenhum cofre tem ligação inferida ainda.
INSERT INTO vault_second_stage_queue (vault_id) SELECT id FROM vaults
ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');

-- +goose Down
DELETE FROM note_links WHERE kind = 'inferred';
REVOKE SELECT ON note_edges FROM markupp_indexer;
