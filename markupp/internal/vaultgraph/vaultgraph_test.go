package vaultgraph_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"

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

	rank, _ := g.PageRank()

	assert.InDelta(t, 0.350877, rank["a"], 1e-5, "a massa da nota sem saida se espalha por todas")
	assert.InDelta(t, 0.649123, rank["b"], 1e-5)
}

func TestPageRank_NotaCitadaPorTodasLidera(t *testing.T) {
	g := vaultgraph.New([]string{"x", "y", "z", "indice"}, []vaultgraph.Edge{
		{Source: "x", Target: "indice"}, {Source: "y", Target: "indice"}, {Source: "z", Target: "indice"}, {Source: "indice", Target: "x"},
	})

	rank, _ := g.PageRank()

	assert.Greater(t, rank["indice"], rank["x"])
	assert.Greater(t, rank["x"], rank["y"])
	assert.InDelta(t, 1, rank["x"]+rank["y"]+rank["z"]+rank["indice"], 1e-9)
}

func TestPageRank_ConfereComOGonum(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7)) //nolint:gosec // G404: o grafo do teste precisa ser reprodutivel, nao imprevisivel
	ids := make([]string, 300)
	referencia := simple.NewDirectedGraph()
	for i := range ids {
		ids[i] = fmt.Sprintf("n%03d", i)
		referencia.AddNode(simple.Node(i))
	}
	var edges []vaultgraph.Edge
	for range 900 {
		from, to := rng.IntN(250), rng.IntN(300)
		if from != to {
			edges = append(edges, vaultgraph.Edge{Source: ids[from], Target: ids[to]})
			referencia.SetEdge(simple.Edge{F: simple.Node(from), T: simple.Node(to)})
		}
	}

	rank, _ := vaultgraph.New(ids, edges).PageRank()

	esperado := network.PageRankSparse(referencia, 0.85, 1e-10)
	for i, id := range ids {
		assert.InDelta(t, esperado[int64(i)], rank[id], 1e-6, "nota %s", id)
	}
}

func TestPageRank_CicloPequenoConvergeAntesDoTeto(t *testing.T) {
	g := vaultgraph.New([]string{"a", "b", "c", "d"}, []vaultgraph.Edge{
		{Source: "a", Target: "b"}, {Source: "b", Target: "c"}, {Source: "c", Target: "a"}, {Source: "d", Target: "a"},
	})

	_, parada := g.PageRank()

	assert.False(t, parada.Capped, "parada: %+v", parada)
	assert.Less(t, parada.Iterations, 100, "o ciclo gira a diferenca a 0,85 por iteracao e cai abaixo de N x 1e-6 na norma 1")
}

func TestPageRank_ConvergeAntesDoTeto(t *testing.T) {
	_, parada := vaultgraph.New([]string{"a", "b", "c"}, nil).PageRank()

	assert.Equal(t, vaultgraph.Convergence{Iterations: 1}, parada, "sem ligacao, o vetor uniforme ja e o PageRank")
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
	comunidades, parada := doisTriangulos().Communities()

	assert.Equal(t, [][]string{{"a", "b", "c"}, {"d", "e", "f"}}, comunidades)
	assert.False(t, parada.Capped, "parada: %+v", parada)
}

func TestCommunities_AnelDeCliquesSaiUmaComunidadePorClique(t *testing.T) {
	var ids []string
	var edges []vaultgraph.Edge
	var esperado [][]string
	for c := range 6 {
		var clique []string
		for i := range 5 {
			clique = append(clique, fmt.Sprintf("c%d-%d", c, i))
		}
		for i := range clique {
			for j := i + 1; j < len(clique); j++ {
				edges = append(edges, vaultgraph.Edge{Source: clique[i], Target: clique[j]})
			}
		}
		edges = append(edges, vaultgraph.Edge{Source: clique[0], Target: fmt.Sprintf("c%d-1", (c+1)%6)})
		ids = append(ids, clique...)
		esperado = append(esperado, clique)
	}

	comunidades, _ := vaultgraph.New(ids, edges).Communities()
	assert.Equal(t, esperado, comunidades)
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

	primeira, _ := vaultgraph.New(ids, edges).Communities()
	for range 5 {
		outra, _ := vaultgraph.New(ids, edges).Communities()
		assert.Equal(t, primeira, outra)
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
