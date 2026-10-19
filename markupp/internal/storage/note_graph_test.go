package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// cofreLigado monta um cofre indexado com ligações de todos os casos: wikilink,
// alias com outra caixa, link markdown por caminho, título ambíguo, ligação para si
// mesma, embed de anexo e título inexistente.
func cofreLigado(t *testing.T) (*pgxpool.Pool, *storage.PostgresNotesRepository, *storage.PostgresNoteGraph) {
	t.Helper()
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("a", "a.md", "[[B]] [[Inexistente]] ![[foto.png]]"),
		notaComCaminho("b", "pasta/b.md", "---\naliases: [Bê]\n---\n[c](../c.md)"),
		notaComCaminho("c", "c.md", "[[b]]"),
		notaComCaminho("c2", "outra/c.md", "x"),
		notaComCaminho("d", "d.md", "[[BÊ]]"),
		notaComCaminho("e", "e.md", "solta"),
		notaComCaminho("f", "f.md", "[[C]]"),
		notaComCaminho("g", "g.md", "[[g]]"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	return db, repo, storage.NewPostgresNoteGraph(db, "simple")
}

func backlinks(ctx context.Context, t *testing.T, graph *storage.PostgresNoteGraph, noteID string) []string {
	t.Helper()
	lidas, err := graph.Backlinks(ctx, noteID, 0, 100)
	require.NoError(t, err, "backlinks de %q", noteID)
	ids := make([]string, 0, len(lidas))
	for _, lida := range lidas {
		ids = append(ids, lida.ID)
	}
	return ids
}

func TestNoteGraph_Backlinks_ResolvemPorTituloAliasECaminho(t *testing.T) {
	_, _, graph := cofreLigado(t)

	assert.Equal(t, []string{"a", "c", "d"}, backlinks(ctxDeTeste, t, graph, "b"), "titulo e alias sem caixa nem forma Unicode")
	assert.Equal(t, []string{"f", "b"}, backlinks(ctxDeTeste, t, graph, "c"), "link markdown com caminho existente liga so a ele; ordem por caminho")
	assert.Equal(t, []string{"f"}, backlinks(ctxDeTeste, t, graph, "c2"), "titulo ambiguo liga a todas as notas com ele")
	assert.Empty(t, backlinks(ctxDeTeste, t, graph, "g"), "ligacao para si mesma nao conta")
}

func TestNoteGraph_Backlinks_Paginados(t *testing.T) {
	_, _, graph := cofreLigado(t)

	lidas, err := graph.Backlinks(ctxDeTeste, "b", 1, 1)

	require.NoError(t, err)
	require.Len(t, lidas, 1)
	assert.Equal(t, notes.Backlink{
		LinkedNote: notes.LinkedNote{ID: "c", VaultID: cofreDeTeste, Path: "c.md", UpdatedAt: lidas[0].UpdatedAt},
		Predicates: []string{},
	}, lidas[0], "ligacao so do corpo nao tem predicado")
}

func TestNoteGraph_Neighbors_EntradaESaida(t *testing.T) {
	_, _, graph := cofreLigado(t)

	vizinhos, err := graph.Neighbors(ctxDeTeste, "b", 0, 100)

	require.NoError(t, err)
	require.Len(t, vizinhos, 3)
	assert.Equal(t, []notes.Neighbor{
		{LinkedNote: notes.LinkedNote{ID: "a", VaultID: cofreDeTeste, Path: "a.md", UpdatedAt: vizinhos[0].UpdatedAt}, Incoming: true, IncomingPredicates: []string{}, OutgoingPredicates: []string{}},
		{LinkedNote: notes.LinkedNote{ID: "c", VaultID: cofreDeTeste, Path: "c.md", UpdatedAt: vizinhos[1].UpdatedAt}, Incoming: true, Outgoing: true, IncomingPredicates: []string{}, OutgoingPredicates: []string{}},
		{LinkedNote: notes.LinkedNote{ID: "d", VaultID: cofreDeTeste, Path: "d.md", UpdatedAt: vizinhos[2].UpdatedAt}, Incoming: true, IncomingPredicates: []string{}, OutgoingPredicates: []string{}},
	}, vizinhos)
}

func TestNoteGraph_UnresolvedLinks_TitulosSemNota(t *testing.T) {
	_, _, graph := cofreLigado(t)

	soltas, err := graph.UnresolvedLinks(ctxDeTeste, cofreDeTeste, 0, 100)

	require.NoError(t, err)
	assert.Equal(t, []notes.UnresolvedLink{
		{NoteID: "a", Path: "a.md", Title: "foto.png", Kind: "embed"},
		{NoteID: "a", Path: "a.md", Title: "Inexistente", Kind: "explicit"},
	}, soltas)
}

func TestNoteGraph_NotaRenomeada_ResolveNaLeituraSemReindexarQuemCita(t *testing.T) {
	db, repo, graph := cofreLigado(t)
	_, err := repo.Update(ctxDeTeste, "b", "pasta/beta.md", "---\naliases: [Bê]\n---\n[c](../c.md)", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, []string{"d"}, backlinks(ctxDeTeste, t, graph, "b"), "so o alias continua citando a nota renomeada")
	soltas, err := graph.UnresolvedLinks(ctxDeTeste, cofreDeTeste, 0, 100)
	require.NoError(t, err)
	assert.Contains(t, soltas, notes.UnresolvedLink{NoteID: "a", Path: "a.md", Title: "B", Kind: "explicit"})
	assert.Contains(t, soltas, notes.UnresolvedLink{NoteID: "c", Path: "c.md", Title: "b", Kind: "explicit"})
}

func TestNoteGraph_CaminhoDoLinkMarkdownApagado_CaiParaOTitulo(t *testing.T) {
	_, repo, graph := cofreLigado(t)

	require.NoError(t, repo.Delete(ctxDeTeste, "c"))

	assert.Equal(t, []string{"f", "b"}, backlinks(ctxDeTeste, t, graph, "c2"), "sem nota no caminho, o link vale pelo titulo")
}

func TestNoteGraph_NotaInvisivel_ErrNotFound(t *testing.T) {
	db, _, graph := cofreLigado(t)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	_, err := graph.Backlinks(ctxFora, "b", 0, 10)
	require.ErrorIs(t, err, notes.ErrNotFound)
	_, err = graph.Neighbors(ctxDeTeste, "inexistente", 0, 10)
	require.ErrorIs(t, err, notes.ErrNotFound)
}

func TestNoteGraph_CofreInvisivel_ErrVaultForbidden(t *testing.T) {
	db, _, graph := cofreLigado(t)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	_, err := graph.UnresolvedLinks(ctxFora, cofreDeTeste, 0, 10)
	require.ErrorIs(t, err, notes.ErrVaultForbidden)
}

func TestNoteGraph_ViewerLe(t *testing.T) {
	db, _, graph := cofreLigado(t)
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)

	assert.Equal(t, []string{"a", "c", "d"}, backlinks(ctxViewer, t, graph, "b"))
}
