package indexing_test

import (
	"fmt"
	"path"
	"strings"
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

func BenchmarkIndex_CofreInteiroComFrontmatterTipado(b *testing.B) {
	benchChangeset(b, comFrontmatterTipado(changesetSintetico(benchfixture.VaultSize)))
}

// comFrontmatterTipado põe em cada nota um frontmatter com duas ligações tipadas para
// as notas seguintes e dois valores literais, o que o lote passa a ler neste passo.
func comFrontmatterTipado(cs indexing.Changeset) indexing.Changeset {
	for i := range cs.Notes {
		next := func(k int) string {
			return strings.TrimSuffix(path.Base(cs.Notes[(i+k)%len(cs.Notes)].Path), ".md")
		}
		cs.Notes[i].Content = fmt.Sprintf("---\nautor: \"[[%s]]\"\nrelacionadas: [\"[[%s]]\"]\ndata: 2026-10-19\ntags: [x, y]\n---\n%s",
			next(1), next(2), cs.Notes[i].Content)
	}
	return cs
}

// benchIndex mede só a função pura do lote (parser, títulos e ligações), sem banco,
// sobre as primeiras porLote notas do cofre sintético.
func benchIndex(b *testing.B, porLote int) {
	benchChangeset(b, changesetSintetico(porLote))
}

func benchChangeset(b *testing.B, cs indexing.Changeset) {
	b.ReportAllocs()
	for b.Loop() {
		indexing.Index(cs)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(cs.Notes)), "ns/nota")
}

func changesetSintetico(size int) indexing.Changeset {
	vault := benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize)[:size]
	cs := indexing.Changeset{TenantID: "t-bench", VaultID: "cofre-bench", Notes: make([]indexing.NoteChange, 0, size)}
	for i, note := range vault {
		cs.Notes = append(cs.Notes, indexing.NoteChange{NoteID: fmt.Sprintf("n-%05d", i), Path: note.Path, Content: note.Content})
	}
	return cs
}
