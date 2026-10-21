-- +goose Up
-- Versão do grafo de cada cofre: a revisão da marcação que o último segundo estágio
-- leu. Muda só quando ele roda, junto com o que ele grava, e serve de validador das
-- leituras do grafo. O tenant vem do cofre, pelas políticas, como em note_titles.
CREATE TABLE vault_graph_versions (
    vault_id TEXT PRIMARY KEY REFERENCES vaults (id) ON DELETE CASCADE,
    version  BIGINT NOT NULL
);

GRANT SELECT, INSERT, UPDATE ON vault_graph_versions TO markupp_indexer;
GRANT SELECT ON vault_graph_versions TO markupp_app;

ALTER TABLE vault_graph_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY vault_graph_versions_indexer ON vault_graph_versions TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true));
CREATE POLICY vault_graph_versions_read ON vault_graph_versions FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

-- +goose Down
DROP TABLE IF EXISTS vault_graph_versions;
