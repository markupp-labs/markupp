-- +goose Up
-- Termos de cada nota para a similaridade léxica: os de maior tf-idf, no analisador da
-- busca, com o tf e o peso tf-idf de cada um na mesma posição, e a norma do vetor dos
-- termos gravados. Termos e pesos são a lista de postagens da nota: o cosseno lê só
-- esta tabela. O lote pesa com o df e o N vigentes, e o segundo estágio repesa o cofre
-- inteiro quando os recalcula. Uma linha por nota, e não por termo, porque a checagem
-- de chave estrangeira e de RLS por linha dominava o lote. O tenant vem do cofre,
-- pelas políticas, como em note_titles.
CREATE TABLE note_terms (
    note_id  TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    vault_id TEXT NOT NULL,
    terms    TEXT[] NOT NULL,
    tfs      DOUBLE PRECISION[] NOT NULL CHECK (cardinality(tfs) = cardinality(terms)),
    weights  DOUBLE PRECISION[] NOT NULL CHECK (cardinality(weights) = cardinality(terms)),
    norm     DOUBLE PRECISION NOT NULL
);
-- O GIN acha as notas que dividem termos com outra.
CREATE INDEX note_terms_terms_idx ON note_terms USING GIN (terms);

-- idf = log10(notas do cofre / df). Termo ausente do df apareceu depois do último
-- segundo estágio e conta como de uma nota só; cofre ainda sem df pesa só pelo tf. df
-- defasado acima do total de notas não dá peso negativo.
CREATE FUNCTION markupp_term_idf(note_count BIGINT, df INTEGER, df_known BOOLEAN) RETURNS DOUBLE PRECISION
LANGUAGE sql IMMUTABLE PARALLEL SAFE
RETURN CASE WHEN df_known THEN greatest(log(greatest(note_count, 1)::double precision / coalesce(df, 1)), 0) ELSE 1 END;

GRANT SELECT, INSERT, UPDATE, DELETE ON note_terms TO markupp_indexer;
GRANT SELECT ON note_terms TO markupp_app;

ALTER TABLE note_terms ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_terms_indexer ON note_terms TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true)
           AND EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));
CREATE POLICY note_terms_read ON note_terms FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

-- As notas já indexadas não têm termos: todas voltam para a fila.
SELECT markupp_requeue_notes(NULL);

-- +goose Down
DROP FUNCTION IF EXISTS markupp_term_idf(BIGINT, INTEGER, BOOLEAN);
DROP TABLE IF EXISTS note_terms;
