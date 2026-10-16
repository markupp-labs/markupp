package markdown

import (
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterFence = "---"

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

// parseAliases aceita aliases como texto único ou lista; item que não é texto,
// como número, vale pela forma escrita. YAML inválido não tem aliases.
func parseAliases(meta string) []string {
	var fields struct {
		Aliases yaml.Node `yaml:"aliases"`
	}
	if meta == "" || yaml.Unmarshal([]byte(meta), &fields) != nil {
		return nil
	}
	if fields.Aliases.Kind == yaml.ScalarNode {
		return nonEmpty([]*yaml.Node{&fields.Aliases})
	}
	return nonEmpty(fields.Aliases.Content)
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
