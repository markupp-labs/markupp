package publication_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

func TestBuildPublicGraph_IndicesEGrauPelasLigacoesPublicas(t *testing.T) {
	nos := []publication.PublicGraphNode{{PublicID: "aaaaaaaaaaaa", Title: "A"}, {PublicID: "bbbbbbbbbbbb", Title: "B"}}
	ligacoes := []publication.PublicEdge{
		{SourcePublicID: "aaaaaaaaaaaa", TargetPublicID: "bbbbbbbbbbbb", Type: "explicit"},
		{SourcePublicID: "bbbbbbbbbbbb", TargetPublicID: "aaaaaaaaaaaa", Type: "embed"},
		{SourcePublicID: "aaaaaaaaaaaa", TargetPublicID: "cccccccccccc", Type: "explicit"},
	}

	g := publication.BuildPublicGraph(nos, ligacoes)

	assert.Equal(t, []publication.PublicGraphLink{{Source: 0, Target: 1, Type: "explicit"}, {Source: 1, Target: 0, Type: "embed"}}, g.Links)
	assert.Equal(t, 2, *g.Nodes[0].Degree)
	assert.Equal(t, 2, *g.Nodes[1].Degree)
}

func TestGraphTag_VersaoZeroNaoValidaEPublicacaoMudaATag(t *testing.T) {
	assert.Empty(t, publication.GraphTag(0, "p1"))
	assert.NotEqual(t, publication.GraphTag(3, "p1"), publication.GraphTag(3, "p2"))
	assert.NotEqual(t, publication.GraphTag(3, "p1"), publication.GraphTag(4, "p1"))
	assert.NotContains(t, publication.GraphTag(3, "p1"), "p1", "o id da publicacao nao sai")
}
