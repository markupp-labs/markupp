package vaultgraph

import (
	"cmp"
	"maps"
	"slices"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

// adjacency é o grafo de índices 0..n-1 no formato que o gonum percorre, com os
// vizinhos sempre em ordem de id: o grafo do pacote simple percorre mapas, e a
// ordem de visita mudaria a partição do Louvain de uma execução para outra.
type adjacency struct {
	from    [][]graph.Node
	to      [][]graph.Node
	weights [][]float64 // peso de cada ligação de from, na mesma ordem
}

var (
	_ graph.Directed   = adjacency{}
	_ graph.Undirected = adjacency{}
)

// Node devolve o nó id, ou nil fora do grafo.
func (a adjacency) Node(id int64) graph.Node {
	if id < 0 || id >= int64(len(a.from)) {
		return nil
	}
	return simple.Node(id)
}

// Nodes devolve os nós em ordem de id.
func (a adjacency) Nodes() graph.Nodes {
	nodes := make([]graph.Node, len(a.from))
	for i := range nodes {
		nodes[i] = simple.Node(i)
	}
	return iterator.NewOrderedNodes(nodes)
}

// From devolve, em ordem de id, os nós a que id liga.
func (a adjacency) From(id int64) graph.Nodes {
	return iterator.NewOrderedNodes(a.from[id])
}

// To devolve, em ordem de id, os nós que ligam a id.
func (a adjacency) To(id int64) graph.Nodes {
	return iterator.NewOrderedNodes(a.to[id])
}

// HasEdgeFromTo diz se u liga a v.
func (a adjacency) HasEdgeFromTo(uid, vid int64) bool {
	_, found := slices.BinarySearchFunc(a.from[uid], vid, func(n graph.Node, id int64) int { return cmp.Compare(n.ID(), id) })
	return found
}

// HasEdgeBetween diz se x e y se ligam em algum sentido.
func (a adjacency) HasEdgeBetween(xid, yid int64) bool {
	return a.HasEdgeFromTo(xid, yid) || a.HasEdgeFromTo(yid, xid)
}

// Edge devolve a aresta de u para v, ou nil se ela não existe.
func (a adjacency) Edge(uid, vid int64) graph.Edge {
	if !a.HasEdgeFromTo(uid, vid) {
		return nil
	}
	return simple.Edge{F: simple.Node(uid), T: simple.Node(vid)}
}

// EdgeBetween devolve a aresta entre x e y; só faz sentido na projeção não dirigida,
// em que from e to são iguais.
func (a adjacency) EdgeBetween(xid, yid int64) graph.Edge {
	return a.Edge(xid, yid)
}

// adjacencyBuilder junta as arestas sem repetição, com o peso maior da repetida,
// antes de ordenar os vizinhos.
type adjacencyBuilder struct {
	out []map[int64]float64
}

func newAdjacencyBuilder(n int) *adjacencyBuilder {
	b := &adjacencyBuilder{out: make([]map[int64]float64, n)}
	for i := range b.out {
		b.out[i] = make(map[int64]float64)
	}
	return b
}

func (b *adjacencyBuilder) add(from, to int64, weight float64) {
	b.out[from][to] = max(b.out[from][to], weight)
}

// build devolve o grafo dirigido, a projeção não dirigida dele e os pesos da
// projeção, em que o par ligado nos dois sentidos fica com o peso maior.
func (b *adjacencyBuilder) build() (directed, undirected adjacency, undirectedWeighted [][]weightedEdge) {
	n := len(b.out)
	directed = adjacency{from: make([][]graph.Node, n), to: make([][]graph.Node, n), weights: make([][]float64, n)}
	both := newAdjacencyBuilder(n).out
	for from, targets := range b.out {
		for _, to := range slices.Sorted(maps.Keys(targets)) {
			w := targets[to]
			directed.from[from] = append(directed.from[from], simple.Node(to))
			directed.weights[from] = append(directed.weights[from], w)
			directed.to[to] = append(directed.to[to], simple.Node(from))
			both[from][to], both[to][int64(from)] = max(both[from][to], w), max(both[to][int64(from)], w)
		}
	}
	sortEach(directed.to)
	undirected = adjacency{from: mergeSorted(directed.from, directed.to)}
	undirected.to = undirected.from
	return directed, undirected, weightedNeighbors(both)
}

// weightedNeighbors passa os pesos de cada nó para a lista de vizinhos em ordem.
func weightedNeighbors(both []map[int64]float64) [][]weightedEdge {
	out := make([][]weightedEdge, len(both))
	for i, ws := range both {
		for _, j := range slices.Sorted(maps.Keys(ws)) {
			out[i] = append(out[i], weightedEdge{to: int(j), weight: ws[j]})
		}
	}
	return out
}

// mergeSorted junta, nó a nó, duas listas de vizinhos, em ordem e sem repetir.
func mergeSorted(a, b [][]graph.Node) [][]graph.Node {
	out := make([][]graph.Node, len(a))
	for i := range a {
		out[i] = slices.CompactFunc(slices.SortedFunc(slices.Values(slices.Concat(a[i], b[i])), byID),
			func(x, y graph.Node) bool { return x.ID() == y.ID() })
	}
	return out
}

func sortEach(lists [][]graph.Node) {
	for _, list := range lists {
		slices.SortFunc(list, byID)
	}
}

func byID(a, b graph.Node) int {
	return cmp.Compare(a.ID(), b.ID())
}
