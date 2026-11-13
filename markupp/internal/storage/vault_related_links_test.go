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

// cofreComTextoParecido monta um cofre indexado em que orquestracao e containers falam
// quase do mesmo jeito, doce liga a receita por escrito e as duas também se parecem, e
// viagem divide com elas só a palavra bolo.
func cofreComTextoParecido(t *testing.T) (*pgxpool.Pool, *storage.PostgresNotesRepository, *storage.PostgresNoteGraph) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("orquestracao", "Orquestracao.md", "kubernetes cluster pod deploy helm"),
		notaComCaminho("containers", "Containers.md", "kubernetes cluster pod deploy helm chart"),
		notaComCaminho("receita", "Receita.md", "bolo farinha ovos acucar forno manteiga leite"),
		notaComCaminho("doce", "Doce.md", "[[Receita]] bolo farinha ovos acucar chocolate manteiga leite"),
		notaComCaminho("viagem", "Viagem.md", "viagem praia sol bolo areia mar onda vento"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return db, repo, storage.NewPostgresNoteGraph(db, "simple")
}

func textoParecidoDePara(origem, destino notes.NoteSummary) notes.Link {
	return notes.Link{Source: origem, Target: &destino, Type: notes.LinkRelated}
}

func TestLigacaoPorTextoParecido_LigaOParNosDoisSentidos(t *testing.T) {
	_, repo, graph := cofreComTextoParecido(t)
	orquestracao, containers := resumo("orquestracao", "Orquestracao.md"), resumo("containers", "Containers.md")

	daOrquestracao, err := graph.Links(ctxDeTeste, "orquestracao", notes.LinkFilter{Type: notes.LinkRelated, Page: todas.Page})
	require.NoError(t, err)
	dosContainers, err := graph.Links(ctxDeTeste, "containers", todas)
	require.NoError(t, err)

	assert.Equal(t, []notes.Link{textoParecidoDePara(orquestracao, containers)}, daOrquestracao)
	assert.Equal(t, []notes.Link{textoParecidoDePara(containers, orquestracao)}, dosContainers)
	orfas := notasDoCofre(ctxDeTeste, t, repo, notes.VaultNotesQuery{Orphan: ptr(true), Limit: 100})
	assert.Equal(t, []string{"viagem"}, idsDasNotas(orfas), "a ligacao por texto parecido tira o par das orfas")
}

func TestLigacaoPorTextoParecido_NaoLigaAbaixoDoCossenoMinimoNemRepeteEscrita(t *testing.T) {
	_, _, graph := cofreComTextoParecido(t)

	daViagem, err := graph.Links(ctxDeTeste, "viagem", todas)
	require.NoError(t, err)
	doDoce, err := graph.Links(ctxDeTeste, "doce", todas)
	require.NoError(t, err)

	assert.Empty(t, daViagem, "viagem divide so bolo com receita e doce")
	assert.Equal(t, []string{notes.LinkExplicit}, tiposDasLigacoes(doDoce), "doce ja liga a receita por escrito")
}

func TestLigacaoPorTextoParecido_LoteTiraQuandoOTextoDeixaDeParecer(t *testing.T) {
	db, repo, graph := cofreComTextoParecido(t)
	antes := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	_, err := repo.Update(ctxDeTeste, "containers", "Containers.md", "lista de compras do mercado", antes.Add(time.Hour), antes, false)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	saida, err := graph.Links(ctxDeTeste, "orquestracao", todas)
	require.NoError(t, err)
	assert.Empty(t, saida)
}
