// Package markdown lê o que o indexador precisa de uma nota: as ligações escritas no
// corpo e os aliases e as ligações tipadas do frontmatter. É a fronteira fina do
// projeto sobre goldmark, a extensão de wikilink e yaml.v3; nenhum tipo dessas
// bibliotecas sai daqui.
package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/wikilink"
)

// LinkForm é como a ligação foi escrita no corpo da nota.
type LinkForm int

// Formas de ligação reconhecidas no corpo.
const (
	Wikilink     LinkForm = iota // [[Alvo]]
	Embed                        // ![[Alvo]]
	MarkdownLink                 // [texto](destino)
)

// Link é uma ligação escrita no corpo. Target é o alvo do wikilink sem apelido nem
// âncora, ou o destino cru do link markdown, ainda sem decodificar nem filtrar.
type Link struct {
	Target string
	Form   LinkForm
}

// Document é o que a nota declara: os aliases e as ligações tipadas do frontmatter e
// as ligações do corpo, na ordem em que aparecem, fora de bloco e de trecho de código.
type Document struct {
	Aliases    []string
	TypedLinks []TypedLink
	Links      []Link
}

var parser = goldmark.New(goldmark.WithExtensions(&wikilink.Extender{})).Parser()

// Parse lê content. Frontmatter inválido não impede a leitura do corpo.
func Parse(content string) Document {
	meta, body := splitFrontmatter(content)
	fields := frontmatterFields(meta)
	return Document{
		Aliases:    parseAliases(fields),
		TypedLinks: parseTypedLinks(fields),
		Links:      collectLinks(parser.Parse(text.NewReader([]byte(body)))),
	}
}

// collectLinks percorre a árvore; código vira nó próprio, então nunca chega aqui.
func collectLinks(root ast.Node) []Link {
	var links []Link
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if link, ok := toLink(n); ok {
			links = append(links, link)
		}
		return ast.WalkContinue, nil
	})
	return links
}

func toLink(n ast.Node) (Link, bool) {
	switch node := n.(type) {
	case *wikilink.Node:
		return wikilinkTarget(node)
	case *ast.Link:
		return Link{Target: string(node.Destination), Form: MarkdownLink}, len(node.Destination) > 0
	}
	return Link{}, false
}

// wikilinkTarget descarta a âncora de bloco (^) que a extensão deixa no alvo; o
// wikilink só de seção ou de bloco fica sem alvo e não é ligação.
func wikilinkTarget(node *wikilink.Node) (Link, bool) {
	target, _, _ := strings.Cut(string(node.Target), "^")
	form := Wikilink
	if node.Embed {
		form = Embed
	}
	return Link{Target: target, Form: form}, strings.TrimSpace(target) != ""
}
