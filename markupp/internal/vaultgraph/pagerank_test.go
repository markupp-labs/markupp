package vaultgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPageRank_ParaNoTetoDeIteracoes(t *testing.T) {
	g := New([]string{"a", "b", "c"}, []Edge{{Source: "a", Target: "b"}, {Source: "b", Target: "c"}, {Source: "c", Target: "a"}, {Source: "a", Target: "c"}})

	_, parada := pageRank(g.directed.from, g.directed.weights, damping, pageRankTolerance, 2)

	assert.Equal(t, Convergence{Iterations: 2, Capped: true}, parada, "a diferenca da primeira iteracao ainda passa da tolerancia")
}

func TestPageRank_CofreVazio_NaoChegaAoTeto(t *testing.T) {
	ranks, parada := New(nil, nil).PageRank()

	assert.Empty(t, ranks)
	assert.False(t, parada.Capped, "sem nota nao ha o que iterar")
}
