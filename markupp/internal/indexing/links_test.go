package indexing_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
)

func indexaUma(t *testing.T, notePath, content string) indexing.IndexedNote {
	t.Helper()
	indexed := indexing.Index(indexing.Changeset{TenantID: "t1", VaultID: "va", Notes: []indexing.NoteChange{
		{NoteID: "n1", Path: notePath, Content: content},
	}})
	require.Len(t, indexed, 1)
	return indexed[0]
}

func TestTitleKey_IgnoraCaixaEFormaUnicodeMasNaoAcento(t *testing.T) {
	assert.Equal(t, indexing.TitleKey("Café"), indexing.TitleKey("CAFÉ"), "caixa e forma decomposta casam")
	assert.Equal(t, indexing.TitleKey("Straße"), indexing.TitleKey("STRASSE"), "case fold completo")
	assert.NotEqual(t, indexing.TitleKey("Cafe"), indexing.TitleKey("Café"), "acento distingue titulos")
}

func TestIndex_Aliases_EntramNosTitulosENoDocumentoDeBusca(t *testing.T) {
	nota := indexaUma(t, "pasta/Kubernetes.md", "---\naliases: [K8s, kubernetes]\n---\ncorpo")

	assert.Equal(t, []string{"K8s", "kubernetes"}, nota.Search.Aliases)
	assert.Equal(t, []string{"kubernetes", "k8s"}, nota.TitleKeys, "um titulo por chave normalizada, sem repetir")
}

func TestIndex_Wikilinks_ViramLigacoesPeloTituloCitado(t *testing.T) {
	nota := indexaUma(t, "origem.md", "[[Alvo]] [[pasta/Outra Nota.md|x]] ![[foto.png]] [[ALVO#secao]] [[#so secao]]")

	assert.Equal(t, []indexing.Link{
		{CitedTitle: "Alvo", TitleKey: "alvo", Kind: indexing.ExplicitLink},
		{CitedTitle: "Outra Nota", TitleKey: "outra nota", Kind: indexing.ExplicitLink},
		{CitedTitle: "foto.png", TitleKey: "foto.png", Kind: indexing.EmbedLink},
	}, nota.Links, "ligacao repetida para o mesmo titulo conta uma vez")
}

func TestIndex_LinksMarkdown_ResolvemOCaminhoAPartirDaPastaDaNota(t *testing.T) {
	conteudo := "[a](irma.md) [b](../raiz.md) [c](/outra/Nota%20Longa.md#secao) [d](https://x.org/n.md) [e](foto.png) [f](sub/Doc.MD)"

	nota := indexaUma(t, "pasta/origem.md", conteudo)

	assert.Equal(t, []indexing.Link{
		{CitedTitle: "irma", TitleKey: "irma", TargetPath: "pasta/irma.md", Kind: indexing.ExplicitLink},
		{CitedTitle: "raiz", TitleKey: "raiz", TargetPath: "raiz.md", Kind: indexing.ExplicitLink},
		{CitedTitle: "Nota Longa", TitleKey: "nota longa", TargetPath: "outra/Nota Longa.md", Kind: indexing.ExplicitLink},
		{CitedTitle: "Doc", TitleKey: "doc", TargetPath: "pasta/sub/Doc.MD", Kind: indexing.ExplicitLink},
	}, nota.Links, "url externa e alvo que nao e .md ficam de fora")
}

func TestIndex_SemLigacoes_DevolveLinksVazio(t *testing.T) {
	assert.Empty(t, indexaUma(t, "a.md", "texto `[[codigo]]` sem ligacao").Links)
}
