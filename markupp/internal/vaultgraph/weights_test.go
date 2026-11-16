package vaultgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_ProjecaoNaoDirigidaFicaComOPesoMaiorDosDoisSentidos(t *testing.T) {
	g := New([]string{"a", "b", "c"}, []Edge{
		{Source: "a", Target: "b", Automatic: true}, {Source: "b", Target: "a"}, {Source: "b", Target: "c", Automatic: true},
	})

	assert.Equal(t, [][]weightedEdge{
		{{to: 1, weight: 1}},
		{{to: 0, weight: 1}, {to: 2, weight: automaticWeight}},
		{{to: 1, weight: automaticWeight}},
	}, g.undirectedWeighted)
}

func TestCommunities_NotaFicaComQuemElaLigaPorEscrito(t *testing.T) {
	var edges []Edge
	for _, clique := range [][]string{{"a1", "a2", "a3", "a4"}, {"b1", "b2", "b3", "b4"}} {
		for i := range clique {
			for j := i + 1; j < len(clique); j++ {
				edges = append(edges, Edge{Source: clique[i], Target: clique[j]})
			}
		}
	}
	edges = append(edges, Edge{Source: "x", Target: "a1"}, Edge{Source: "x", Target: "a2"},
		Edge{Source: "x", Target: "b1", Automatic: true}, Edge{Source: "x", Target: "b2", Automatic: true},
		Edge{Source: "x", Target: "b3", Automatic: true})
	g := New([]string{"a1", "a2", "a3", "a4", "b1", "b2", "b3", "b4", "x"}, edges)

	comunidades, _ := g.Communities()

	assert.Equal(t, [][]string{{"a1", "a2", "a3", "a4", "x"}, {"b1", "b2", "b3", "b4"}}, comunidades,
		"sem peso as tres automaticas venceriam as duas escritas")
}
