package vaultgraph_test

import (
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaultgraph"
)

var wikilink = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// grafoSintetico monta o grafo das ligações do cofre sintético de 10 mil notas, com
// o caminho como id.
func grafoSintetico() *vaultgraph.Graph {
	notas := benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize)
	porTitulo := make(map[string]string, len(notas))
	ids := make([]string, 0, len(notas))
	for _, n := range notas {
		porTitulo[strings.TrimSuffix(path.Base(n.Path), ".md")] = n.Path
		ids = append(ids, n.Path)
	}
	var edges []vaultgraph.Edge
	for _, n := range notas {
		for _, m := range wikilink.FindAllStringSubmatch(n.Content, -1) {
			edges = append(edges, vaultgraph.Edge{Source: n.Path, Target: porTitulo[m[1]]})
		}
	}
	return vaultgraph.New(ids, edges)
}

func BenchmarkPageRank_CofreSintetico(b *testing.B) {
	g := grafoSintetico()
	b.ReportAllocs()
	for b.Loop() {
		g.PageRank()
	}
}

func BenchmarkCommunities_CofreSintetico(b *testing.B) {
	g := grafoSintetico()
	b.ReportAllocs()
	for b.Loop() {
		g.Communities()
	}
}
