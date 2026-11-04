package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

// leituraPublica semeia doisCofresUmPublicado e devolve a leitura pública sobre ele.
// O contexto não tem ator: a leitura pública não depende de conta.
func leituraPublica(t *testing.T) *storage.PostgresPublicVaults {
	t.Helper()
	return storage.NewPostgresPublicVaults(doisCofresUmPublicado(t), "simple")
}

func TestLeituraPublica_CofrePublicado_DevolveSlugTituloEVersao(t *testing.T) {
	r := leituraPublica(t)

	vault, err := r.PublicVault(context.Background(), "docs")

	require.NoError(t, err)
	assert.Equal(t, "pub", vault.VaultID)
	assert.Equal(t, "docs", vault.Slug)
	assert.Equal(t, "docs", vault.Title)
	assert.Positive(t, vault.GraphVersion)
}

func TestLeituraPublica_SlugInexistente_DevolveNaoPublicado(t *testing.T) {
	r := leituraPublica(t)

	_, err := r.PublicVault(context.Background(), "priv")

	require.ErrorIs(t, err, publication.ErrNotPublished)
	assert.Contains(t, err.Error(), `"priv"`)
}

func TestLeituraPublica_Notas_SoAsDoCofrePublicado(t *testing.T) {
	r := leituraPublica(t)

	list, err := r.PublicNotes(context.Background(), "docs", notes.VaultNotesQuery{Limit: 10})

	require.NoError(t, err)
	caminhos := []string{}
	for _, n := range list {
		caminhos = append(caminhos, n.Path)
	}
	assert.Equal(t, []string{"guia.md", "inicio.md"}, caminhos)
}

func TestLeituraPublica_Nota_DevolveConteudoEOrganizacao(t *testing.T) {
	r := leituraPublica(t)

	n, err := r.PublicNote(context.Background(), "docs", "p1")

	require.NoError(t, err)
	assert.Equal(t, "leia o [[inicio]] antes do guia", n.Content)
	assert.NotNil(t, n.Rank.Degree)
}

func TestLeituraPublica_NotaDeCofrePrivado_DevolveNaoEncontrada(t *testing.T) {
	r := leituraPublica(t)

	_, err := r.PublicNote(context.Background(), "docs", "s1")

	require.ErrorIs(t, err, notes.ErrNotFound)
}

func TestLeituraPublica_NotaDeOutroCofrePublicado_DevolveNaoEncontrada(t *testing.T) {
	db := doisCofresUmPublicado(t)
	publicaCofre(t, db, "t2", "priv", "outro")
	r := storage.NewPostgresPublicVaults(db, "simple")

	_, err := r.PublicNote(context.Background(), "docs", "s1")

	require.ErrorIs(t, err, notes.ErrNotFound)
	assert.Contains(t, err.Error(), `"s1"`)
}

func TestLeituraPublica_Busca_SoNoCofrePublicado(t *testing.T) {
	r := leituraPublica(t)

	hits, err := r.SearchPublicNotes(context.Background(), "docs", "guia", 0, 10)

	require.NoError(t, err)
	ids := []string{}
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	assert.ElementsMatch(t, []string{"p1", "p2"}, ids)
}

func TestLeituraPublica_Grafo_LigaAsNotasDoCofre(t *testing.T) {
	r := leituraPublica(t)

	version, err := r.PublicGraphVersion(context.Background(), "docs")
	require.NoError(t, err)
	graph, err := r.PublicGraph(context.Background(), "docs", notes.GraphFilter{})

	require.NoError(t, err)
	assert.Positive(t, version)
	assert.Len(t, graph.Nodes, 2)
	assert.Len(t, graph.Links, 1)
}

func TestLeituraPublica_Ligacoes_DoCofrePublicado(t *testing.T) {
	r := leituraPublica(t)

	links, err := r.PublicLinks(context.Background(), "docs", notes.VaultLinkQuery{LinkFilter: notes.LinkFilter{Page: notes.Page{Limit: 10}}})

	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "p1", links[0].Source.ID)
	require.NotNil(t, links[0].Target)
	assert.Equal(t, "p2", links[0].Target.ID)
}

func TestLeituraPublica_DepoisDeDespublicar_DevolveNaoPublicado(t *testing.T) {
	db := doisCofresUmPublicado(t)
	_, err := db.Exec(context.Background(), "DELETE FROM vault_publications")
	require.NoError(t, err)
	r := storage.NewPostgresPublicVaults(db, "simple")

	_, err = r.PublicNotes(context.Background(), "docs", notes.VaultNotesQuery{Limit: 10})

	require.ErrorIs(t, err, publication.ErrNotPublished)
}
