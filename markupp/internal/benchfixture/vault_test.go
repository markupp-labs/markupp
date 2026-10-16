package benchfixture_test

import (
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
)

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

func TestGenerateVault_MesmaSemente_GeraOMesmoCofre(t *testing.T) {
	primeiro := benchfixture.GenerateVault(benchfixture.VaultSeed, 500)
	segundo := benchfixture.GenerateVault(benchfixture.VaultSeed, 500)

	assert.Equal(t, primeiro, segundo)
}

func TestGenerateVault_SementesDiferentes_GeramCofresDiferentes(t *testing.T) {
	primeiro := benchfixture.GenerateVault(1, 50)
	segundo := benchfixture.GenerateVault(2, 50)

	assert.NotEqual(t, primeiro, segundo)
}

func TestGenerateVault_TamanhoPadrao_GeraDezMilNotasComCaminhosUnicos(t *testing.T) {
	vault := benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize)

	require.Len(t, vault, 10000)
	assert.Len(t, titlesOf(vault), 10000, "cada nota tem caminho e titulo proprios")
}

func TestGenerateVault_Wikilinks_ApontamParaTitulosDoCofre(t *testing.T) {
	vault := benchfixture.GenerateVault(benchfixture.VaultSeed, 1000)
	titles := titlesOf(vault)

	links := 0
	for _, note := range vault {
		for _, match := range wikilinkPattern.FindAllStringSubmatch(note.Content, -1) {
			links++
			assert.True(t, titles[match[1]], "wikilink [[%s]] da nota %q sem nota de destino", match[1], note.Path)
		}
	}
	assert.Greater(t, links, len(vault), "o cofre precisa de mais ligacoes que notas para servir de grafo")
}

func TestGenerateVault_Nota_TemTituloNoCabecalhoETexto(t *testing.T) {
	note := benchfixture.GenerateVault(benchfixture.VaultSeed, 1)[0]

	title := strings.TrimSuffix(path.Base(note.Path), ".md")
	assert.True(t, strings.HasPrefix(note.Content, "# "+title+"\n"), "conteudo %q deveria abrir com o titulo", note.Content)
	assert.Greater(t, len(note.Content), 500, "a nota precisa de texto para a busca ter o que varrer")
}

// titlesOf devolve o conjunto de títulos (último segmento do caminho sem extensão).
func titlesOf(vault []benchfixture.Note) map[string]bool {
	titles := make(map[string]bool, len(vault))
	for _, note := range vault {
		titles[strings.TrimSuffix(path.Base(note.Path), ".md")] = true
	}
	return titles
}
