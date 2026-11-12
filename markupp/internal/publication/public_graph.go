package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// PublicEdge é uma ligação entre duas notas da mesma publicação, pelos ids públicos.
type PublicEdge struct {
	SourcePublicID string
	TargetPublicID string
	Type           string
}

// BuildPublicGraph liga os nós pelas arestas cujas pontas estão entre eles e conta o
// grau de cada nó só por essas arestas: a organização derivada do cofre inteiro contaria
// as ligações com notas privadas.
func BuildPublicGraph(nodes []PublicGraphNode, edges []PublicEdge) PublicGraph {
	index := make(map[string]int, len(nodes))
	degrees := make([]int, len(nodes))
	for i, n := range nodes {
		index[n.PublicID] = i
	}
	g := PublicGraph{Nodes: nodes, Links: []PublicGraphLink{}}
	for _, e := range edges {
		source, okSource := index[e.SourcePublicID]
		target, okTarget := index[e.TargetPublicID]
		if okSource && okTarget {
			g.Links = append(g.Links, PublicGraphLink{Source: source, Target: target, Type: e.Type})
			degrees[source]++
			degrees[target]++
		}
	}
	for i := range g.Nodes {
		g.Nodes[i].Degree = &degrees[i]
	}
	return g
}

// GraphTag é o validador do grafo público: a versão do grafo do cofre e a publicação,
// que muda quando outra publicação passa a cobrir a nota. Vazio na versão zero, antes do
// primeiro segundo estágio, que não valida nada. O id da publicação sai só como resumo.
func GraphTag(version int64, publicationID string) string {
	if version == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(publicationID))
	return fmt.Sprintf("%d-%s", version, hex.EncodeToString(sum[:8]))
}
