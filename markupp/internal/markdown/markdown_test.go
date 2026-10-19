package markdown_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/markdown"
)

func TestParse_Wikilinks_DevolveOAlvoSemApelidoNemAncora(t *testing.T) {
	doc := markdown.Parse("ver [[Alvo]], [[Outro|apelido]], [[Secao#Titulo]], [[Bloco^abc123]] e [[Misto#^abc]]")

	assert.Equal(t, []markdown.Link{
		{Target: "Alvo", Form: markdown.Wikilink},
		{Target: "Outro", Form: markdown.Wikilink},
		{Target: "Secao", Form: markdown.Wikilink},
		{Target: "Bloco", Form: markdown.Wikilink},
		{Target: "Misto", Form: markdown.Wikilink},
	}, doc.Links)
}

func TestParse_WikilinkSoDeSecao_NaoViraLigacao(t *testing.T) {
	assert.Empty(t, markdown.Parse("volta para [[#Introducao]] e [[#^bloco]]").Links)
}

func TestParse_Embeds_ContamComoLigacaoInclusiveAnexo(t *testing.T) {
	doc := markdown.Parse("![[Nota Embutida]]\n\n![[foto.png]]")

	assert.Equal(t, []markdown.Link{
		{Target: "Nota Embutida", Form: markdown.Embed},
		{Target: "foto.png", Form: markdown.Embed},
	}, doc.Links)
}

func TestParse_LinkMarkdown_DevolveODestinoCru(t *testing.T) {
	doc := markdown.Parse("[texto](pasta/Outra%20Nota.md) e [site](https://exemplo.com)")

	assert.Equal(t, []markdown.Link{
		{Target: "pasta/Outra%20Nota.md", Form: markdown.MarkdownLink},
		{Target: "https://exemplo.com", Form: markdown.MarkdownLink},
	}, doc.Links)
}

func TestParse_Codigo_NaoViraLigacao(t *testing.T) {
	conteudo := "texto `[[EmLinha]]` e\n\n```\n[[Cercado]]\n[x](cercado.md)\n```\n\n    [[Indentado]]\n"

	assert.Empty(t, markdown.Parse(conteudo).Links)
}

func TestParse_FrontmatterComListaDeAliases_DevolveOsAliases(t *testing.T) {
	doc := markdown.Parse("---\naliases:\n  - Apelido Um\n  - 2026\ntags: [x]\n---\ncorpo com [[Alvo]]\n")

	assert.Equal(t, []string{"Apelido Um", "2026"}, doc.Aliases)
	assert.Equal(t, []markdown.Link{{Target: "Alvo", Form: markdown.Wikilink}}, doc.Links)
}

func TestParse_FrontmatterComAliasUnico_DevolveOAlias(t *testing.T) {
	assert.Equal(t, []string{"Unico"}, markdown.Parse("---\naliases: Unico\n---\ncorpo").Aliases)
}

func TestParse_WikilinkNoFrontmatter_NaoEhLigacaoExplicita(t *testing.T) {
	assert.Empty(t, markdown.Parse("---\nrelacionada: \"[[Outra]]\"\n---\ncorpo").Links)
}

func TestParse_FrontmatterInvalido_IgnoraOsAliasesESegueComAsLigacoes(t *testing.T) {
	doc := markdown.Parse("---\naliases: [sem fechar\n---\n[[Alvo]]")

	assert.Empty(t, doc.Aliases)
	assert.Equal(t, []markdown.Link{{Target: "Alvo", Form: markdown.Wikilink}}, doc.Links)
}

func TestParse_SemFrontmatter_SemAliases(t *testing.T) {
	doc := markdown.Parse("---\nlinha horizontal sem fechar [[Alvo]]")

	assert.Empty(t, doc.Aliases)
	assert.Equal(t, []markdown.Link{{Target: "Alvo", Form: markdown.Wikilink}}, doc.Links)
}

func TestParse_FrontmatterComWikilinks_DevolveLigacoesTipadasPelaChave(t *testing.T) {
	conteudo := "---\nautor: \"[[Fulana]]\"\nprojetos:\n  - \"[[markupp|m]]\"\n  - \"[[Outro#Secao]]\"\n  - literal\n" +
		"tags: [adr, busca]\ndata: 2026-10-19\ntexto: \"ver [[A]] e [[B]]\"\nancora: \"[[#Secao]]\"\n---\ncorpo"

	doc := markdown.Parse(conteudo)

	assert.Equal(t, []markdown.TypedLink{
		{Predicate: "autor", Target: "Fulana"},
		{Predicate: "projetos", Target: "markupp"},
		{Predicate: "projetos", Target: "Outro"},
	}, doc.TypedLinks, "valor literal, texto com ligacao no meio e ancora sem nota nao viram ligacao tipada")
	assert.Empty(t, doc.Links)
}

func TestParse_AliasesComWikilink_NaoViraLigacaoTipada(t *testing.T) {
	doc := markdown.Parse("---\naliases: \"[[Apelido]]\"\n---\ncorpo")

	assert.Empty(t, doc.TypedLinks)
	assert.Equal(t, []string{"[[Apelido]]"}, doc.Aliases)
}

func TestParse_FrontmatterQueNaoEhMapa_SemLigacoesTipadas(t *testing.T) {
	assert.Empty(t, markdown.Parse("---\n- \"[[Alvo]]\"\n---\ncorpo").TypedLinks)
	assert.Empty(t, markdown.Parse("---\nrel: \"[[Alvo]]\n---\ncorpo").TypedLinks, "YAML invalido")
}
