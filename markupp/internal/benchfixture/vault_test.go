package benchfixture_test

import (
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
)

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// generators são os dois cofres sintéticos: o de Zipf, padrão, e o uniforme, pior caso.
var generators = map[string]func(seed uint64, size int) []benchfixture.Note{
	"zipf":     benchfixture.GenerateVault,
	"uniforme": benchfixture.GenerateUniformVault,
}

func TestGenerateVault_MesmaSemente_GeraOMesmoCofre(t *testing.T) {
	for nome, gerar := range generators {
		assert.Equal(t, gerar(benchfixture.VaultSeed, 500), gerar(benchfixture.VaultSeed, 500), nome)
	}
}

func TestGenerateVault_SementesDiferentes_GeramCofresDiferentes(t *testing.T) {
	for nome, gerar := range generators {
		assert.NotEqual(t, gerar(1, 50), gerar(2, 50), nome)
	}
}

func TestGenerateVault_TamanhoPadrao_GeraDezMilNotasComTitulosUnicos(t *testing.T) {
	for nome, gerar := range generators {
		vault := gerar(benchfixture.VaultSeed, benchfixture.VaultSize)

		require.Len(t, vault, 10000, nome)
		assert.Len(t, titleKeysOf(vault), 10000, "%s: cada nota tem titulo proprio, sem diferenca por caixa", nome)
	}
}

func TestGenerateVault_Wikilinks_ApontamParaTitulosDoCofre(t *testing.T) {
	for nome, gerar := range generators {
		vault := gerar(benchfixture.VaultSeed, 1000)
		titles := titlesOf(vault)
		links := 0
		for _, note := range vault {
			for _, match := range wikilinkPattern.FindAllStringSubmatch(note.Content, -1) {
				links++
				assert.True(t, titles[match[1]], "%s: wikilink [[%s]] da nota %q sem nota de destino", nome, match[1], note.Path)
			}
		}
		assert.Greater(t, links, len(vault), "%s: o cofre precisa de mais ligacoes que notas para servir de grafo", nome)
	}
}

func TestGenerateVault_Nota_TemTituloNoCabecalhoETexto(t *testing.T) {
	for nome, gerar := range generators {
		note := gerar(benchfixture.VaultSeed, 1)[0]

		title := strings.TrimSuffix(path.Base(note.Path), ".md")
		assert.True(t, strings.HasPrefix(note.Content, "# "+title+"\n"), "%s: conteudo %q deveria abrir com o titulo", nome, note.Content)
		assert.Greater(t, len(note.Content), 500, "%s: a nota precisa de texto para a busca ter o que varrer", nome)
	}
}

func TestGenerateVault_Zipf_TitulosVariamSemOPadraoDoUniforme(t *testing.T) {
	padraoUniforme := regexp.MustCompile(`^\S+ \S+ \d{5}$`)
	tamanhos := map[int]int{}
	for title := range titlesOf(benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize)) {
		assert.False(t, padraoUniforme.MatchString(title), "titulo %q no padrao 'palavra palavra NNNNN'", title)
		tamanhos[len(strings.Fields(title))]++
	}
	assert.GreaterOrEqual(t, len(tamanhos), 3, "titulos com quantidades de palavras variadas, veio %v", tamanhos)
}

func TestGenerateVault_Zipf_FrequenciaDasPalavrasSegueZipf(t *testing.T) {
	ranking := wordRanking(benchfixture.GenerateVault(benchfixture.VaultSeed, 2000))

	assert.Greater(t, len(ranking), 5000, "vocabulario grande, vieram %d palavras distintas", len(ranking))
	razao := float64(ranking[0]) / float64(ranking[9])
	assert.InDelta(t, 10, razao, 4, "com expoente perto de 1 a 1a palavra aparece ~10 vezes mais que a 10a, veio %.1f", razao)
	razao = float64(ranking[9]) / float64(ranking[99])
	assert.InDelta(t, 10, razao, 4, "a 10a aparece ~10 vezes mais que a 100a, veio %.1f", razao)
}

func TestGenerateUniformVault_TitulosSeguemOPadraoDePiorCaso(t *testing.T) {
	note := benchfixture.GenerateUniformVault(benchfixture.VaultSeed, 43)[42]

	assert.Regexp(t, `/\S+ \S+ 00042\.md$`, note.Path)
}

// wordRanking conta as palavras do texto, fora cabeçalhos e wikilinks, e devolve as
// contagens da mais frequente para a menos.
func wordRanking(vault []benchfixture.Note) []int {
	counts := map[string]int{}
	for _, note := range vault {
		_, body, _ := strings.Cut(note.Content, "\n")
		for _, word := range strings.Fields(wikilinkPattern.ReplaceAllString(body, " ")) {
			counts[word]++
		}
	}
	ranking := make([]int, 0, len(counts))
	for _, c := range counts {
		ranking = append(ranking, c)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ranking)))
	return ranking
}

// titlesOf devolve o conjunto de títulos (último segmento do caminho sem extensão).
func titlesOf(vault []benchfixture.Note) map[string]bool {
	titles := make(map[string]bool, len(vault))
	for _, note := range vault {
		titles[strings.TrimSuffix(path.Base(note.Path), ".md")] = true
	}
	return titles
}

// titleKeysOf devolve os títulos sem caixa, como o cofre os compara.
func titleKeysOf(vault []benchfixture.Note) map[string]bool {
	keys := make(map[string]bool, len(vault))
	for title := range titlesOf(vault) {
		keys[strings.ToLower(title)] = true
	}
	return keys
}
