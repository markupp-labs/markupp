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
	rows, err := db.Query(context.Background(), "SELECT term, tf FROM note_terms WHERE note_id = $1", noteID)
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
		"WITH i AS (INSERT INTO note_terms (note_id, vault_id, term, tf) VALUES ('n2', 'vb', 'x', 1) RETURNING 1) SELECT count(*) FROM i")

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "42501", pgErr.Code, "recebido %v", err)
}
}

func TestVaultIndexer_DocumentoSemLexema_NaoGravaTermosNemTravaOCofre(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n1", "--.md", "..."))

	indexaTodosOsCofres(t, db, "simple")

	assert.Empty(t, termosDaNota(t, db, "n1"))
	assert.Zero(t, countRows(t, db, consultaDoScaler))
}
