package storage_test

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// cofreComLigacaoPorMencao monta um cofre indexado em que pipeline e recusada citam
// Pulumi sem ligar, escrita liga e cita, pulumi liga a volta, que cita Pulumi de volta,
// e solta não cita nada.
func cofreComLigacaoPorMencao(t *testing.T) (*pgxpool.Pool, *storage.PostgresNotesRepository, *storage.PostgresNoteGraph) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("pulumi", "Pulumi.md", "infra como codigo [[Volta]]"),
		notaComCaminho("pipeline", "Pipeline.md", "o pipeline sobe com Pulumi"),
		notaComCaminho("escrita", "Escrita.md", "[[Pulumi]] e de novo pulumi"),
		notaComCaminho("volta", "Volta.md", "cita pulumi de volta"),
		notaComCaminho("recusada", "Recusada.md", "cita Pulumi"),
		notaComCaminho("solta", "Solta.md", "nada"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return db, repo, storage.NewPostgresNoteGraph(db, "simple")
}

func mencaoDePara(origem, destino notes.NoteSummary, titulo string) notes.Link {
	return notes.Link{Source: origem, Target: &destino, CitedTitle: titulo, Type: notes.LinkMentioned}
}

func TestLigacaoPorMencao_VaiDaNotaQueCitaParaACitada(t *testing.T) {
	_, _, graph := cofreComLigacaoPorMencao(t)

	saida, err := graph.Links(ctxDeTeste, "pipeline", todas)

	require.NoError(t, err)
	assert.Equal(t, []notes.Link{mencaoDePara(resumo("pipeline", "Pipeline.md"), resumo("pulumi", "Pulumi.md"), "pulumi")}, saida)
}

func TestLigacaoPorMencao_NaoRepeteLigacaoEscritaEmNenhumSentido(t *testing.T) {
	_, _, graph := cofreComLigacaoPorMencao(t)

	assert.Equal(t, []string{"escrita", "pipeline", "recusada"}, backlinks(ctxDeTeste, t, graph, "pulumi"),
		"escrita ja liga por escrito e pulumi liga a volta")
	saida, err := graph.Links(ctxDeTeste, "escrita", todas)
	require.NoError(t, err)
	assert.Equal(t, []string{notes.LinkExplicit}, tiposDasLigacoes(saida))
}

func TestLigacaoPorMencao_TiraANotaDasOrfasEDaMencaoNaoLigada(t *testing.T) {
	_, repo, graph := cofreComLigacaoPorMencao(t)

	orfas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Orphan: ptr(true), Limit: 100})
	mencoes, err := graph.Mentions(ctxDeTeste, "pipeline", notes.Page{Limit: 100})

	require.NoError(t, err)
	assert.Equal(t, []string{"solta"}, idsDasNotas(orfas), "pipeline so tem a ligacao por mencao")
	assert.Empty(t, mencoes, "o par ja tem ligacao")
}

func TestLigacaoPorMencao_LoteCriaQuandoSoOTextoMuda(t *testing.T) {
	db, repo, graph := cofreComLigacaoPorMencao(t)
	antes := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	_, err := repo.Update(ctxDeTeste, "solta", "Solta.md", "agora cita Pulumi", antes.Add(time.Hour), antes, false)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	saida, err := graph.Links(ctxDeTeste, "solta", notes.LinkFilter{Type: notes.LinkMentioned, Page: todas.Page})
	require.NoError(t, err)
	assert.Equal(t, []notes.Link{mencaoDePara(resumo("solta", "Solta.md"), resumo("pulumi", "Pulumi.md"), "pulumi")}, saida)
	orfas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Orphan: ptr(true), Limit: 100})
	assert.Empty(t, orfas, "o lote marca o cofre e o segundo estagio refaz o grau")
}

func TestLigacaoPorMencao_SomeQuandoOTextoDeixaDeCitar(t *testing.T) {
	db, repo, graph := cofreComLigacaoPorMencao(t)
	antes := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	_, err := repo.Update(ctxDeTeste, "pipeline", "Pipeline.md", "o pipeline sobe sozinho", antes.Add(time.Hour), antes, false)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	saida, err := graph.Links(ctxDeTeste, "pipeline", todas)
	require.NoError(t, err)
	assert.Empty(t, saida)
}

func tiposDasLigacoes(lidas []notes.Link) []string {
	tipos := make([]string, 0, len(lidas))
	for _, l := range lidas {
		tipos = append(tipos, l.Type)
	}
	return tipos
}
