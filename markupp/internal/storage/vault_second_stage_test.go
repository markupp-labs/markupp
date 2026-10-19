package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// frequenciasDoCofre lê, com o papel dono do banco, o df gravado pelo segundo
// estágio para o cofre vaultID, como termo=df.
func frequenciasDoCofre(t *testing.T, db *pgxpool.Pool, vaultID string) map[string]int {
	t.Helper()
	rows, err := db.Query(context.Background(), "SELECT term, df FROM vault_term_frequencies WHERE vault_id = $1", vaultID)
	require.NoError(t, err)
	df := map[string]int{}
	for rows.Next() {
		var term string
		var n int
		require.NoError(t, rows.Scan(&term, &n))
		df[term] = n
	}
	require.NoError(t, rows.Err())
	return df
}

const cofresMarcados = "SELECT count(*) FROM vault_second_stage_queue"

// cofreComDf indexa duas notas que dividem o termo alfa e devolve o banco e o
// repositório.
func cofreComDf(t *testing.T) (*pgxpool.Pool, *storage.PostgresNotesRepository) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "n1.md", "alfa beta"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n2", "n2.md", "alfa gama"))
	indexaTodosOsCofres(t, db, "simple")
	return db, repo
}

func TestSegundoEstagio_NotasNovas_GravaODfDoCofre(t *testing.T) {
	db, _ := cofreComDf(t)

	df := frequenciasDoCofre(t, db, cofreDeTeste)

	assert.Equal(t, 2, df["alfa"])
	assert.Equal(t, 1, df["beta"])
	assert.Equal(t, 1, df["n1"], "o titulo entra no documento e no df")
	assert.Zero(t, countRows(t, db, cofresMarcados), "o segundo estagio desmarca o cofre")
	assert.Zero(t, countRows(t, db, consultaDoScaler))
}

func TestSegundoEstagio_ConteudoSemMudarLigacoes_NaoRecalcula(t *testing.T) {
	db, repo := cofreComDf(t)

	_, err := repo.Update(ctxDeTeste, "n1", "n1.md", "alfa beta delta", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	assert.NotContains(t, frequenciasDoCofre(t, db, cofreDeTeste), "delta", "nem ligacao nem titulo mudou")
}

func TestSegundoEstagio_LoteQueMudaLigacao_Recalcula(t *testing.T) {
	db, repo := cofreComDf(t)

	_, err := repo.Update(ctxDeTeste, "n1", "n1.md", "alfa beta delta [[n2]]", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, 1, frequenciasDoCofre(t, db, cofreDeTeste)["delta"])
}

func TestSegundoEstagio_NotaRenomeada_Recalcula(t *testing.T) {
	db, repo := cofreComDf(t)

	_, err := repo.Update(ctxDeTeste, "n1", "outro.md", "alfa beta", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	df := frequenciasDoCofre(t, db, cofreDeTeste)
	assert.Equal(t, 1, df["outro"])
	assert.NotContains(t, df, "n1")
}

func TestSegundoEstagio_NotaApagada_MarcaOCofreParaOScaler(t *testing.T) {
	db, repo := cofreComDf(t)

	require.NoError(t, repo.Delete(ctxDeTeste, "n2"))

	assert.Equal(t, 1, countRows(t, db, consultaDoScaler), "o KEDA dispara o Job do cofre marcado")
	found, _ := indexaProximo(t, novoIndexador(t, db, "simple"))
	assert.True(t, found, "cofre so marcado tambem e reservado")
	df := frequenciasDoCofre(t, db, cofreDeTeste)
	assert.Equal(t, 1, df["alfa"])
	assert.NotContains(t, df, "gama")
	assert.Zero(t, countRows(t, db, consultaDoScaler))
}

func TestSegundoEstagio_CofreApagado_NaoDeixaPendencia(t *testing.T) {
	db, _ := cofreComDf(t)

	_, err := db.Exec(context.Background(), "DELETE FROM vaults WHERE id = $1", cofreDeTeste)

	require.NoError(t, err, "apagar o cofre apaga as notas em cascata")
	assert.Zero(t, countRows(t, db, consultaDoScaler))
}

func TestSegundoEstagio_Reindex_MarcaOCofre(t *testing.T) {
	db, _ := cofreComDf(t)

	_, err := storage.RequeueNotes(context.Background(), db, cofreDeTeste)

	require.NoError(t, err)
	assert.Equal(t, 1, countRows(t, db, cofresMarcados), "trocar a configuracao de texto muda os termos do df")
}

func TestRLS_PapelDoIndexador_NaoGravaDfNemMarcaOutroCofre(t *testing.T) {
	db := setupTestDB(t)
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	storagetest.SeedMember(t, db, ator, "vb")

	for _, sql := range []string{
		"WITH i AS (INSERT INTO vault_term_frequencies (vault_id, term, df) VALUES ('vb', 'x', 1) RETURNING 1) SELECT count(*) FROM i",
		"WITH i AS (INSERT INTO vault_term_stats (vault_id, note_count) VALUES ('vb', 1) RETURNING 1) SELECT count(*) FROM i",
		"WITH i AS (INSERT INTO vault_second_stage_queue (vault_id) VALUES ('vb') RETURNING 1) SELECT count(*) FROM i",
	} {
		_, err := comoIndexador(t, db, "t1", "va", sql)

		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr, sql)
		assert.Equal(t, "42501", pgErr.Code, "recebido %v", err)
	}
}

// grauDeEntrada lê, com o papel dono do banco, o grau de entrada que o segundo
// estágio gravou para a nota noteID.
func grauDeEntrada(t *testing.T, db *pgxpool.Pool, noteID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT in_degree FROM note_centrality WHERE note_id = $1", noteID).Scan(&n), "grau da nota %q", noteID)
	return n
}

func TestSegundoEstagio_NotaMovidaDePasta_Recalcula(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("x1", "a/alvo.md", "x"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("x2", "b/alvo.md", "x"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("o", "o.md", "[l](a/alvo.md)"))
	indexaTodosOsCofres(t, db, "simple")
	require.Zero(t, grauDeEntrada(t, db, "x2"), "o caminho citado existe e liga so a x1")

	_, err := repo.Update(ctxDeTeste, "x1", "c/alvo.md", "x", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, 1, grauDeEntrada(t, db, "x2"), "o titulo nao mudou, mas o caminho citado sumiu e a ligacao cai para o titulo")
}
