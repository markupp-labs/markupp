-- +goose Up
-- Centralidade e comunidades das notas, que o segundo estágio recalcula sobre o grafo
-- inteiro do cofre, ligações inferidas incluídas. O tenant vem do cofre, pelas
-- políticas, como em note_titles.

-- PageRank (d=0,85) e graus de entrada e de saída, em notas distintas.
CREATE TABLE note_centrality (
    note_id    TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    vault_id   TEXT NOT NULL,
    pagerank   DOUBLE PRECISION NOT NULL,
    in_degree  INTEGER NOT NULL,
    out_degree INTEGER NOT NULL
);
CREATE INDEX note_centrality_vault_rank_idx ON note_centrality (vault_id, pagerank DESC);
-- Nota órfã é a de grau zero: o filtro orphan=true lê só estas linhas do cofre.
CREATE INDEX note_centrality_vault_orphan_idx ON note_centrality (vault_id) WHERE in_degree = 0 AND out_degree = 0;

-- Comunidade de cada nota pelo Louvain. O id vem da sequência quando a comunidade não
-- herda o de uma anterior, então nunca volta a ser usado. Nota sozinha não tem linha.
CREATE SEQUENCE note_community_id_seq;
CREATE TABLE note_communities (
    note_id      TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    vault_id     TEXT NOT NULL,
    community_id BIGINT NOT NULL
);
CREATE INDEX note_communities_vault_idx ON note_communities (vault_id, community_id);

GRANT SELECT, INSERT, DELETE ON note_centrality, note_communities TO markupp_indexer;
GRANT USAGE ON SEQUENCE note_community_id_seq TO markupp_indexer;
GRANT SELECT ON note_centrality, note_communities TO markupp_app;

ALTER TABLE note_centrality ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_centrality_indexer ON note_centrality TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true)
           AND EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));
CREATE POLICY note_centrality_read ON note_centrality FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

ALTER TABLE note_communities ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_communities_indexer ON note_communities TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true)
           AND EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));
CREATE POLICY note_communities_read ON note_communities FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

-- Nenhum cofre tem centralidade nem comunidade ainda.
INSERT INTO vault_second_stage_queue (vault_id) SELECT id FROM vaults
ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');

-- +goose Down
DROP TABLE IF EXISTS note_communities;
DROP SEQUENCE IF EXISTS note_community_id_seq;
DROP TABLE IF EXISTS note_centrality;
