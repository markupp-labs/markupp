package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// antesDaMigracaoDeCollation devolve um banco migrado até a 00046, a última antes da
// que refaz os índices de texto (ADR-0055).
func antesDaMigracaoDeCollation(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	_, err := gooseProvider(t, pool).UpTo(context.Background(), 46)
	require.NoError(t, err)
	return pool
}

// arquivosDosIndices mapeia cada índice do schema ao relfilenode atual e diz se ele
// depende da ordenação do texto. REINDEX troca o relfilenode.
const arquivosDosIndices = `SELECT c.relname, c.relfilenode,
	EXISTS (SELECT 1 FROM unnest(i.indcollation) AS coll
	        WHERE coll NOT IN (0, 'pg_catalog."C"'::regcollation, 'pg_catalog."POSIX"'::regcollation))
	FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
	WHERE c.relnamespace = current_schema()::regnamespace`

type arquivoDoIndice struct {
	relfilenode uint32
	deTexto     bool
}

func arquivosAtuais(t *testing.T, pool *pgxpool.Pool) map[string]arquivoDoIndice {
	t.Helper()
	rows, err := pool.Query(context.Background(), arquivosDosIndices)
	require.NoError(t, err)
	defer rows.Close()
	arquivos := map[string]arquivoDoIndice{}
	for rows.Next() {
		var nome string
		var a arquivoDoIndice
		require.NoError(t, rows.Scan(&nome, &a.relfilenode, &a.deTexto))
		arquivos[nome] = a
	}
	require.NoError(t, rows.Err())
	return arquivos
}

func versaoDaCollation(t *testing.T, pool *pgxpool.Pool) (gravada, atual *string) {
	t.Helper()
	err := pool.QueryRow(context.Background(), `SELECT datcollversion, pg_database_collation_actual_version(oid)
		FROM pg_database WHERE datname = current_database()`).Scan(&gravada, &atual)
	require.NoError(t, err)
	require.NotNil(t, atual, "a imagem de teste precisa de collation glibc versionada")
	return gravada, atual
}

func gravaVersaoDaCollation(t *testing.T, pool *pgxpool.Pool, versao *string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		"UPDATE pg_database SET datcollversion = $1 WHERE datname = current_database()", versao)
	require.NoError(t, err)
}

func TestMigracao00047_RefazSoOsIndicesDeTexto(t *testing.T) {
	pool := antesDaMigracaoDeCollation(t)
	antes := arquivosAtuais(t, pool)
	require.Contains(t, antes, "publications_folder_key")

	_, err := gooseProvider(t, pool).Up(context.Background())

	require.NoError(t, err)
	depois := arquivosAtuais(t, pool)
	for nome, a := range antes {
		mudou := depois[nome].relfilenode != a.relfilenode
		assert.Equal(t, a.deTexto, mudou, "indice %s (de texto: %v)", nome, a.deTexto)
	}
}

func TestMigracao00047_VersaoVaziaComSuperusuario_RegistraAVersaoAtual(t *testing.T) {
	pool := antesDaMigracaoDeCollation(t)
	gravaVersaoDaCollation(t, pool, nil)

	_, err := gooseProvider(t, pool).Up(context.Background())

	require.NoError(t, err)
	gravada, atual := versaoDaCollation(t, pool)
	assert.Equal(t, atual, gravada, "o banco criado no musl passa a ter versao, e o Postgres avisa na proxima troca")
}

func TestMigracao00047_VersaoDiferente_AtualizaParaAAtual(t *testing.T) {
	pool := antesDaMigracaoDeCollation(t)
	antiga := "0.0"
	gravaVersaoDaCollation(t, pool, &antiga)

	_, err := gooseProvider(t, pool).Up(context.Background())

	require.NoError(t, err)
	gravada, atual := versaoDaCollation(t, pool)
	assert.Equal(t, atual, gravada)
}

// comoDonoSemSuperusuario abre um pool no mesmo banco com um papel que só pode refazer
// índices (pg_maintain) e gravar o controle do goose, como o dono de um banco externo.
func comoDonoSemSuperusuario(t *testing.T, pool *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'markupp_teste_dono') THEN
			CREATE ROLE markupp_teste_dono NOLOGIN NOSUPERUSER;
		END IF;
	END $$`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `GRANT pg_maintain TO markupp_teste_dono;
		GRANT ALL ON goose_db_version TO markupp_teste_dono;
		GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO markupp_teste_dono`)
	require.NoError(t, err)
	cfg := pool.Config().Copy()
	cfg.ConnConfig.RuntimeParams["role"] = "markupp_teste_dono"
	dono, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(dono.Close)
	return dono
}

func TestMigracao00047_VersaoVaziaSemSuperusuario_RefazIndicesESegueSemRegistrar(t *testing.T) {
	pool := antesDaMigracaoDeCollation(t)
	gravaVersaoDaCollation(t, pool, nil)
	antes := arquivosAtuais(t, pool)
	dono := comoDonoSemSuperusuario(t, pool)

	_, err := gooseProvider(t, dono).UpTo(context.Background(), 47)

	require.NoError(t, err, "a falta de superusuario nao trava o upgrade")
	assert.NotEqual(t, antes["publications_folder_key"].relfilenode, arquivosAtuais(t, pool)["publications_folder_key"].relfilenode)
	gravada, _ := versaoDaCollation(t, pool)
	assert.Nil(t, gravada, "so o superusuario grava a versao a partir de vazio")
}
