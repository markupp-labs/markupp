package inference_test

import (
	"path"
	"strings"
	"testing"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/inference"
)

// BenchmarkCandidates_CofreSintetico mede a busca de candidatas para 500 títulos
// citados com a primeira letra trocada contra os 10 mil títulos do cofre sintético,
// como o segundo estágio faria.
func BenchmarkCandidates_CofreSintetico(b *testing.B) {
	benchCandidates(b, benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize))
}

// BenchmarkCandidates_CofreUniforme mede o mesmo no cofre uniforme, o pior caso para
// o descarte pela contagem de letras: todos os títulos têm o mesmo formato.
func BenchmarkCandidates_CofreUniforme(b *testing.B) {
	benchCandidates(b, benchfixture.GenerateUniformVault(benchfixture.VaultSeed, benchfixture.VaultSize))
}

func benchCandidates(b *testing.B, notas []benchfixture.Note) {
	titulos := make([]inference.Title, 0, len(notas))
	for _, n := range notas {
		titulos = append(titulos, inference.Title{NoteID: n.Path, Key: strings.ToLower(strings.TrimSuffix(path.Base(n.Path), ".md"))})
	}
	idx := inference.NewTitleIndex(titulos)
	b.ReportAllocs()
	for b.Loop() {
		for _, t := range titulos[1:501] {
			idx.Candidates("", "x"+t.Key[1:])
		}
	}
}
