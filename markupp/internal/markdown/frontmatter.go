package markdown

import (
	"strings"

	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

const (
	frontmatterFence = "---"
	aliasesKey       = "aliases"
)

// TypedLink é a ligação tipada escrita no frontmatter: a chave é o predicado e Target
// é o alvo do wikilink, sem apelido nem âncora.
type TypedLink struct {
	Predicate string
	Target    string
}

// splitFrontmatter separa o bloco YAML entre a cerca da primeira linha e a próxima
// linha só com a cerca. Sem a cerca de fechamento, não há frontmatter.
func splitFrontmatter(content string) (meta, body string) {
	first, rest, ok := strings.Cut(content, "\n")
	if !ok || strings.TrimRight(first, "\r") != frontmatterFence {
		return "", content
	}
	for offset := 0; offset < len(rest); {
		line, _, _ := strings.Cut(rest[offset:], "\n")
		end := min(offset+len(line)+1, len(rest))
		if strings.TrimRight(line, "\r") == frontmatterFence {
			return rest[:offset], rest[end:]
		}
		offset = end
	}
	return "", content
}

// frontmatterFields devolve chaves e valores do mapa YAML intercalados, na ordem
// escrita. YAML inválido ou que não é mapa não tem campos.
func frontmatterFields(meta string) []*yaml.Node {
	var doc yaml.Node
	if meta == "" || yaml.Unmarshal([]byte(meta), &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	if doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return doc.Content[0].Content
}

// parseAliases aceita aliases como texto único ou lista; item que não é texto,
// como número, vale pela forma escrita.
func parseAliases(fields []*yaml.Node) []string {
	for i := 0; i+1 < len(fields); i += 2 {
		if fields[i].Value == aliasesKey {
			return nonEmpty(scalarItems(fields[i+1]))
		}
	}
	return nil
}

// parseTypedLinks lê as chaves, salvo aliases, cujo valor é wikilink ou lista com
// wikilinks; valor literal não é ligação.
func parseTypedLinks(fields []*yaml.Node) []TypedLink {
	var links []TypedLink
	for i := 0; i+1 < len(fields); i += 2 {
		key := fields[i].Value
		if key == aliasesKey || strings.TrimSpace(key) == "" {
			continue
		}
		for _, item := range wikilinkItems(fields[i+1]) {
			if target, ok := wholeWikilink(item); ok {
				links = append(links, TypedLink{Predicate: key, Target: target})
			}
		}
	}
	return links
}

// wikilinkItems devolve o texto de cada wikilink candidato do valor: o próprio valor
// ou cada item da lista, escritos entre aspas ou sem elas.
func wikilinkItems(value *yaml.Node) []string {
	if text, ok := unquotedWikilink(value); ok {
		return []string{text}
	}
	var items []string
	for _, item := range listItems(value) {
		if text, ok := unquotedWikilink(item); ok {
			items = append(items, text)
		} else if item.Kind == yaml.ScalarNode {
			items = append(items, item.Value)
		}
	}
	return items
}

// unquotedWikilink reconhece [[X]] escrito sem aspas, que o YAML lê como uma lista
// com uma lista de um texto só, e devolve o wikilink reescrito.
func unquotedWikilink(value *yaml.Node) (string, bool) {
	if value.Kind != yaml.SequenceNode || len(value.Content) != 1 {
		return "", false
	}
	inner := value.Content[0]
	if inner.Kind != yaml.SequenceNode || len(inner.Content) != 1 || inner.Content[0].Kind != yaml.ScalarNode {
		return "", false
	}
	return "[[" + inner.Content[0].Value + "]]", true
}

// listItems devolve o próprio valor quando ele não é lista, ou os itens da lista.
func listItems(value *yaml.Node) []*yaml.Node {
	if value.Kind == yaml.SequenceNode {
		return value.Content
	}
	return []*yaml.Node{value}
}

// scalarItems devolve o próprio valor quando ele é escalar, ou os itens de uma lista;
// mapa aninhado não tem itens.
func scalarItems(value *yaml.Node) []*yaml.Node {
	if value.Kind == yaml.ScalarNode {
		return []*yaml.Node{value}
	}
	if value.Kind != yaml.SequenceNode {
		return nil
	}
	return value.Content
}

// wholeWikilink devolve o alvo quando value inteiro é um único wikilink, como o
// Obsidian grava entre aspas; texto com ligação no meio não conta.
func wholeWikilink(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[[") || !strings.HasSuffix(value, "]]") {
		return "", false
	}
	links := collectLinks(parser.Parse(text.NewReader([]byte(value))))
	if len(links) != 1 || links[0].Form != Wikilink {
		return "", false
	}
	return links[0].Target, true
}

func nonEmpty(nodes []*yaml.Node) []string {
	var aliases []string
	for _, node := range nodes {
		if alias := strings.TrimSpace(node.Value); node.Kind == yaml.ScalarNode && alias != "" {
			aliases = append(aliases, alias)
		}
	}
	return aliases
}
