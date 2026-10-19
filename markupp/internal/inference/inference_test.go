package inference_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/inference"
)

func TestDistance_DamerauLevenshteinComTransposicao(t *testing.T) {
	casos := map[string]struct {
		a, b string
		want int
	}{
		"iguais":                  {"kubernetes", "kubernetes", 0},
		"troca":                   {"kubernetes", "kubernetis", 1},
		"insercao":                {"kubernetes", "kubernetess", 1},
		"remocao":                 {"kubernetes", "kubrnetes", 1},
		"transposicao adjacente":  {"kubernetes", "kubrenetes", 1},
		"vazio":                   {"", "abc", 3},
		"runas e nao bytes":       {"ação", "acao", 2},
		"acima do limite satura":  {"abcdef", "uvwxyz", 4},
		"limite exato nao satura": {"abc", "xyz", 3},
	}
	for nome, caso := range casos {
		assert.Equal(t, caso.want, inference.Distance([]rune(caso.a), []rune(caso.b), 3), nome)
	}
}

var titulosDoCofre = []inference.Title{
	{NoteID: "k", Key: "kubernetes"},
	{NoteID: "k2", Key: "kubermetis"},
	{NoteID: "p", Key: "postgres"},
	{NoteID: "p", Key: "postgresql"},
	{NoteID: "c", Key: "ação"},
	{NoteID: "o", Key: "kubernetes"},
}

func TestCandidates_SoAteUmQuintoDoTamanhoSemCaixaNemAcento(t *testing.T) {
	idx := inference.NewTitleIndex(titulosDoCofre)

	assert.Equal(t, []inference.Candidate{
		{NoteID: "k", TitleKey: "kubernetes", Distance: 0.1},
		{NoteID: "k2", TitleKey: "kubermetis", Distance: 0.2},
	}, idx.Candidates("o", "kubernets"), "a propria nota fica de fora; ordem por distancia")
	assert.Equal(t, []inference.Candidate{{NoteID: "c", TitleKey: "ação", Distance: 0}}, idx.Candidates("x", "ACAO"))
	assert.Equal(t, []inference.Candidate{{NoteID: "p", TitleKey: "postgresql", Distance: 0.1}}, idx.Candidates("x", "postgresq"),
		"a nota entra uma vez, pelo titulo mais proximo")
	assert.Empty(t, idx.Candidates("x", "kub"), "distancia acima de 0,2 do tamanho do titulo")
}

func TestChoose_FundeDistanciaProbabilidadeEParentescoPorRRF(t *testing.T) {
	escolhida, ok := inference.Choose([]inference.Candidate{
		{NoteID: "perto", Distance: 0.1, Prior: 0.1, Relatedness: 0},
		{NoteID: "ligada", Distance: 0.2, Prior: 0.5, Relatedness: 1.5},
	})

	assert.True(t, ok)
	assert.Equal(t, "ligada", escolhida.NoteID, "ganha em dois dos tres sinais")
}

func TestChoose_EmpateNoSinalOrdenaPelaDistanciaEDepoisPeloID(t *testing.T) {
	escolhida, _ := inference.Choose([]inference.Candidate{
		{NoteID: "a", Distance: 0.2, Prior: 0.5},
		{NoteID: "b", Distance: 0.1, Prior: 0.1},
	})
	assert.Equal(t, "b", escolhida.NoteID, "parentesco empatado em zero conta a favor da mais proxima")

	escolhida, _ = inference.Choose([]inference.Candidate{{NoteID: "z", Distance: 0.1}, {NoteID: "y", Distance: 0.1}})
	assert.Equal(t, "y", escolhida.NoteID)
}

func TestChoose_SemCandidata(t *testing.T) {
	_, ok := inference.Choose(nil)

	assert.False(t, ok)
}
