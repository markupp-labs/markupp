package vaultgraph_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaultgraph"
)

func TestNew_IgnoraLacoRepeticaoENotaDeFora(t *testing.T) {
	g := vaultgraph.New([]string{"a", "b"}, []vaultgraph.Edge{
		{Source: "a", Target: "b"}, {Source: "a", Target: "b"}, {Source: "a", Target: "a"}, {Source: "a", Target: "fora"},
	})

	in, out := g.Degree("a")
	assert.Equal(t, []int{0, 1}, []int{in, out})
	in, out = g.Degree("b")
	assert.Equal(t, []int{1, 0}, []int{in, out})
}

func TestResourceAllocation_SomaUmSobreOGrauDosVizinhosEmComum(t *testing.T) {
	g := vaultgraph.New([]string{"a", "b", "c", "d", "e"}, []vaultgraph.Edge{
		{Source: "a", Target: "b"}, {Source: "c", Target: "a"}, {Source: "b", Target: "d"},
		{Source: "d", Target: "c"}, {Source: "c", Target: "e"},
	})

	assert.InDelta(t, 1.0/2+1.0/3, g.ResourceAllocation("a", "d"), 1e-12, "projecao nao dirigida")
	assert.Zero(t, g.ResourceAllocation("a", "inexistente"))
}

func TestPageRank_AmortecimentoDe085ComNotaSemSaida(t *testing.T) {
	g := vaultgraph.New([]string{"a", "b"}, []vaultgraph.Edge{{Source: "a", Target: "b"}})

	rank := g.PageRank()

	assert.InDelta(t, 0.350877, rank["a"], 1e-5, "a massa da nota sem saida se espalha por todas")
	assert.InDelta(t, 0.649123, rank["b"], 1e-5)
}

func TestPageRank_NotaCitadaPorTodasLidera(t *testing.T) {
	g := vaultgraph.New([]string{"x", "y", "z", "indice"}, []vaultgraph.Edge{
		{Source: "x", Target: "indice"}, {Source: "y", Target: "indice"}, {Source: "z", Target: "indice"}, {Source: "indice", Target: "x"},
	})

	rank := g.PageRank()

	assert.Greater(t, rank["indice"], rank["x"])
	assert.Greater(t, rank["x"], rank["y"])
	assert.InDelta(t, 1, rank["x"]+rank["y"]+rank["z"]+rank["indice"], 1e-9)
}

// doisTriangulos é o grafo de dois triângulos ligados por uma ponte, mais uma nota
// sem ligação.
func doisTriangulos() *vaultgraph.Graph {
	return vaultgraph.New([]string{"a", "b", "c", "d", "e", "f", "solta"}, []vaultgraph.Edge{
		{Source: "a", Target: "b"}, {Source: "b", Target: "c"}, {Source: "c", Target: "a"},
		{Source: "d", Target: "e"}, {Source: "e", Target: "f"}, {Source: "f", Target: "d"},
		{Source: "c", Target: "d"},
	})
}

func TestCommunities_LouvainSeparaOsTriangulosESemNotaSozinha(t *testing.T) {
	assert.Equal(t, [][]string{{"a", "b", "c"}, {"d", "e", "f"}}, doisTriangulos().Communities())
}

func TestCommunities_Deterministicas(t *testing.T) {
	ids := make([]string, 0, 60)
	var edges []vaultgraph.Edge
	for i := range 60 {
		ids = append(ids, string(rune('A'+i)))
	}
	for i := range 60 {
		edges = append(edges, vaultgraph.Edge{Source: ids[i], Target: ids[(i*7+3)%60]}, vaultgraph.Edge{Source: ids[i], Target: ids[(i+1)%60]})
	}

	primeira := vaultgraph.New(ids, edges).Communities()
	for range 5 {
		assert.Equal(t, primeira, vaultgraph.New(ids, edges).Communities())
	}
}

func TestInheritCommunityIDs_HerdaComJaccardDeMeioOuMais(t *testing.T) {
	anteriores := map[string]int64{"a": 7, "b": 7, "c": 7, "d": 9, "e": 9}

	ids := vaultgraph.InheritCommunityIDs([][]string{{"a", "b", "c", "x"}, {"d", "e", "f", "g", "h"}}, anteriores)

	assert.Equal(t, []int64{7, 0}, ids, "3/4 herda; 2/5 ganha id novo, marcado com zero")
}

func TestInheritCommunityIDs_IdHerdadoUmaVezSo(t *testing.T) {
	anteriores := map[string]int64{"a": 1, "b": 1, "c": 1, "d": 1}

	ids := vaultgraph.InheritCommunityIDs([][]string{{"a", "b"}, {"c", "d"}}, anteriores)

	assert.Equal(t, []int64{1, 0}, ids, "a comunidade dividida ao meio passa o id so para a primeira")
}
