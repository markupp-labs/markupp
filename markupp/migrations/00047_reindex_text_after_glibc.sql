-- +goose NO TRANSACTION
-- +goose Up
-- A imagem do Postgres trocou Alpine (musl) por Debian (glibc) na rc.7 (ADR-0055), e as
-- duas ordenam texto de jeitos diferentes. Refaz os índices que dependem da ordenação
-- do texto e registra a versão da collation, para o Postgres avisar numa próxima troca.
-- Índices sem collation (HNSW, tsvector, números) ficam como estão.
-- Sem transação para cada índice confirmar sozinho, sem segurar o lock de todas as
-- tabelas até o fim. Rodar de novo só refaz os índices outra vez.
-- +goose StatementBegin
DO $$
DECLARE
    idx regclass;
BEGIN
    FOR idx IN
        SELECT i.indexrelid::regclass FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
        WHERE c.relnamespace = current_schema()::regnamespace
          AND EXISTS (SELECT 1 FROM unnest(i.indcollation) AS coll
                      WHERE coll NOT IN (0, 'pg_catalog."C"'::regcollation, 'pg_catalog."POSIX"'::regcollation))
    LOOP
        EXECUTE format('REINDEX INDEX %s', idx);
        COMMIT;
    END LOOP;
END
$$;
-- +goose StatementEnd

-- REFRESH COLLATION VERSION recusa a troca a partir de vazio, e o banco criado no musl
-- não tem versão: nesse caso só o superusuário grava direto no catálogo. Sem ele, o
-- upgrade segue e o aviso diz o comando.
-- +goose StatementBegin
DO $$
DECLARE
    gravada TEXT;
    atual TEXT;
BEGIN
    SELECT datcollversion, pg_database_collation_actual_version(oid) INTO gravada, atual
    FROM pg_database WHERE datname = current_database();
    IF atual IS NULL OR gravada IS NOT DISTINCT FROM atual THEN
        RETURN;
    END IF;
    IF gravada IS NOT NULL THEN
        EXECUTE format('ALTER DATABASE %I REFRESH COLLATION VERSION', current_database());
        RETURN;
    END IF;
    IF current_setting('is_superuser') <> 'on' THEN
        RAISE WARNING 'database % has no collation version recorded, expected %; as a superuser run: UPDATE pg_database SET datcollversion = pg_database_collation_actual_version(oid) WHERE datname = %',
            current_database(), atual, quote_literal(current_database());
        RETURN;
    END IF;
    UPDATE pg_database SET datcollversion = atual WHERE datname = current_database();
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Índice refeito e versão registrada não têm o que desfazer.
