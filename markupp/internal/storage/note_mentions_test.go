package storage_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// cofreComMencoes monta um cofre indexado com menções não ligadas e os casos que
// não contam: ligação em qualquer sentido, parte de palavra, texto de ligação,
// título curto, título stopword e a própria nota. Os títulos ambíguos "plano"
// diferem só pelas ligações que recebem.
func cofreComMencoes(t *testing.T) (*pgxpool.Pool, *storage.PostgresNoteGraph) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("k", "Kubernetes.md", "o kubernetes orquestra [[volta]]"),
		notaComCaminho("dep", "deploy.md", "subimos o Kubernetes ontem"),
		notaComCaminho("lig", "ligada.md", "[[Kubernetes]] e o kubernetes de novo"),
		notaComCaminho("volta", "volta.md", "kubernetes aqui"),
		notaComCaminho("par", "parcial.md", "kubernetesx e [[Kubernetes Deploy]]"),
		notaComCaminho("idx", "Indexação.md", "x"),
		notaComCaminho("sem", "sem-acento.md", "sobre a indexacao"),
		notaComCaminho("go", "Go.md", "x"),
		notaComCaminho("para", "para.md", "x"),
		notaComCaminho("usa", "usa-go.md", "uso go e para"),
		notaComCaminho("pa", "a/plano.md", "x"),
		notaComCaminho("pb", "b/plano.md", "x"),
		notaComCaminho("l1", "l1.md", "[p](a/plano.md)"),
		notaComCaminho("m", "m.md", "o plano novo"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return db, storage.NewPostgresNoteGraph(db, "simple")
}

func mencoes(t *testing.T, graph *storage.PostgresNoteGraph, noteID string, offset, limit int) []notes.UnlinkedMention {
	t.Helper()
	lidas, err := graph.Mentions(ctxDeTeste, noteID, offset, limit)
	require.NoError(t, err, "mencoes de %q", noteID)
	return lidas
}

func resumoDasMencoes(lidas []notes.UnlinkedMention) []string {
	resumo := make([]string, 0, len(lidas))
	for _, lida := range lidas {
		resumo = append(resumo, lida.Direction+" "+lida.Path+" "+lida.Title)
	}
	return resumo
}

func TestNoteGraph_Mentions_QuemMencionaANota(t *testing.T) {
	_, graph := cofreComMencoes(t)

	lidas := mencoes(t, graph, "k", 0, 100)

	assert.Equal(t, []string{"incoming deploy.md kubernetes"}, resumoDasMencoes(lidas),
		"sem quem liga em algum sentido, sem parte de palavra, sem texto de ligacao e sem a propria nota")
	require.Len(t, lidas, 1)
	assert.Equal(t, notes.LinkedNote{ID: "dep", VaultID: cofreDeTeste, Path: "deploy.md", UpdatedAt: lidas[0].UpdatedAt}, lidas[0].LinkedNote)
	assert.InDelta(t, 2.0/6.0, lidas[0].Prior, 1e-9, "uma ligacao com o titulo, cinco notas com o termo e uma candidata")
}

func TestNoteGraph_Mentions_TitulosQueANotaMenciona(t *testing.T) {
	_, graph := cofreComMencoes(t)

	assert.Equal(t, []string{"outgoing Kubernetes.md kubernetes"}, resumoDasMencoes(mencoes(t, graph, "dep", 0, 100)))
}

func TestNoteGraph_Mentions_SemAcento(t *testing.T) {
	_, graph := cofreComMencoes(t)

	assert.Equal(t, []string{"incoming sem-acento.md indexação"}, resumoDasMencoes(mencoes(t, graph, "idx", 0, 100)))
	assert.Equal(t, []string{"outgoing Indexação.md indexação"}, resumoDasMencoes(mencoes(t, graph, "sem", 0, 100)))
}

func TestNoteGraph_Mentions_IgnoraTituloCurtoEStopword(t *testing.T) {
	_, graph := cofreComMencoes(t)

	assert.Empty(t, mencoes(t, graph, "usa", 0, 100))
	assert.Empty(t, mencoes(t, graph, "go", 0, 100))
	assert.Empty(t, mencoes(t, graph, "para", 0, 100))
}

func TestNoteGraph_Mentions_OrdenaPelaProbabilidadeDaAncora(t *testing.T) {
	_, graph := cofreComMencoes(t)

	lidas := mencoes(t, graph, "m", 0, 100)

	assert.Equal(t, []string{"outgoing a/plano.md plano", "outgoing b/plano.md plano"}, resumoDasMencoes(lidas),
		"o plano que ja recebe ligacao com esse titulo vem antes")
	require.Len(t, lidas, 2)
	assert.Greater(t, lidas[0].Prior, lidas[1].Prior)
	assert.Equal(t, []string{"outgoing b/plano.md plano"}, resumoDasMencoes(mencoes(t, graph, "m", 1, 1)), "paginado")
}

func TestNoteGraph_Mentions_ViewerLeEEstranhoNaoVe(t *testing.T) {
	db, graph := cofreComMencoes(t)
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	lidas, err := graph.Mentions(ctxViewer, "dep", 0, 10)
	require.NoError(t, err)
	assert.Len(t, lidas, 1)
	_, err = graph.Mentions(ctxFora, "dep", 0, 10)
	require.ErrorIs(t, err, notes.ErrNotFound)
}
