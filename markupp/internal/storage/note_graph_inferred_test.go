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

// cofreComErroDeDigitacao monta um cofre indexado em que a nota a cita
// "Kubernets", que não existe, e s cita "postgresqk", mais perto do título de p, que
// m menciona sem ligar, do que do título de x, que hub liga e que divide hub com s.
func cofreComErroDeDigitacao(t *testing.T) (*pgxpool.Pool, *storage.PostgresNotesRepository, *storage.PostgresNoteGraph) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("k", "Kubernetes.md", "orquestrador"),
		notaComCaminho("a", "a.md", "[[Kubernets]]"),
		notaComCaminho("p", "postgresql.md", "banco"),
		notaComCaminho("x", "postgresqlx.md", "driver"),
		notaComCaminho("m", "m.md", "postgresql sem ligar"),
		notaComCaminho("hub", "hub.md", "[[postgresqlx]]"),
		notaComCaminho("s", "s.md", "[[postgresqk]] [[hub]]"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return db, repo, storage.NewPostgresNoteGraph(db, "simple")
}

func TestLigacaoInferida_ApareceComOTipoInferredNasLigacoesDasDuasNotas(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	entrada, err := graph.Backlinks(ctxDeTeste, "k", todas)
	require.NoError(t, err)
	saida, err := graph.Links(ctxDeTeste, "a", todas)
	require.NoError(t, err)

	k := resumoPtr("k", "Kubernetes.md")
	inferida := notes.Link{Source: resumo("a", "a.md"), Target: k, CitedTitle: "Kubernets", Type: "inferred"}
	assert.Equal(t, []notes.Link{inferida}, entrada)
	assert.Equal(t, []notes.Link{
		inferida,
		{Source: resumo("a", "a.md"), CitedTitle: "Kubernets", Type: "explicit", InferredTarget: k},
	}, saida, "a ligacao escrita continua nao resolvida, com o palpite em inferred_target")
}

func TestLigacaoInferida_ExplicitaNaoEMarcada(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	lidas, err := graph.Backlinks(ctxDeTeste, "hub", todas)

	require.NoError(t, err)
	require.Len(t, lidas, 1)
	assert.Equal(t, "explicit", lidas[0].Type)
}

func TestLigacaoInferida_FiltroPorTipo(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	lidas, err := graph.VaultLinks(ctxDeTeste, cofreDeTeste, notes.VaultLinkQuery{LinkFilter: notes.LinkFilter{Type: "inferred", Page: todas.Page}})

	require.NoError(t, err)
	assert.Len(t, lidas, 2, "a cita Kubernets e s cita postgresqk")
}

func TestLigacaoInferida_ParentescoDecideEntreDistanciaEProbabilidade(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	assert.Equal(t, []string{"hub", "s"}, backlinks(ctxDeTeste, t, graph, "x"), "p esta mais perto; x e mais ligada e divide o hub com s")
	assert.Empty(t, backlinks(ctxDeTeste, t, graph, "p"))
}

func TestLigacaoInferida_NaoViraNaoResolvidaEMarcaOPalpite(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	soltas := ligacoesDoCofre(t, graph, notes.LinkUnresolved)

	assert.Equal(t, []notes.Link{
		{Source: resumo("a", "a.md"), CitedTitle: "Kubernets", Type: "explicit", InferredTarget: resumoPtr("k", "Kubernetes.md")},
		{Source: resumo("s", "s.md"), CitedTitle: "postgresqk", Type: "explicit", InferredTarget: resumoPtr("x", "postgresqlx.md")},
	}, soltas, "a ligacao escrita continua nao resolvida; a inferida nao entra")
}

func TestLigacaoInferida_SomeQuandoOTituloCitadoGanhaNota(t *testing.T) {
	db, repo, graph := cofreComErroDeDigitacao(t)
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("k2", "Kubernets.md", "nova"))

	indexaTodosOsCofres(t, db, "simple")

	assert.Empty(t, backlinks(ctxDeTeste, t, graph, "k"), "o segundo estagio reavalia so as nao resolvidas")
	assert.Equal(t, []string{"a"}, backlinks(ctxDeTeste, t, graph, "k2"))
}

func TestLigacaoInferida_SobreviveAoLoteQueNaoMudaLigacoes(t *testing.T) {
	db, repo, graph := cofreComErroDeDigitacao(t)
	_, err := repo.Update(ctxDeTeste, "a", "a.md", "[[Kubernets]] mais texto", time.Now(), time.Time{}, true)
	require.NoError(t, err)

	indexaTodosOsCofres(t, db, "simple")

	assert.NotContains(t, frequenciasDoCofre(t, db, cofreDeTeste), "texto", "o lote nao mudou titulo nem ligacao escrita")
	assert.Equal(t, []string{"a"}, backlinks(ctxDeTeste, t, graph, "k"))
}

// TestLigacaoInferida_TituloComumNaoEntraNoPrior: s cita "postgresqk", mais perto
// do título de p; x é parente de s pelo hub e, citada por sete notas de 111, teria o
// maior prior, mas o título dela passa de 5% do cofre. O primeiro segundo estágio do
// cofre já grava o df antes de inferir.
func TestLigacaoInferida_TituloComumNaoEntraNoPriorJaNoPrimeiroJob(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("p", "postgresql.md", "banco"),
		notaComCaminho("x", "postgresqlx.md", "driver"),
		notaComCaminho("m", "m.md", "postgresql sem ligar"),
		notaComCaminho("hub", "hub.md", "[[postgresqlx]]"),
		notaComCaminho("s", "s.md", "[[postgresqk]] [[hub]]"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	semeiaNotas(t, db, "c", "[[postgresqlx]]", 6)
	semeiaNotas(t, db, "e", "enchimento", enchimento)
	indexaTodosOsCofres(t, db, "simple")
	graph := storage.NewPostgresNoteGraph(db, "simple")

	assert.Equal(t, []string{"s"}, backlinks(ctxDeTeste, t, graph, "p"), "sem o prior de x, distancia e prior ficam com p")
	assert.NotContains(t, backlinks(ctxDeTeste, t, graph, "x"), "s")
}
