package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// marcaCofreDeTeste marca cofreDeTeste para o segundo estágio, como a exclusão de nota
// faria.
const marcaCofreDeTeste = `INSERT INTO vault_second_stage_queue (vault_id) VALUES ('cofre-teste')
ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq')`

// derivadoDoCofre resume, com o papel dono do banco, tudo que o segundo estágio grava
// para cofreDeTeste, em ordem fixa. O PageRank iterativo só converge até a tolerância,
// e a ordem da soma varia entre execuções: vale até a nona casa.
const derivadoDoCofre = `SELECT concat_ws(' | ',
    (SELECT string_agg(concat_ws(':', note_id, round(pagerank::numeric, 9), in_degree, out_degree), ',' ORDER BY note_id) FROM note_centrality),
    (SELECT string_agg(concat_ws(':', note_id, community_id), ',' ORDER BY note_id) FROM note_communities),
    (SELECT string_agg(concat_ws(':', note_id, title_key, target_path), ',' ORDER BY note_id, title_key) FROM note_links WHERE kind = 'inferred'),
    (SELECT string_agg(concat_ws(':', note_id, weights::text, norm), ',' ORDER BY note_id) FROM note_terms),
    (SELECT string_agg(concat_ws(':', term, df), ',' ORDER BY term) FROM vault_term_frequencies),
    (SELECT string_agg(concat_ws(':', vault_id, note_count), ',' ORDER BY vault_id) FROM vault_term_stats))`

func lerDerivado(t *testing.T, db *pgxpool.Pool) string {
	t.Helper()
	var resumo string
	require.NoError(t, db.QueryRow(context.Background(), derivadoDoCofre).Scan(&resumo))
	return resumo
}

func TestSegundoEstagio_RodarDeNovoSemMudanca_GravaOMesmo(t *testing.T) {
	db, _, _ := cofreComDoisGrupos(t)
	antes := lerDerivado(t, db)

	_, err := db.Exec(context.Background(), marcaCofreDeTeste)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, antes, lerDerivado(t, db))
	assert.Zero(t, countRows(t, db, cofresMarcados))
}

// seguraCentralidade trava as linhas de note_centrality numa transação do dono do
// banco, para o segundo estágio parar ao regravá-las, e devolve a função que solta.
func seguraCentralidade(t *testing.T, db *pgxpool.Pool) func() {
	t.Helper()
	tx, err := db.Begin(context.Background())
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), "SELECT 1 FROM note_centrality FOR UPDATE")
	require.NoError(t, err)
	solta := func() { _ = tx.Rollback(context.Background()) }
	t.Cleanup(solta)
	return solta
}

// esperaTravaPendente espera alguma sessão do banco esperar por uma trava.
func esperaTravaPendente(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	require.Eventually(t, func() bool {
		return countRows(t, db, "SELECT count(*) FROM pg_locks WHERE NOT granted") > 0
	}, 10*time.Second, 20*time.Millisecond, "o segundo estagio deveria parar na centralidade travada")
}

// segundoEstagioPreso marca cofreDeTeste e roda o Job numa goroutine que para no meio
// do segundo estágio, com a centralidade travada até solta ser chamada.
func segundoEstagioPreso(t *testing.T, db *pgxpool.Pool, idx *storage.PostgresVaultIndexer) (solta func(), terminou <-chan error) {
	t.Helper()
	_, err := db.Exec(context.Background(), marcaCofreDeTeste)
	require.NoError(t, err)
	solta = seguraCentralidade(t, db)
	fim := make(chan error, 1)
	go func() {
		_, err := idx.IndexNextVault(context.Background(), indexing.Index)
		fim <- err
	}()
	esperaTravaPendente(t, db)
	return solta, fim
}

func TestSegundoEstagio_EmAndamento_NaoPrendeOCofreParaOLote(t *testing.T) {
	db, repo, _ := cofreComDoisGrupos(t)
	idx := novoIndexador(t, db, "simple")
	solta, terminou := segundoEstagioPreso(t, db, idx)

	_, err := repo.Update(ctxDeTeste, "solta", "solta.md", "agora [[a]] alfaomega", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	found, _ := indexaProximo(t, idx)

	require.True(t, found, "o lote reserva o cofre durante o segundo estagio de outro Job")
	assert.Equal(t, 1, casaNoIndice(t, db, "solta", "simple", "alfaomega"))
	found, _ = indexaProximo(t, idx)
	assert.False(t, found, "o cofre so marcado com segundo estagio em andamento nao e reservado")
	solta()
	require.NoError(t, <-terminou)
	assert.Equal(t, 1, countRows(t, db, cofresMarcados), "o lote concorrente mudou ligacoes depois da leitura")
	indexaTodosOsCofres(t, db, "simple")
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM note_centrality WHERE note_id = 'solta' AND out_degree = 1"))
}

// salvaNotas grava n notas de conteúdo curto em cofreDeTeste.
func salvaNotas(t *testing.T, repo *storage.PostgresNotesRepository, n int) {
	t.Helper()
	for i := range n {
		storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho(fmt.Sprintf("n%03d", i), fmt.Sprintf("n%03d.md", i), "conteudo [[n000]]"))
	}
}

// analisadas conta as tabelas derivadas do lote que já passaram por ANALYZE manual.
func analisadas(t *testing.T, db *pgxpool.Pool) int {
	t.Helper()
	return countRows(t, db, `SELECT count(*) FROM pg_stat_user_tables
WHERE relname IN ('note_search_index', 'note_titles', 'note_links', 'note_terms') AND last_analyze IS NOT NULL`)
}

func TestVaultIndexer_LoteGrande_AnalisaAsTabelasDerivadas(t *testing.T) {
	db := setupNotesDB(t)
	salvaNotas(t, storage.NewPostgresNotesRepository(db, "simple"), 60)

	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, 4, analisadas(t, db), "60 notas passam de 50 + 10% do cofre")
}

func TestVaultIndexer_LotePequeno_NaoAnalisa(t *testing.T) {
	db := setupNotesDB(t)
	salvaNotas(t, storage.NewPostgresNotesRepository(db, "simple"), 5)

	indexaTodosOsCofres(t, db, "simple")

	assert.Zero(t, analisadas(t, db))
}
