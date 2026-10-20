package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// arestasDoGrafo descreve cada ligação do grafo como "origem->destino:tipo", pelos
// títulos dos nós.
func arestasDoGrafo(g notes.Graph) []string {
	out := make([]string, 0, len(g.Links))
	for _, l := range g.Links {
		out = append(out, g.Nodes[l.Source].Title+"->"+g.Nodes[l.Target].Title+":"+l.Type)
	}
	return out
}

// nosDoGrafo devolve o título de cada nó, com "?" na frente do nó sem nota.
func nosDoGrafo(g notes.Graph) []string {
	out := make([]string, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		if n.Unresolved {
			out = append(out, "?"+n.Title)
			continue
		}
		out = append(out, n.Title)
	}
	return out
}

var arestasDosDoisGrupos = []string{
	"a->b:explicit", "a->indice:explicit", "b->c:explicit", "b->indice:explicit", "c->a:explicit", "c->d:explicit",
	"c->indice:explicit", "d->e:explicit", "d->indice:inferred", "e->f:explicit", "f->d:explicit", "d->indise:explicit",
}

func TestVaultGraph_NotasLigacoesENoSemNota(t *testing.T) {
	_, _, graph := cofreComDoisGrupos(t)

	g, err := graph.VaultGraph(ctxDeTeste, cofreDeTeste, notes.GraphFilter{})

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c", "d", "e", "f", "indice", "solta", "?indise"}, nosDoGrafo(g))
	assert.ElementsMatch(t, arestasDosDoisGrupos, arestasDoGrafo(g))
	indice := g.Nodes[6]
	assert.Equal(t, "indice.md", *indice.Path)
	assert.Equal(t, 4, *indice.Degree)
	assert.NotNil(t, indice.PageRank)
	assert.NotNil(t, indice.Community)
	assert.Nil(t, g.Nodes[8].ID, "o no sem nota nao tem id")
}

func TestVaultGraph_FiltrosEscondemOrfasSemNotaEInferidas(t *testing.T) {
	_, _, graph := cofreComDoisGrupos(t)

	g, err := graph.VaultGraph(ctxDeTeste, cofreDeTeste, notes.GraphFilter{HideOrphans: true, HideUnresolved: true, HideInferred: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c", "d", "e", "f", "indice"}, nosDoGrafo(g))
	assert.ElementsMatch(t, []string{
		"a->b:explicit", "a->indice:explicit", "b->c:explicit", "b->indice:explicit", "c->a:explicit",
		"c->d:explicit", "c->indice:explicit", "d->e:explicit", "e->f:explicit", "f->d:explicit",
	}, arestasDoGrafo(g))
}

func TestVaultGraph_RecortaPelaComunidade(t *testing.T) {
	_, repo, graph := cofreComDoisGrupos(t)
	comunidade := idDaComunidade(comunidadesLidas(t, repo, graph), "d")

	g, err := graph.VaultGraph(ctxDeTeste, cofreDeTeste, notes.GraphFilter{Community: &comunidade})

	require.NoError(t, err)
	assert.Equal(t, []string{"d", "e", "f", "?indise"}, nosDoGrafo(g))
	assert.ElementsMatch(t, []string{"d->e:explicit", "e->f:explicit", "f->d:explicit", "d->indise:explicit"}, arestasDoGrafo(g))
}

func TestLocalGraph_VizinhancaPelaProfundidadeNosDoisSentidos(t *testing.T) {
	_, _, graph := cofreComDoisGrupos(t)

	g1, err := graph.LocalGraph(ctxDeTeste, "e", 1, notes.GraphFilter{})
	require.NoError(t, err)
	g2, err := graph.LocalGraph(ctxDeTeste, "e", 2, notes.GraphFilter{})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"d", "e", "f"}, nosDoGrafo(g1))
	assert.ElementsMatch(t, []string{"d->e:explicit", "e->f:explicit", "f->d:explicit"}, arestasDoGrafo(g1), "o no sem nota de d fica a dois saltos")
	assert.ElementsMatch(t, []string{"c", "d", "e", "f", "indice", "?indise"}, nosDoGrafo(g2))
	assert.ElementsMatch(t, []string{
		"c->d:explicit", "c->indice:explicit", "d->e:explicit", "d->indice:inferred", "e->f:explicit", "f->d:explicit", "d->indise:explicit",
	}, arestasDoGrafo(g2))
}

func TestLocalGraph_FiltroValeNoCaminhoEANotaLidaFica(t *testing.T) {
	_, _, graph := cofreComDoisGrupos(t)

	semInferidas, err := graph.LocalGraph(ctxDeTeste, "e", 2, notes.GraphFilter{HideInferred: true, HideUnresolved: true})
	require.NoError(t, err)
	solta, err := graph.LocalGraph(ctxDeTeste, "solta", 3, notes.GraphFilter{HideOrphans: true})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"c", "d", "e", "f"}, nosDoGrafo(semInferidas), "indice so chegava pela inferida")
	assert.Equal(t, []string{"solta"}, nosDoGrafo(solta))
}

func TestGraphVersion_DoCofreEDaNota(t *testing.T) {
	db, _, graph := cofreComDoisGrupos(t)
	gravada := versaoDoGrafo(t, db, cofreDeTeste)

	doCofre, err := graph.VaultGraphVersion(ctxDeTeste, cofreDeTeste)
	require.NoError(t, err)
	daNota, err := graph.NoteGraphVersion(ctxDeTeste, "e")
	require.NoError(t, err)
	vazio := storagetest.SeedMember(t, db, membroDeTeste, "cofre-vazio")
	semVersao, err := graph.VaultGraphVersion(vazio, "cofre-vazio")
	require.NoError(t, err)

	assert.Equal(t, []int64{gravada, gravada, 0}, []int64{doCofre, daNota, semVersao})
}

func TestGraph_ViewerLeEEstranhoNao(t *testing.T) {
	db, _, graph := cofreComDoisGrupos(t)
	viewer := identity.Actor{UserID: "u-viewer", TenantID: membroDeTeste.TenantID, Role: "viewer"}
	ctxViewer := storagetest.SeedMember(t, db, viewer, cofreDeTeste)
	estranho := identity.Actor{UserID: "u-fora", TenantID: "outro", Role: "admin"}
	ctxFora := storagetest.SeedMember(t, db, estranho, "cofre-fora")

	g, err := graph.VaultGraph(ctxViewer, cofreDeTeste, notes.GraphFilter{})
	require.NoError(t, err)
	assert.Len(t, g.Nodes, 9)
	versao, err := graph.VaultGraphVersion(ctxViewer, cofreDeTeste)
	require.NoError(t, err)
	assert.Positive(t, versao)
	exigeInvisivel(ctxFora, t, graph)
}

// exigeInvisivel confere que o ator de ctx não lê o grafo nem a versão do cofre de
// teste nem da nota e dele.
func exigeInvisivel(ctx context.Context, t *testing.T, graph *storage.PostgresNoteGraph) {
	t.Helper()
	_, err := graph.VaultGraph(ctx, cofreDeTeste, notes.GraphFilter{})
	require.ErrorIs(t, err, notes.ErrVaultForbidden)
	_, err = graph.VaultGraphVersion(ctx, cofreDeTeste)
	require.ErrorIs(t, err, notes.ErrVaultForbidden)
	_, err = graph.LocalGraph(ctx, "e", 1, notes.GraphFilter{})
	require.ErrorIs(t, err, notes.ErrNotFound)
	_, err = graph.NoteGraphVersion(ctx, "e")
	require.ErrorIs(t, err, notes.ErrNotFound)
}
