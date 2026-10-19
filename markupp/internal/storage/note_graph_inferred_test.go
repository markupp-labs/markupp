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

func TestLigacaoInferida_ApareceMarcadaEmBacklinksEVizinhanca(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	lidas, err := graph.Backlinks(ctxDeTeste, "k", 0, 10)
	require.NoError(t, err)
	require.Len(t, lidas, 1)
	assert.Equal(t, notes.Backlink{
		LinkedNote: notes.LinkedNote{ID: "a", VaultID: cofreDeTeste, Path: "a.md", UpdatedAt: lidas[0].UpdatedAt},
		Predicates: []string{}, Inferred: true,
	}, lidas[0])

	vizinhos, err := graph.Neighbors(ctxDeTeste, "a", 0, 10)
	require.NoError(t, err)
	require.Len(t, vizinhos, 1)
	assert.True(t, vizinhos[0].Outgoing)
	assert.True(t, vizinhos[0].Inferred, "a so se liga a k por inferencia")
}

func TestLigacaoInferida_ExplicitaNaoEMarcada(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	lidas, err := graph.Backlinks(ctxDeTeste, "hub", 0, 10)

	require.NoError(t, err)
	require.Len(t, lidas, 1)
	assert.False(t, lidas[0].Inferred)
}

func TestLigacaoInferida_ParentescoDecideEntreDistanciaEProbabilidade(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	assert.Equal(t, []string{"hub", "s"}, backlinks(ctxDeTeste, t, graph, "x"), "p esta mais perto; x e mais ligada e divide o hub com s")
	assert.Empty(t, backlinks(ctxDeTeste, t, graph, "p"))
}

func TestLigacaoInferida_NaoViraNaoResolvida(t *testing.T) {
	_, _, graph := cofreComErroDeDigitacao(t)

	soltas, err := graph.UnresolvedLinks(ctxDeTeste, cofreDeTeste, 0, 100)
	require.NoError(t, err)
	assert.Equal(t, []notes.UnresolvedLink{
		{NoteID: "a", Path: "a.md", Title: "Kubernets", Kind: "explicit"},
		{NoteID: "s", Path: "s.md", Title: "postgresqk", Kind: "explicit"},
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
