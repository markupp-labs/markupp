package vaultgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPageRank_ParaNoTetoDeIteracoes(t *testing.T) {
	g := New([]string{"a", "b", "c"}, []Edge{{Source: "a", Target: "b"}, {Source: "b", Target: "c"}, {Source: "c", Target: "a"}, {Source: "a", Target: "c"}})

	_, parada := pageRank(g.directed.from, damping, pageRankTolerance, 2)

	assert.Equal(t, Convergence{Iterations: 2, Capped: true}, parada, "a diferenca da primeira iteracao ainda passa da tolerancia")
}
