// Package vaultgraph calcula as medidas do grafo de ligações de um cofre inteiro:
// grau, alocação de recursos, PageRank e comunidades. O gonum, que calcula o
// PageRank, fica atrás deste pacote; quem chama vê só ids de nota.
package vaultgraph

import (
	"slices"

	"gonum.org/v1/gonum/graph/network"
)

const (
	// damping é o fator de amortecimento do PageRank, o valor canônico de Page et al. (1998).
	damping = 0.85
	// pageRankTolerance é a norma 2 da diferença entre iterações que encerra o PageRank.
	pageRankTolerance = 1e-10
	// louvainSeed fixa a ordem de visita do Louvain: o mesmo grafo dá a mesma partição.
	louvainSeed = 20261016
)

// Edge é uma ligação resolvida de Source para Target.
type Edge struct {
	Source string
	Target string
}

// Graph é o grafo dirigido das ligações do cofre, sem laço e sem aresta repetida,
// com a projeção não dirigida ao lado.
type Graph struct {
	ids        []string
	index      map[string]int64
	directed   adjacency
	undirected adjacency
}

// New monta o grafo das notas noteIDs. Ligação para si mesma, repetida ou com ponta
// fora de noteIDs fica de fora.
func New(noteIDs []string, edges []Edge) *Graph {
	g := &Graph{ids: slices.Clone(noteIDs), index: make(map[string]int64, len(noteIDs))}
	for i, id := range g.ids {
		g.index[id] = int64(i)
	}
	builder := newAdjacencyBuilder(len(g.ids))
	for _, e := range edges {
		from, okFrom := g.index[e.Source]
		to, okTo := g.index[e.Target]
		if okFrom && okTo && from != to {
			builder.add(from, to)
		}
	}
	g.directed, g.undirected = builder.build()
	return g
}

// Degree devolve quantas notas ligam à nota id e a quantas ela liga.
func (g *Graph) Degree(id string) (in, out int) {
	i, ok := g.index[id]
	if !ok {
		return 0, 0
	}
	return len(g.directed.to[i]), len(g.directed.from[i])
}

// ResourceAllocation é a alocação de recursos (Zhou, Lü e Zhang, 2009) entre a e b
// na projeção não dirigida: a soma de 1/grau de cada vizinho em comum.
func (g *Graph) ResourceAllocation(a, b string) float64 {
	ia, okA := g.index[a]
	ib, okB := g.index[b]
	if !okA || !okB {
		return 0
	}
	score := 0.0
	for _, z := range g.undirected.from[ia] {
		if g.undirected.HasEdgeFromTo(z.ID(), ib) {
			score += 1 / float64(len(g.undirected.from[z.ID()]))
		}
	}
	return score
}

// PageRank devolve o PageRank de cada nota com d=0,85; a massa de nota sem ligação
// de saída se espalha por todas.
func (g *Graph) PageRank() map[string]float64 {
	ranks := network.PageRankSparse(g.directed, damping, pageRankTolerance)
	out := make(map[string]float64, len(ranks))
	for i, r := range ranks {
		out[g.ids[i]] = r
	}
	return out
}

// Communities devolve as comunidades do Louvain na projeção não dirigida, com
// resolução 1 e semente fixa. Nota sozinha não é comunidade e fica de fora. Cada
// comunidade sai com os ids em ordem, e a lista por tamanho e pelo primeiro id.
func (g *Graph) Communities() [][]string {
	groups := make(map[int][]string)
	for i, c := range louvain(g.undirectedIndexes(), louvainSeed) {
		groups[c] = append(groups[c], g.ids[i])
	}
	var out [][]string
	for _, ids := range groups {
		if len(ids) > 1 {
			slices.Sort(ids)
			out = append(out, ids)
		}
	}
	slices.SortFunc(out, bySizeThenFirst)
	return out
}

// undirectedIndexes devolve os vizinhos de cada nota na projeção não dirigida, por
// índice e em ordem.
func (g *Graph) undirectedIndexes() [][]int {
	adj := make([][]int, len(g.ids))
	for i, ns := range g.undirected.from {
		for _, n := range ns {
			adj[i] = append(adj[i], int(n.ID()))
		}
	}
	return adj
}

func bySizeThenFirst(a, b []string) int {
	if len(a) != len(b) {
		return len(b) - len(a)
	}
	return slices.Compare(a, b)
}
