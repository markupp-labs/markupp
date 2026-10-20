package notes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

func ptrTo[T any](v T) *T { return &v }

func notaDoGrafo(id, notePath string, degree int) notes.GraphNote {
	return notes.GraphNote{NoteSummary: notes.Summarize(id, notePath), Rank: notes.NoteRank{Degree: &degree, PageRank: ptrTo(0.5), Community: ptrTo(int64(7))}}
}

func TestBuildGraph_LigacoesViramParesDeIndicesNosNos(t *testing.T) {
	graph := notes.BuildGraph(
		[]notes.GraphNote{notaDoGrafo("n1", "a.md", 1), notaDoGrafo("n2", "pasta/b.md", 1)},
		[]notes.GraphEdge{{SourceID: "n1", TargetID: "n2", Type: notes.LinkTyped, Predicate: ptrTo("autor")}},
		nil,
	)

	assert.Equal(t, []notes.GraphNode{
		{ID: ptrTo("n1"), Title: "a", Path: ptrTo("a.md"), Degree: ptrTo(1), PageRank: ptrTo(0.5), Community: ptrTo(int64(7))},
		{ID: ptrTo("n2"), Title: "b", Path: ptrTo("pasta/b.md"), Degree: ptrTo(1), PageRank: ptrTo(0.5), Community: ptrTo(int64(7))},
	}, graph.Nodes)
	assert.Equal(t, []notes.GraphLink{{Source: 0, Target: 1, Type: notes.LinkTyped, Predicate: ptrTo("autor")}}, graph.Links)
}

func TestBuildGraph_LigacaoComPontaForaDosNos_Sai(t *testing.T) {
	graph := notes.BuildGraph(
		[]notes.GraphNote{notaDoGrafo("n1", "a.md", 1)},
		[]notes.GraphEdge{{SourceID: "n1", TargetID: "fora", Type: notes.LinkExplicit}, {SourceID: "fora", TargetID: "n1", Type: notes.LinkInferred}},
		[]notes.GraphCitation{{NoteID: "fora", TitleKey: "x", CitedTitle: "X", Type: notes.LinkExplicit}},
	)

	assert.Len(t, graph.Nodes, 1, "a citacao de nota fora dos nos nao cria no sem nota")
	assert.Empty(t, graph.Links)
}

func TestBuildGraph_TituloNaoResolvido_UmNoSemNotaPorChave(t *testing.T) {
	graph := notes.BuildGraph(
		[]notes.GraphNote{notaDoGrafo("n1", "a.md", 0), notaDoGrafo("n2", "b.md", 0)},
		nil,
		[]notes.GraphCitation{
			{NoteID: "n1", TitleKey: "foto.png", CitedTitle: "Foto.png", Type: notes.LinkEmbed},
			{NoteID: "n2", TitleKey: "foto.png", CitedTitle: "foto.png", Type: notes.LinkExplicit},
			{NoteID: "n2", TitleKey: "ideia", CitedTitle: "Ideia", Type: notes.LinkTyped, Predicate: ptrTo("fonte")},
		},
	)

	assert.Equal(t, []notes.GraphNode{
		{Title: "Foto.png", Unresolved: true},
		{Title: "Ideia", Unresolved: true},
	}, graph.Nodes[2:], "o titulo e a primeira grafia citada")
	assert.Equal(t, []notes.GraphLink{
		{Source: 0, Target: 2, Type: notes.LinkEmbed},
		{Source: 1, Target: 2, Type: notes.LinkExplicit},
		{Source: 1, Target: 3, Type: notes.LinkTyped, Predicate: ptrTo("fonte")},
	}, graph.Links)
}

func TestBuildGraph_SemNos_ListasVaziasENaoNulas(t *testing.T) {
	graph := notes.BuildGraph(nil, nil, nil)

	assert.NotNil(t, graph.Nodes)
	assert.NotNil(t, graph.Links)
}
