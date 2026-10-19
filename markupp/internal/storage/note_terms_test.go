package storage_test

import (
	"context"
	"fmt"
	"math"
	"strings"
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

// termosDaNota lê, com o papel dono do banco, os termos gravados para a nota noteID
// com o tf de cada um.
func termosDaNota(t *testing.T, db *pgxpool.Pool, noteID string) map[string]float64 {
	t.Helper()
	rows, err := db.Query(context.Background(), "SELECT u.term, u.tf FROM note_terms t, unnest(t.terms, t.tfs) AS u (term, tf) WHERE t.note_id = $1", noteID)
	require.NoError(t, err)
	termos := map[string]float64{}
	for rows.Next() {
		var term string
		var tf float64
		require.NoError(t, rows.Scan(&term, &tf))
		termos[term] = tf
	}
	require.NoError(t, rows.Err())
	return termos
}

// postagensDaNota lê, com o papel dono do banco, o peso tf-idf gravado de cada termo
// da nota noteID e a norma gravada do vetor dela.
func postagensDaNota(t *testing.T, db *pgxpool.Pool, noteID string) (map[string]float64, float64) {
	t.Helper()
	var termos []string
	var pesos []float64
	var norma float64
	err := db.QueryRow(context.Background(), "SELECT terms, weights, norm FROM note_terms WHERE note_id = $1", noteID).Scan(&termos, &pesos, &norma)
	require.NoError(t, err, "postagens da nota %q", noteID)
	porTermo := make(map[string]float64, len(termos))
	for i, termo := range termos {
		porTermo[termo] = pesos[i]
	}
	return porTermo, norma
}

// notasDoDf lê o N que o segundo estágio gravou junto com o df do cofre vaultID.
func notasDoDf(t *testing.T, db *pgxpool.Pool, vaultID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(context.Background(), "SELECT note_count FROM vault_term_stats WHERE vault_id = $1", vaultID).Scan(&n))
	return n
}

// palavrasDistintas devolve n palavras diferentes, cada uma uma vez.
func palavrasDistintas(prefixo string, n int) string {
	palavras := make([]string, 0, n)
	for i := range n {
		palavras = append(palavras, fmt.Sprintf("%s%02d", prefixo, i))
	}
	return strings.Join(palavras, " ")
}

func TestVaultIndexer_SemDf_GravaOs50TermosDeMaiorTf(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "n1.md", "comum comum comum "+palavrasDistintas("w", 60)))

	indexaTodosOsCofres(t, db, "simple")

	termos := termosDaNota(t, db, "n1")
	assert.Len(t, termos, 50)
	assert.InDelta(t, 1+math.Log10(3), termos["comum"], 1e-9, "tf = 1 + log10 das ocorrencias")
	assert.InDelta(t, 1.0, termos["w00"], 1e-9)
}

func TestVaultIndexer_ComDf_TermoDeTodasAsNotasPerdeLugar(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "n1.md", "comum"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n2", "n2.md", "comum comum comum "+palavrasDistintas("w", 60)))
	indexaTodosOsCofres(t, db, "simple")
	require.Contains(t, termosDaNota(t, db, "n2"), "comum", "sem df o tf puro escolhe o termo repetido")

	_, err := repo.Update(ctxDeTeste, "n2", "n2.md", "comum comum comum "+palavrasDistintas("w", 61), time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	termos := termosDaNota(t, db, "n2")
	assert.Len(t, termos, 50)
	assert.NotContains(t, termos, "comum", "idf zero: o termo esta em todas as notas do cofre")
}

func TestSegundoEstagio_GravaONEOsPesosENormasComODfNovo(t *testing.T) {
	db, _ := cofreComDf(t)

	pesos, norma := postagensDaNota(t, db, "n1")

	assert.Equal(t, 2, notasDoDf(t, db, cofreDeTeste))
	assert.InDelta(t, 0.0, pesos["alfa"], 1e-9, "alfa esta nas duas notas")
	assert.InDelta(t, math.Log10(2), pesos["beta"], 1e-9, "tf 1 vezes log10(2/1)")
	assert.InDelta(t, math.Log10(2), pesos["n1"], 1e-9)
	assert.InDelta(t, math.Sqrt2*math.Log10(2), norma, 1e-9, "norma do vetor dos termos gravados")
}

func TestVaultIndexer_LoteSemSegundoEstagio_PesaComODfEONGravados(t *testing.T) {
	db, repo := cofreComDf(t)

	_, err := repo.Update(ctxDeTeste, "n1", "n1.md", "alfa beta zeta zeta", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	pesos, norma := postagensDaNota(t, db, "n1")
	zeta := (1 + math.Log10(2)) * math.Log10(2)
	assert.InDelta(t, zeta, pesos["zeta"], 1e-9, "termo sem df conta como df=1")
	assert.InDelta(t, 0.0, pesos["alfa"], 1e-9)
	assert.InDelta(t, math.Sqrt(2*math.Pow(math.Log10(2), 2)+zeta*zeta), norma, 1e-9)
}

func TestVaultIndexer_Termos_UsamOAnalisadorDaBusca(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "pg_catalog.portuguese")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "n1.md", "correndo"))
	var radical string
	require.NoError(t, db.QueryRow(context.Background(),
		"SELECT lexeme FROM unnest(to_tsvector('pg_catalog.portuguese', 'correndo'))").Scan(&radical))

	indexaTodosOsCofres(t, db, "pg_catalog.portuguese")

	assert.Contains(t, termosDaNota(t, db, "n1"), radical)
}

func TestRLS_PapelDoIndexador_NaoGravaTermosDeOutroCofre(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ator := identity.Actor{UserID: "ua", TenantID: "t1", Role: "member"}
	storagetest.SeedMember(t, db, ator, "va")
	ctx := storagetest.SeedMember(t, db, ator, "vb")
	storagetest.SaveNote(ctx, t, repo, notaComConteudo("n2", "vb", "dois"))

	_, err := comoIndexador(t, db, "t1", "va",
		"WITH i AS (INSERT INTO note_terms (note_id, vault_id, terms, tfs, weights, norm) VALUES ('n2', 'vb', '{x}', '{1}', '{1}', 1) RETURNING 1) SELECT count(*) FROM i")

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "42501", pgErr.Code, "recebido %v", err)
}

func TestVaultIndexer_DocumentoSemLexema_NaoGravaTermosNemTravaOCofre(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "--.md", "..."))

	indexaTodosOsCofres(t, db, "simple")

	assert.Empty(t, termosDaNota(t, db, "n1"))
	assert.Zero(t, countRows(t, db, consultaDoScaler))
}
