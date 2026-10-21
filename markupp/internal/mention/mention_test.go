package mention_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/mention"
)

func TestFold_TiraCaixaEAcento(t *testing.T) {
	assert.Equal(t, "indexacao deterministica", mention.Fold("Indexação DETERMINÍSTICA"))
	assert.Equal(t, mention.Fold("é"), mention.Fold("é"), "forma Unicode nao importa")
}

func TestEligible_PedeTresLetras(t *testing.T) {
	assert.False(t, mention.Eligible("Go"))
	assert.False(t, mention.Eligible("C++"), "simbolo nao conta como letra")
	assert.False(t, mention.Eligible("k8s"), "digito nao conta como letra")
	assert.True(t, mention.Eligible("API"))
	assert.True(t, mention.Eligible("Ação"))
}

func TestMatcher_TituloInteiroComFronteiraDePalavra(t *testing.T) {
	m := mention.NewMatcher([]string{"kubernetes", "plano de registro", "indexação", "registro"})

	assert.ElementsMatch(t, []string{"kubernetes", "plano de registro", "registro"},
		m.Find("Subimos o Kubernetes; o Plano de Registro ficou."), "sem caixa e com titulo de varias palavras")
	assert.Empty(t, m.Find("kubernetesx e subkubernetes"), "parte de palavra nao e mencao")
	assert.Equal(t, []string{"indexação"}, m.Find("sobre a INDEXACAO"), "sem acento, devolve o titulo como recebido")
}

func TestMatcher_IgnoraTextoDeLigacao(t *testing.T) {
	m := mention.NewMatcher([]string{"kubernetes"})

	assert.Empty(t, m.Find("[[Kubernetes Deploy]] ![[kubernetes.png]] [o kubernetes](k.md)"))
	assert.Equal(t, []string{"kubernetes"}, m.Find("[[Outra]] e kubernetes"))
}

func TestMatcher_TituloRepetidoSaiUmaVez(t *testing.T) {
	m := mention.NewMatcher([]string{"Nota", "nota"})

	assert.ElementsMatch(t, []string{"Nota", "nota"}, m.Find("nota, nota e NOTA"))
}

func TestPrior_SuavizacaoDeLaplace(t *testing.T) {
	assert.InDelta(t, 3.0/12.0, mention.Prior(2, 10, 2), 1e-9, "(ligacoes + 1) / (ocorrencias + candidatas)")
	assert.InDelta(t, 1.0, mention.Prior(0, 0, 1), 1e-9, "titulo sem ocorrencia nem ligacao, uma candidata")
}
