package indexing_test

import (
	"fmt"
	"testing"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
)

// loteIncremental é o número de notas editadas entre duas execuções do Job, o mesmo
// do benchmark do indexador com banco.
const loteIncremental = 100

func BenchmarkIndex_LoteIncremental(b *testing.B) {
	benchIndex(b, loteIncremental)
}

func BenchmarkIndex_CofreInteiro(b *testing.B) {
	benchIndex(b, benchfixture.VaultSize)
}

// benchIndex mede só a função pura do lote (parser, títulos e ligações), sem banco,
// sobre as primeiras porLote notas do cofre sintético.
func benchIndex(b *testing.B, porLote int) {
	cs := changesetSintetico(porLote)
	b.ReportAllocs()
	for b.Loop() {
		indexing.Index(cs)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*porLote), "ns/nota")
}

func changesetSintetico(size int) indexing.Changeset {
	vault := benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize)[:size]
	cs := indexing.Changeset{TenantID: "t-bench", VaultID: "cofre-bench", Notes: make([]indexing.NoteChange, 0, size)}
	for i, note := range vault {
		cs.Notes = append(cs.Notes, indexing.NoteChange{NoteID: fmt.Sprintf("n-%05d", i), Path: note.Path, Content: note.Content})
	}
	return cs
}
