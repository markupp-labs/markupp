-- +goose Up
-- Ligação por sentido (ADR-0053): o indexador de embedding grava o vetor de cada nota
-- no pgvector (ADR-0055), e o segundo estágio liga os pares do kNN mútuo do cofre.
-- O banco externo precisa ter o pgvector instalado; quem cria a extensão é superusuário.
CREATE EXTENSION IF NOT EXISTS vector;

-- Fila própria do indexador de embedding, separada da do indexador de texto: cada um
-- tira a nota da sua quando termina. Mesma revisão por escrita da note_index_queue.
CREATE SEQUENCE note_embedding_revision_seq;
CREATE TABLE note_embedding_queue (
    note_id  TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    revision BIGINT NOT NULL DEFAULT nextval('note_embedding_revision_seq')
);
ALTER SEQUENCE note_embedding_revision_seq OWNED BY note_embedding_queue.revision;
INSERT INTO note_embedding_queue (note_id) SELECT id FROM notes;

-- SECURITY DEFINER porque a nota é escrita no papel markupp_app, que não enxerga a fila.
-- +goose StatementBegin
CREATE FUNCTION markupp_enqueue_note_embedding() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    INSERT INTO note_embedding_queue (note_id) VALUES (NEW.id)
    ON CONFLICT (note_id) DO UPDATE SET revision = nextval('note_embedding_revision_seq');
    RETURN NULL;
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_enqueue_note_embedding() FROM PUBLIC;
CREATE TRIGGER notes_enqueue_embedding_on_insert AFTER INSERT ON notes
    FOR EACH ROW EXECUTE FUNCTION markupp_enqueue_note_embedding();
CREATE TRIGGER notes_enqueue_embedding_on_update AFTER UPDATE OF content ON notes
    FOR EACH ROW WHEN (OLD.content_sha256 IS DISTINCT FROM NEW.content_sha256)
    EXECUTE FUNCTION markupp_enqueue_note_embedding();

-- Vetor de cada nota, a média normalizada dos pedaços (ADR-0056). A dimensão vem do
-- modelo em uso, então a coluna nasce sem ela e markupp_use_embedding_model a fixa.
CREATE TABLE note_embeddings (
    note_id   TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    vault_id  TEXT NOT NULL,
    embedding vector NOT NULL
);
CREATE INDEX note_embeddings_vault_idx ON note_embeddings (vault_id);

-- O modelo que gerou os vetores gravados: provedor, modelo e arquivo, e a dimensão.
CREATE TABLE embedding_model (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    name      TEXT NOT NULL,
    dimension INTEGER NOT NULL CHECK (dimension >= 0)
);

-- Troca o modelo em uso. Outro modelo apaga os vetores e as ligações por sentido, fixa
-- a dimensão da coluna, refaz o índice HNSW de cosseno e reenfileira todas as notas.
-- Dimensão zero é a instalação sem provedor. Devolve se trocou. Roda no papel dono.
-- +goose StatementBegin
CREATE FUNCTION markupp_use_embedding_model(p_name TEXT, p_dimension INTEGER) RETURNS BOOLEAN
LANGUAGE plpgsql SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM embedding_model WHERE name = p_name AND dimension = p_dimension) THEN
        RETURN false;
    END IF;
    TRUNCATE note_embeddings;
    WITH apagadas AS (DELETE FROM note_automatic_links WHERE kind = 'semantic' RETURNING vault_id)
    INSERT INTO vault_second_stage_queue (vault_id) SELECT DISTINCT vault_id FROM apagadas
    ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');
    DROP INDEX IF EXISTS note_embeddings_hnsw_idx;
    IF p_dimension = 0 THEN
        ALTER TABLE note_embeddings ALTER COLUMN embedding TYPE vector;
    ELSE
        EXECUTE format('ALTER TABLE note_embeddings ALTER COLUMN embedding TYPE vector(%s)', p_dimension);
    END IF;
    -- O HNSW do pgvector indexa até 2000 dimensões; acima, o kNN lê o cofre inteiro.
    IF p_dimension BETWEEN 1 AND 2000 THEN
        CREATE INDEX note_embeddings_hnsw_idx ON note_embeddings USING hnsw (embedding vector_cosine_ops);
    END IF;
    INSERT INTO note_embedding_queue (note_id) SELECT id FROM notes
    ON CONFLICT (note_id) DO UPDATE SET revision = nextval('note_embedding_revision_seq');
    INSERT INTO embedding_model (name, dimension) VALUES (p_name, p_dimension)
    ON CONFLICT (singleton) DO UPDATE SET name = EXCLUDED.name, dimension = EXCLUDED.dimension;
    RETURN true;
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_use_embedding_model(TEXT, INTEGER) FROM PUBLIC;

-- O indexador de embedding lê a fila e grava os vetores de um cofre só, o da sessão;
-- o segundo estágio lê os vetores para o kNN.
GRANT SELECT, DELETE ON note_embedding_queue TO markupp_indexer;
GRANT SELECT, INSERT, DELETE ON note_embeddings TO markupp_indexer;

ALTER TABLE note_embedding_queue ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_embedding_queue_indexer ON note_embedding_queue TO markupp_indexer
    USING (EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));

ALTER TABLE note_embeddings ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_embeddings_indexer ON note_embeddings TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true)
           AND EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));

-- +goose Down
DELETE FROM note_automatic_links WHERE kind = 'semantic';
DROP FUNCTION IF EXISTS markupp_use_embedding_model(TEXT, INTEGER);
DROP TABLE IF EXISTS embedding_model;
DROP TABLE IF EXISTS note_embeddings;
DROP TRIGGER IF EXISTS notes_enqueue_embedding_on_update ON notes;
DROP TRIGGER IF EXISTS notes_enqueue_embedding_on_insert ON notes;
DROP FUNCTION IF EXISTS markupp_enqueue_note_embedding();
DROP TABLE IF EXISTS note_embedding_queue;
DROP EXTENSION IF EXISTS vector;
