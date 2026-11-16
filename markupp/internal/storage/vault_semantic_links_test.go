package storage_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/embedding"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// angulosDasNotas põe seis notas num leque de 0 a 50 graus e longe do outro lado:
// cada nota do leque tem as outras cinco como vizinhas, e longe não é vizinha de
// nenhuma, porque cada uma tem cinco mais perto que ela.
var angulosDasNotas = map[string]float64{"n0": 0, "n10": 10, "n20": 20, "n30": 30, "n40": 40, "n50": 50, "longe": 180}

func vetorNoAngulo(graus float64) []float32 {
	rad := graus * math.Pi / 180
	return []float32{float32(math.Cos(rad)), float32(math.Sin(rad))}
}

// cofreComVetores salva as notas do leque, n10 com ligação escrita para n0, grava o
// vetor de cada uma no modelo de teste e roda o indexador de texto, que liga pelo
// sentido no segundo estágio.
func cofreComVetores(t *testing.T) (*pgxpool.Pool, *storage.PostgresNotesRepository, *storage.PostgresNoteGraph) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for id := range angulosDasNotas {
		storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho(id, id+".md", "texto de "+id))
	}
	_, err := repo.Update(ctxDeTeste, "n10", "n10.md", "[[n0]]", time.Date(2026, 10, 16, 13, 0, 0, 0, time.UTC), time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), false)
	require.NoError(t, err)
	_, err = storage.UseEmbeddingModel(context.Background(), db, "teste", 2)
	require.NoError(t, err)
	gravaVetores(t, db, func(n embedding.PendingNote) []float32 { return vetorNoAngulo(angulosDasNotas[n.NoteID]) })
	indexaTodosOsCofres(t, db, "simple")
	return db, repo, storage.NewPostgresNoteGraph(db, "simple")
}

// gravaVetores esvazia a fila de embedding com o vetor que vetor dá a cada nota.
func gravaVetores(t *testing.T, db *pgxpool.Pool, vetor func(embedding.PendingNote) []float32) {
	t.Helper()
	fila := storage.NewPostgresEmbeddingQueue(db)
	for {
		lote, found, err := fila.NextBatch(context.Background(), 100)
		require.NoError(t, err)
		if !found {
			return
		}
		vetores := make([]embedding.NoteVector, 0, len(lote.Notes))
		for _, n := range lote.Notes {
			vetores = append(vetores, embedding.NoteVector{Note: n, Vector: vetor(n)})
		}
		require.NoError(t, fila.SaveVectors(context.Background(), lote, vetores))
	}
}

func destinosPorTipo(t *testing.T, graph *storage.PostgresNoteGraph, noteID string) map[string][]string {
	t.Helper()
	saida, err := graph.Links(ctxDeTeste, noteID, todas)
	require.NoError(t, err)
	destinos := map[string][]string{}
	for _, l := range saida {
		destinos[l.Type] = append(destinos[l.Type], l.Target.ID)
	}
	return destinos
}

func TestLigacaoPorSentido_LigaOsParesDoKnnMutuoNosDoisSentidos(t *testing.T) {
	_, _, graph := cofreComVetores(t)

	assert.ElementsMatch(t, []string{"n0", "n10", "n20", "n30", "n40"}, destinosPorTipo(t, graph, "n50")[notes.LinkSemantic])
	assert.Contains(t, destinosPorTipo(t, graph, "n40")[notes.LinkSemantic], "n50", "o par mutuo liga nos dois sentidos")
}

func TestLigacaoPorSentido_NotaQueNinguemTemEntreOsVizinhosFicaSemLigacao(t *testing.T) {
	_, repo, graph := cofreComVetores(t)

	assert.Empty(t, destinosPorTipo(t, graph, "longe"))
	assert.Empty(t, backlinks(ctxDeTeste, t, graph, "longe"))
	orfas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Orphan: ptr(true), Limit: 100})
	assert.Equal(t, []string{"longe"}, idsDasNotas(orfas))
}

func TestLigacaoPorSentido_NaoRepeteOParComLigacaoEscrita(t *testing.T) {
	_, _, graph := cofreComVetores(t)

	destinos := destinosPorTipo(t, graph, "n10")
	assert.Equal(t, []string{"n0"}, destinos[notes.LinkExplicit])
	assert.ElementsMatch(t, []string{"n20", "n30", "n40", "n50"}, destinos[notes.LinkSemantic])
	assert.NotContains(t, destinosPorTipo(t, graph, "n0")[notes.LinkSemantic], "n10")
}

func TestFilaDeEmbedding_NotaEditadaDepoisDaLeitura_NaoGravaOVetorVelhoEFicaNaFila(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("n0", "n0.md", "antes"))
	_, err := storage.UseEmbeddingModel(context.Background(), db, "teste", 2)
	require.NoError(t, err)
	fila := storage.NewPostgresEmbeddingQueue(db)
	lote, found, err := fila.NextBatch(context.Background(), 100)
	require.NoError(t, err)
	require.True(t, found)

	_, err = repo.Update(ctxDeTeste, "n0", "n0.md", "depois", time.Date(2026, 10, 16, 13, 0, 0, 0, time.UTC), time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), false)
	require.NoError(t, err)
	require.NoError(t, fila.SaveVectors(context.Background(), lote, []embedding.NoteVector{{Note: lote.Notes[0], Vector: vetorNoAngulo(0)}}))

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM note_embeddings"))
	assert.Equal(t, 1, countRows(t, db, "SELECT count(*) FROM note_embedding_queue"))
}

func TestFilaDeEmbedding_NotaSemTexto_SaiDaFilaSemVetor(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("vazia", "vazia.md", ""))
	_, err := storage.UseEmbeddingModel(context.Background(), db, "teste", 2)
	require.NoError(t, err)

	gravaVetores(t, db, func(embedding.PendingNote) []float32 { return nil })

	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM note_embeddings"))
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM note_embedding_queue"))
}

func TestUseEmbeddingModel_MesmoModelo_NaoTroca(t *testing.T) {
	db, _, _ := cofreComVetores(t)

	trocou, err := storage.UseEmbeddingModel(context.Background(), db, "teste", 2)

	require.NoError(t, err)
	assert.False(t, trocou)
	assert.Equal(t, len(angulosDasNotas), countRows(t, db, "SELECT count(*) FROM note_embeddings"))
}

func TestUseEmbeddingModel_OutroModelo_ApagaVetoresELigacoesEReenfileira(t *testing.T) {
	db, _, graph := cofreComVetores(t)

	trocou, err := storage.UseEmbeddingModel(context.Background(), db, "outro", 3)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	assert.True(t, trocou)
	assert.Zero(t, countRows(t, db, "SELECT count(*) FROM note_embeddings"))
	assert.Equal(t, len(angulosDasNotas), countRows(t, db, "SELECT count(*) FROM note_embedding_queue"))
	assert.Empty(t, destinosPorTipo(t, graph, "n50")[notes.LinkSemantic])
}
