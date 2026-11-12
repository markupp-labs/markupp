package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// pastaPublicadaComVizinhas semeia em "va" a pasta docs, com o guia que cita o passo
// público e a nota privada, publica a pasta, indexa e devolve a leitura pública.
func pastaPublicadaComVizinhas(t *testing.T) (*pgxpool.Pool, *storage.PostgresPublicNotes) {
	t.Helper()
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "owner"}, "va")
	for _, n := range []notes.Note{
		notaPublica("guia", "docs/Guia de deploy.md", "va", "leia o [[Passo]] e o [[Segredo]] antes do deploy, e veja ![[Passo]]"),
		notaPublica("passo", "docs/x/Passo.md", "va", "o passo cita o [[Guia de deploy]] e o deploy"),
		notaPublica("segredo", "privado/Segredo.md", "va", "deploy secreto"),
	} {
		storagetest.SaveNote(ctx, t, repo, n)
	}
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))
	indexaTodosOsCofres(t, db, "simple")
	return db, storage.NewPostgresPublicNotes(db, "simple")
}

func idPublicoDe(t *testing.T, db *pgxpool.Pool, noteID string) string {
	t.Helper()
	publicID, _ := coberturaDe(t, db, noteID)
	require.NotEmpty(t, publicID, "nota %s sem id publico", noteID)
	return publicID
}

func TestNotaPublica_NaPasta_TrazConteudoPublicacaoELigacoesSemOAlvoPrivado(t *testing.T) {
	db, leitura := pastaPublicadaComVizinhas(t)
	passo := idPublicoDe(t, db, "passo")

	n, err := leitura.PublicNote(context.Background(), idPublicoDe(t, db, "guia"))

	require.NoError(t, err)
	assert.Equal(t, "Guia de deploy", n.Title)
	assert.Contains(t, n.Content, "[[Passo]]")
	assert.False(t, n.Indexable)
	assert.Equal(t, publication.PublicScope{Kind: "folder", Title: "docs"}, n.Publication)
	tituloDoPasso := "Passo"
	assert.Equal(t, []publication.PublicLink{
		{CitedTitle: "Passo", Kind: "embed", PublicID: &passo, TargetTitle: &tituloDoPasso},
		{CitedTitle: "Passo", Kind: "explicit", PublicID: &passo, TargetTitle: &tituloDoPasso},
		{CitedTitle: "Segredo", Kind: "explicit"},
	}, n.Links)
}

func TestNotaPublica_Avulsa_TituloDaNotaERaizNelaMesma(t *testing.T) {
	db, leitura := pastaPublicadaComVizinhas(t)
	publica(t, db, "p-segredo", "t1", "va", alvoNota("segredo"))
	segredo := idPublicoDe(t, db, "segredo")

	n, err := leitura.PublicNote(context.Background(), segredo)

	require.NoError(t, err)
	assert.Equal(t, publication.PublicScope{Kind: "note", Title: "Segredo", RootPublicID: &segredo}, n.Publication)
}

func TestNotaPublica_CofreIndexavel_TituloDoCofre(t *testing.T) {
	db, leitura := pastaPublicadaComVizinhas(t)
	_, err := db.Exec(context.Background(), `INSERT INTO publications (id, tenant_id, vault_id, kind, indexable, published_by, published_at)
		VALUES ('p-cofre', 't1', 'va', 'vault', true, 'ua', now())`)
	require.NoError(t, err)

	n, err := leitura.PublicNote(context.Background(), idPublicoDe(t, db, "guia"))

	require.NoError(t, err)
	assert.True(t, n.Indexable)
	assert.Equal(t, publication.PublicScope{Kind: "vault", Title: "va"}, n.Publication)
}

func TestNotaPublica_NaoPublicadaInexistenteOuForaDoFormato_MesmoErro(t *testing.T) {
	db, leitura := pastaPublicadaComVizinhas(t)
	despublicada := idPublicoDe(t, db, "guia")
	despublica(t, db, "p-docs")

	for _, id := range []string{despublicada, "AAAAAAAAAAAA", "../segredo", ""} {
		_, err := leitura.PublicNote(context.Background(), id)

		require.ErrorIs(t, err, publication.ErrPublicNoteNotFound, "id %q", id)
	}
}

func TestArvorePublica_SoAPublicacaoComCaminhoRelativo(t *testing.T) {
	db, leitura := pastaPublicadaComVizinhas(t)

	itens, total, err := leitura.PublicTree(context.Background(), idPublicoDe(t, db, "passo"), 0, 1)

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, []publication.PublicTreeItem{{PublicID: idPublicoDe(t, db, "guia"), Title: "Guia de deploy", Path: "Guia de deploy.md"}}, itens)
	itens, _, err = leitura.PublicTree(context.Background(), idPublicoDe(t, db, "passo"), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, "x/Passo.md", itens[0].Path)
}

func TestBuscaPublica_SoNaPublicacaoComTrecho(t *testing.T) {
	db, leitura := pastaPublicadaComVizinhas(t)

	hits, total, err := leitura.SearchPublic(context.Background(), idPublicoDe(t, db, "guia"), "deploy", 0, 10)

	require.NoError(t, err)
	assert.Equal(t, 2, total, "a nota privada tambem fala de deploy")
	require.Len(t, hits, 2)
	for _, h := range hits {
		assert.NotEqual(t, "Segredo", h.Title)
		assert.Contains(t, h.Snippet, "\x02")
	}
}

func TestGrafoPublico_NosPeloIDPublicoESoArestasDaPublicacao(t *testing.T) {
	db, leitura := pastaPublicadaComVizinhas(t)
	guia, passo := idPublicoDe(t, db, "guia"), idPublicoDe(t, db, "passo")

	g, err := leitura.PublicGraph(context.Background(), passo)

	require.NoError(t, err)
	require.Len(t, g.Nodes, 2)
	assert.Equal(t, []string{guia, passo}, []string{g.Nodes[0].PublicID, g.Nodes[1].PublicID})
	assert.Equal(t, 3, *g.Nodes[0].Degree, "o grau so conta as ligacoes publicas")
	for _, l := range g.Links {
		assert.Contains(t, []int{0, 1}, l.Source)
		assert.Contains(t, []int{0, 1}, l.Target)
	}
}

func TestEtiquetaDoGrafoPublico_MudaComAPublicacao(t *testing.T) {
	db, leitura := pastaPublicadaComVizinhas(t)
	guia := idPublicoDe(t, db, "guia")
	antes, err := leitura.PublicGraphTag(context.Background(), guia)
	require.NoError(t, err)

	publica(t, db, "p-cofre", "t1", "va", alvoCofre())

	depois, err := leitura.PublicGraphTag(context.Background(), guia)
	require.NoError(t, err)
	assert.NotEmpty(t, antes)
	assert.NotEqual(t, antes, depois)
}
