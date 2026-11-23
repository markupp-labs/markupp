package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

func vizinha(nota, vizinha string, cosseno float64) gen.ListLexicalNeighborsRow {
	return gen.ListLexicalNeighborsRow{NoteID: nota, NeighborID: vizinha, Score: cosseno}
}

func TestMutualNeighbors_LigaSoOParEmQueCadaUmaEstaEntreAsVizinhasDaOutra(t *testing.T) {
	vizinhas := []gen.ListLexicalNeighborsRow{vizinha("a", "b", 0.8), vizinha("b", "a", 0.8), vizinha("a", "c", 0.5), vizinha("c", "d", 0.4)}

	ligacoes := mutualNeighbors(vizinhas, map[string]bool{"a": true, "b": true, "c": true, "d": true})

	assert.ElementsMatch(t, []relatedLink{{source: "a", target: "b", score: 0.8}, {source: "b", target: "a", score: 0.8}}, ligacoes,
		"c nao tem a entre as vizinhas, e d nao listou as dela")
}

func TestMutualNeighbors_FicaSoComOParQueTocaAsNotasDoLote(t *testing.T) {
	vizinhas := []gen.ListLexicalNeighborsRow{vizinha("a", "b", 0.8), vizinha("b", "a", 0.8), vizinha("b", "c", 0.6), vizinha("c", "b", 0.6)}

	ligacoes := mutualNeighbors(vizinhas, map[string]bool{"a": true})

	assert.ElementsMatch(t, []relatedLink{{source: "a", target: "b", score: 0.8}, {source: "b", target: "a", score: 0.8}}, ligacoes,
		"o par b e c fica como estava, fora do lote")
}
