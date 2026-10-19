package vaultgraph

import (
	"maps"
	"math/rand/v2"
	"slices"
)

// minModularityGain é o ganho mínimo para mover um nó de comunidade; abaixo disso a
// diferença é arredondamento de ponto flutuante.
const minModularityGain = 1e-12

// weightedGraph é um nível do Louvain: o grafo não dirigido de pesos simétricos, com
// o peso do laço de cada nó à parte e o grau ponderado k de cada um, laço incluído.
type weightedGraph struct {
	neighbors [][]weightedEdge
	selfLoop  []float64
	degree    []float64
	total     float64 // 2m, a soma dos graus
}

type weightedEdge struct {
	to     int
	weight float64
}

// louvain devolve a comunidade de cada nó do grafo de vizinhos adj, simétrico e sem
// laço, pelo Louvain (Blondel et al., 2008) com resolução 1: movimentos locais que
// aumentam a modularidade, agregação de cada comunidade num nó e repetição até não
// haver movimento. A ordem de visita sai de uma semente fixa, então o mesmo grafo
// dá sempre a mesma partição. O gonum faz o mesmo, mas soma o grau da comunidade
// membro a membro a cada movimento, e levava dezenas de segundos num cofre de 10 mil
// notas.
func louvain(adj [][]int, seed uint64) []int {
	membership := make([]int, len(adj))
	for i := range membership {
		membership[i] = i
	}
	rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // G404: a ordem precisa ser reprodutivel, nao imprevisivel
	g := unitGraph(adj)
	for {
		communities, moved := g.moveNodes(rng)
		if !moved {
			return membership
		}
		for i := range membership {
			membership[i] = communities[membership[i]]
		}
		g = g.aggregate(communities)
	}
}

// unitGraph é o primeiro nível: peso 1 em cada ligação.
func unitGraph(adj [][]int) *weightedGraph {
	g := newWeightedGraph(len(adj))
	for i, ns := range adj {
		for _, j := range ns {
			g.neighbors[i] = append(g.neighbors[i], weightedEdge{to: j, weight: 1})
		}
	}
	g.computeDegrees()
	return g
}

func newWeightedGraph(n int) *weightedGraph {
	return &weightedGraph{neighbors: make([][]weightedEdge, n), selfLoop: make([]float64, n), degree: make([]float64, n)}
}

func (g *weightedGraph) computeDegrees() {
	g.total = 0
	for i := range g.neighbors {
		g.degree[i] = g.selfLoop[i]
		for _, e := range g.neighbors[i] {
			g.degree[i] += e.weight
		}
		g.total += g.degree[i]
	}
}

// moveNodes faz a fase de movimentos locais e devolve a comunidade de cada nó,
// renumerada de 0 em diante na ordem dos nós, e se algum nó mudou de comunidade.
func (g *weightedGraph) moveNodes(rng *rand.Rand) ([]int, bool) {
	l := newLocalMover(g)
	order := rng.Perm(len(g.neighbors))
	moved := false
	for improved := true; improved; {
		improved = false
		for _, i := range order {
			if l.moveBest(i) {
				improved, moved = true, true
			}
		}
	}
	return renumber(l.community), moved
}

// localMover guarda a comunidade de cada nó, o grau somado de cada comunidade e,
// durante a avaliação de um nó, o peso dele para cada comunidade vizinha.
type localMover struct {
	g         *weightedGraph
	community []int
	totals    []float64
	links     []float64
	touched   []int
}

func newLocalMover(g *weightedGraph) *localMover {
	n := len(g.neighbors)
	l := &localMover{g: g, community: make([]int, n), totals: make([]float64, n), links: make([]float64, n)}
	for i := range n {
		l.community[i], l.totals[i] = i, g.degree[i]
	}
	return l
}

// moveBest tira o nó i da comunidade dele e o põe na comunidade vizinha de maior
// ganho de modularidade, ou de volta na dele se nenhuma ganhar mais; diz se mudou.
func (l *localMover) moveBest(i int) bool {
	own := l.community[i]
	l.weighNeighbors(i)
	l.totals[own] -= l.g.degree[i]
	best, bestGain := own, l.gain(i, own)
	for _, c := range l.touched {
		if gain := l.gain(i, c); gain > bestGain+minModularityGain {
			best, bestGain = c, gain
		}
	}
	l.totals[best] += l.g.degree[i]
	l.community[i] = best
	l.clearNeighbors()
	return best != own
}

// gain é o ganho de modularidade, a menos de constante, de pôr o nó i na comunidade c.
func (l *localMover) gain(i, c int) float64 {
	return l.links[c] - l.totals[c]*l.g.degree[i]/l.g.total
}

func (l *localMover) weighNeighbors(i int) {
	for _, e := range l.g.neighbors[i] {
		c := l.community[e.to]
		if l.links[c] == 0 {
			l.touched = append(l.touched, c)
		}
		l.links[c] += e.weight
	}
}

func (l *localMover) clearNeighbors() {
	for _, c := range l.touched {
		l.links[c] = 0
	}
	l.touched = l.touched[:0]
}

// renumber troca os ids de comunidade por 0, 1, 2... na ordem do primeiro nó de cada uma.
func renumber(community []int) []int {
	ids := make(map[int]int)
	out := make([]int, len(community))
	for i, c := range community {
		if _, ok := ids[c]; !ok {
			ids[c] = len(ids)
		}
		out[i] = ids[c]
	}
	return out
}

// aggregate devolve o grafo em que cada comunidade vira um nó: o peso entre dois
// nós novos soma os pesos entre os membros, e o de dentro da comunidade vira laço.
func (g *weightedGraph) aggregate(communities []int) *weightedGraph {
	count := 0
	for _, c := range communities {
		count = max(count, c+1)
	}
	next := newWeightedGraph(count)
	weights := make([]map[int]float64, count)
	for i := range weights {
		weights[i] = make(map[int]float64)
	}
	for i, ns := range g.neighbors {
		next.selfLoop[communities[i]] += g.selfLoop[i]
		for _, e := range ns {
			weights[communities[i]][communities[e.to]] += e.weight
		}
	}
	next.fill(weights)
	return next
}

// fill passa os pesos agregados para listas de vizinhos em ordem de nó; o peso de
// uma comunidade para ela mesma soma ao laço.
func (g *weightedGraph) fill(weights []map[int]float64) {
	for c, ws := range weights {
		for _, d := range slices.Sorted(maps.Keys(ws)) {
			if d == c {
				g.selfLoop[c] += ws[d]
				continue
			}
			g.neighbors[c] = append(g.neighbors[c], weightedEdge{to: d, weight: ws[d]})
		}
	}
	g.computeDegrees()
}
