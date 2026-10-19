package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// cofreComSugestoes monta um cofre indexado em que x e y ligam a z, lex fala do mesmo
// assunto que x sem ligação, l já liga a x e longe não tem nada em comum com x.
func cofreComSugestoes(t *testing.T) *storage.PostgresNoteGraph {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("x", "x.md", "[[z]] kubernetes orquestra conteineres"),
		notaComCaminho("y", "y.md", "[[z]]"),
		notaComCaminho("z", "z.md", "alvo"),
		notaComCaminho("l", "l.md", "[[x]] kubernetes orquestra conteineres"),
		notaComCaminho("lex", "lex.md", "kubernetes orquestra conteineres"),
		notaComCaminho("longe", "longe.md", "nada a ver"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return storage.NewPostgresNoteGraph(db, "simple")
}

func sugestoes(t *testing.T, graph *storage.PostgresNoteGraph, noteID string, offset, limit int) []notes.LinkSuggestion {
	t.Helper()
	lidas, err := graph.Suggestions(ctxDeTeste, noteID, offset, limit)
	require.NoError(t, err, "sugestoes de %q", noteID)
	return lidas
}

func caminhosDasSugestoes(lidas []notes.LinkSuggestion) []string {
	caminhos := make([]string, 0, len(lidas))
	for _, lida := range lidas {
		caminhos = append(caminhos, lida.Path)
	}
	return caminhos
}

func TestNoteGraph_Suggestions_FundeEstruturaETextoSemAPropriaNemAsLigadas(t *testing.T) {
	graph := cofreComSugestoes(t)

	lidas := sugestoes(t, graph, "x", 0, 100)

	assert.Equal(t, []string{"y.md", "lex.md"}, caminhosDasSugestoes(lidas),
		"y vem nas duas listas; z e l ja se ligam a x")
	require.Len(t, lidas, 2)
	assert.InDelta(t, 0.5, lidas[0].ResourceAllocation, 1e-9, "z tem dois vizinhos, x e y")
	assert.InDelta(t, 1.0/61+1.0/62, lidas[0].Score, 1e-9, "primeira na estrutural e segunda na lexica")
	assert.Zero(t, lidas[1].ResourceAllocation)
	assert.Greater(t, lidas[1].Cosine, lidas[0].Cosine, "lex divide mais termos com x do que y")
	assert.Equal(t, notes.LinkedNote{ID: "lex", VaultID: cofreDeTeste, Path: "lex.md", UpdatedAt: lidas[1].UpdatedAt}, lidas[1].LinkedNote)
}

func TestNoteGraph_Suggestions_Paginado(t *testing.T) {
	graph := cofreComSugestoes(t)

	assert.Equal(t, []string{"lex.md"}, caminhosDasSugestoes(sugestoes(t, graph, "x", 1, 1)))
}

func TestNoteGraph_Suggestions_NotaSemLigacaoNemTermoEmComum_Vazio(t *testing.T) {
	graph := cofreComSugestoes(t)

	assert.Empty(t, sugestoes(t, graph, "longe", 0, 100))
}

func TestNoteGraph_Suggestions_ViewerLeEEstranhoNaoVe(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("a", "a.md", "kubernetes orquestra"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("b", "b.md", "kubernetes orquestra"))
	storagetest.SaveNote(ctxDeTeste, t, repo, notaComCaminho("c", "c.md", "outro assunto"))
	indexaTodosOsCofres(t, db, "simple")
	graph := storage.NewPostgresNoteGraph(db, "simple")
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	lidas, err := graph.Suggestions(ctxViewer, "a", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"b.md"}, caminhosDasSugestoes(lidas))
	_, err = graph.Suggestions(ctxFora, "a", 0, 10)
	require.ErrorIs(t, err, notes.ErrNotFound)
}
