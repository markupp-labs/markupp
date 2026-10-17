package indexing

import (
	"net/url"
	"path"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/markdown"
)

// LinkKind é o tipo da ligação; o banco já aceita os tipos typed e inferred, que
// vêm do frontmatter e do segundo estágio.
type LinkKind string

// Tipos de ligação que o lote produz.
const (
	ExplicitLink LinkKind = "explicit"
	EmbedLink    LinkKind = "embed"
)

// Link é uma ligação que a nota faz, ainda sem destino: ela liga a todas as notas
// do cofre cujo título tem a chave TitleKey. TargetPath é o caminho que o link
// markdown aponta; quando há nota nesse caminho, a ligação vai só para ela.
type Link struct {
	CitedTitle string
	TitleKey   string
	TargetPath string
	Kind       LinkKind
}

// TitleKey é a forma do título usada para casar: sem diferença de caixa (case fold
// completo) nem de forma Unicode (NFC), mantendo os acentos.
func TitleKey(title string) string {
	return norm.NFC.String(cases.Fold().String(norm.NFD.String(strings.TrimSpace(title))))
}

func titleKeys(title string, aliases []string) []string {
	keys := make([]string, 0, 1+len(aliases))
	seen := make(map[string]bool, 1+len(aliases))
	for _, t := range append([]string{title}, aliases...) {
		key := TitleKey(t)
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

// extractLinks converte as ligações escritas na nota de notePath. A mesma ligação
// escrita de novo, ainda que em outra caixa, conta uma vez, com a primeira grafia.
func extractLinks(notePath string, written []markdown.Link) []Link {
	var links []Link
	seen := make(map[Link]bool, len(written))
	for _, w := range written {
		link, ok := toLink(notePath, w)
		key := Link{TitleKey: link.TitleKey, TargetPath: link.TargetPath, Kind: link.Kind}
		if ok && !seen[key] {
			seen[key] = true
			links = append(links, link)
		}
	}
	return links
}

func toLink(notePath string, w markdown.Link) (Link, bool) {
	switch w.Form {
	case markdown.MarkdownLink:
		return markdownLink(notePath, w.Target)
	case markdown.Embed:
		return citedLink(w.Target, "", EmbedLink)
	}
	return citedLink(w.Target, "", ExplicitLink)
}

// citedLink cita o título do último segmento de target, como o wikilink com pasta
// ou com .md que o editor aceita.
func citedLink(target, targetPath string, kind LinkKind) (Link, bool) {
	title := Title(strings.TrimSpace(target))
	link := Link{CitedTitle: title, TitleKey: TitleKey(title), TargetPath: targetPath, Kind: kind}
	return link, link.TitleKey != ""
}

// markdownLink resolve o destino contra a pasta da nota, ou contra a raiz do cofre
// se começar com /. URL com esquema ou host e alvo que não é .md ficam de fora.
func markdownLink(notePath, destination string) (Link, bool) {
	u, err := url.Parse(destination)
	if err != nil || u.Scheme != "" || u.Host != "" || !strings.EqualFold(path.Ext(u.Path), ".md") {
		return Link{}, false
	}
	target := u.Path
	if !strings.HasPrefix(target, "/") {
		target = path.Join(path.Dir(notePath), target)
	}
	target = strings.TrimPrefix(path.Clean("/"+target), "/")
	return citedLink(target, target, ExplicitLink)
}
