package vaultgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// triangulosComPonte é a lista de vizinhos de dois triângulos, 0-1-2 e 3-4-5, ligados
// pela ponte 2-3.
func triangulosComPonte() [][]int {
	return [][]int{{1, 2}, {0, 2}, {0, 1, 3}, {2, 4, 5}, {3, 5}, {3, 4}}
}

func TestLouvain_ConvergeAntesDosTetos(t *testing.T) {
	membros, parada := louvain(triangulosComPonte(), louvainSeed, louvainCaps)

	assert.Equal(t, []int{0, 0, 0, 1, 1, 1}, membros)
	assert.False(t, parada.Capped, "parada: %+v", parada)
	assert.LessOrEqual(t, parada.Iterations, 10)
}

func TestLouvain_ParaNoTetoDeNiveis(t *testing.T) {
	_, parada := louvain(triangulosComPonte(), louvainSeed, louvainLimits{levels: 1, passes: 100})

	assert.Equal(t, 1, parada.Iterations)
	assert.True(t, parada.Capped, "o primeiro nivel moveu nos e nao houve nivel para confirmar a particao")
}

func TestLouvain_ParaNoTetoDePassadas(t *testing.T) {
	_, parada := louvain(triangulosComPonte(), louvainSeed, louvainLimits{levels: 10, passes: 1})

	assert.Equal(t, 1, parada.Passes)
	assert.True(t, parada.Capped, "a primeira passada moveu nos e o teto impediu a segunda")
}
