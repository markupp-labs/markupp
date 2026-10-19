package storage_test

import (
	"context"
	"path"
	"strings"
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

// resumo é o resumo esperado da nota de id e caminho, com o título do último
// segmento sem .md.
func resumo(id, caminho string) notes.NoteSummary {
	return notes.NoteSummary{ID: id, Title: strings.TrimSuffix(path.Base(caminho), ".md"), Path: caminho}
}

func resumoPtr(id, caminho string) *notes.NoteSummary {
	r := resumo(id, caminho)
	return &r
}

// todas é a primeira página grande o bastante para os cofres de teste.
var todas = notes.LinkFilter{Page: notes.Page{Limit: 100}}

func backlinks(ctx context.Context, t *testing.T, graph *storage.PostgresNoteGraph, noteID string) []string {
	t.Helper()
	lidas, err := graph.Backlinks(ctx, noteID, todas)
	require.NoError(t, err, "backlinks de %q", noteID)
	ids := make([]string, 0, len(lidas))
	for _, lida := range lidas {
		ids = append(ids, lida.Source.ID)
	}
	return ids
}

func ligacoesDoCofre(t *testing.T, graph *storage.PostgresNoteGraph, status string) []notes.Link {
	t.Helper()
	lidas, err := graph.VaultLinks(ctxDeTeste, cofreDeTeste, notes.VaultLinkQuery{Status: status, LinkFilter: todas})
	require.NoError(t, err, "ligacoes do cofre com status %q", status)
	return lidas
}

func TestNoteGraph_Backlinks_ResolvemPorTituloAliasECaminho(t *testing.T) {
	_, _, graph := cofreLigado(t)

	assert.Equal(t, []string{"a", "c", "d"}, backlinks(ctxDeTeste, t, graph, "b"), "titulo e alias sem caixa nem forma Unicode")
	assert.Equal(t, []string{"f", "b"}, backlinks(ctxDeTeste, t, graph, "c"), "link markdown com caminho existente liga so a ele; ordem por caminho")
	assert.Equal(t, []string{"f"}, backlinks(ctxDeTeste, t, graph, "c2"), "titulo ambiguo liga a todas as notas com ele")
	assert.Empty(t, backlinks(ctxDeTeste, t, graph, "g"), "ligacao para si mesma nao conta")
}

func TestNoteGraph_Backlinks_UmItemPorLigacaoComOTituloComoEscrito(t *testing.T) {
	_, _, graph := cofreLigado(t)

	lidas, err := graph.Backlinks(ctxDeTeste, "b", notes.LinkFilter{Page: notes.Page{Offset: 2, Limit: 1}})

	require.NoError(t, err)
	assert.Equal(t, []notes.Link{
		{Source: resumo("d", "d.md"), Target: resumoPtr("b", "pasta/b.md"), CitedTitle: "BÊ", Type: "explicit"},
	}, lidas, "o alias cita b; ligacao so do corpo nao tem predicado")
}

func TestNoteGraph_Links_SaidaResolvidaAmbiguaENaoResolvida(t *testing.T) {
	_, _, graph := cofreLigado(t)

	daA, err := graph.Links(ctxDeTeste, "a", todas)
	require.NoError(t, err)
	daF, err := graph.Links(ctxDeTeste, "f", todas)
	require.NoError(t, err)

	assert.Equal(t, []notes.Link{
		{Source: resumo("a", "a.md"), Target: resumoPtr("b", "pasta/b.md"), CitedTitle: "B", Type: "explicit"},
		{Source: resumo("a", "a.md"), CitedTitle: "foto.png", Type: "embed"},
		{Source: resumo("a", "a.md"), CitedTitle: "Inexistente", Type: "explicit"},
	}, daA, "nao resolvida tem target nulo")
	assert.Equal(t, []notes.Link{
		{Source: resumo("f", "f.md"), Target: resumoPtr("c", "c.md"), CitedTitle: "C", Type: "explicit"},
		{Source: resumo("f", "f.md"), Target: resumoPtr("c2", "outra/c.md"), CitedTitle: "C", Type: "explicit"},
	}, daF, "a ligacao ambigua aparece uma vez por nota com o titulo")
}

func TestNoteGraph_Links_FiltraPorTipo(t *testing.T) {
	_, _, graph := cofreLigado(t)

	embeds, err := graph.Links(ctxDeTeste, "a", notes.LinkFilter{Type: "embed", Page: todas.Page})

	require.NoError(t, err)
	require.Len(t, embeds, 1)
	assert.Equal(t, "foto.png", embeds[0].CitedTitle)
}

func TestNoteGraph_VaultLinks_PorStatusDeResolucao(t *testing.T) {
	_, _, graph := cofreLigado(t)

	soltas := ligacoesDoCofre(t, graph, notes.LinkUnresolved)
	resolvidas := ligacoesDoCofre(t, graph, notes.LinkResolved)
	todasDoCofre := ligacoesDoCofre(t, graph, "")

	assert.Equal(t, []notes.Link{
		{Source: resumo("a", "a.md"), CitedTitle: "foto.png", Type: "embed"},
		{Source: resumo("a", "a.md"), CitedTitle: "Inexistente", Type: "explicit"},
	}, soltas)
	assert.Len(t, resolvidas, 6, "a, c, d e o link markdown de b ligam uma vez; f liga a c e a c2")
	assert.Len(t, todasDoCofre, 8)
}

func TestNoteGraph_NotaRenomeada_ResolveNaLeituraSemReindexarQuemCita(t *testing.T) {
	db, repo, graph := cofreLigado(t)
	_, err := repo.Update(ctxDeTeste, "b", "pasta/beta.md", "---\naliases: [Bê]\n---\n[c](../c.md)", time.Now(), time.Time{}, true)
	require.NoError(t, err)
	indexaTodosOsCofres(t, db, "simple")

	assert.Equal(t, []string{"d"}, backlinks(ctxDeTeste, t, graph, "b"), "so o alias continua citando a nota renomeada")
	soltas := ligacoesDoCofre(t, graph, notes.LinkUnresolved)
	assert.Contains(t, soltas, notes.Link{Source: resumo("a", "a.md"), CitedTitle: "B", Type: "explicit"})
	assert.Contains(t, soltas, notes.Link{Source: resumo("c", "c.md"), CitedTitle: "b", Type: "explicit"})
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

	_, err := graph.Backlinks(ctxFora, "b", todas)
	require.ErrorIs(t, err, notes.ErrNotFound)
	_, err = graph.Links(ctxDeTeste, "inexistente", todas)
	require.ErrorIs(t, err, notes.ErrNotFound)
}

func TestNoteGraph_CofreInvisivel_ErrVaultForbidden(t *testing.T) {
	db, _, graph := cofreLigado(t)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	_, err := graph.VaultLinks(ctxFora, cofreDeTeste, notes.VaultLinkQuery{LinkFilter: todas})
	require.ErrorIs(t, err, notes.ErrVaultForbidden)
}

func TestNoteGraph_ViewerLe(t *testing.T) {
	db, _, graph := cofreLigado(t)
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)

	assert.Equal(t, []string{"a", "c", "d"}, backlinks(ctxViewer, t, graph, "b"))
	lidas, err := graph.Links(ctxViewer, "a", todas)
	require.NoError(t, err)
	assert.Len(t, lidas, 3)
}

func TestMigracao00029_DownEUp(t *testing.T) {
	pool := storagetest.EmptyPool(t)
	provider := gooseProvider(t, pool)
	_, err := provider.Up(context.Background())
	require.NoError(t, err)

	_, err = provider.DownTo(context.Background(), 28)
	require.NoError(t, err)
	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}

func TestNoteGraph_WikilinkComPasta_ResolvePeloCaminhoECaiParaOTitulo(t *testing.T) {
	db := setupNotesDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	for _, nota := range []notes.Note{
		notaComCaminho("x1", "a/Nota.md", "x"),
		notaComCaminho("x2", "b/Nota.md", "x"),
		notaComCaminho("o1", "o1.md", "[[b/Nota]]"),
		notaComCaminho("o2", "o2.md", "[[c/Nota]]"),
	} {
		storagetest.SaveNote(ctxDeTeste, t, repo, nota)
	}
	indexaTodosOsCofres(t, db, "simple")
	graph := storage.NewPostgresNoteGraph(db, "simple")

	assert.Equal(t, []string{"o2"}, backlinks(ctxDeTeste, t, graph, "x1"), "caminho inexistente cai para o titulo")
	assert.Equal(t, []string{"o1", "o2"}, backlinks(ctxDeTeste, t, graph, "x2"), "caminho existente liga so a ele")
}
