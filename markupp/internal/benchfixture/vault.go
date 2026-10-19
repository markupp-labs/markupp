// Package benchfixture gera o cofre sintético dos benchmarks de leveza: notas
// markdown com título, texto e wikilinks entre elas, sempre iguais para a mesma
// semente. O cofre padrão tem a frequência das palavras e das ligações em lei de
// Zipf, como um cofre real; o uniforme é o pior caso.
package benchfixture

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// VaultSeed é a semente fixa do cofre dos benchmarks.
const VaultSeed = 20261016

// VaultSize é o tamanho do cofre dos benchmarks.
const VaultSize = 10000

const (
	paragraphsPerNote = 3
	minWordsPerPara   = 40
	maxWordsPerPara   = 80
	maxLinksPerPara   = 3
	folderCount       = 20
)

// Note é uma nota do cofre sintético; o título é o caminho sem pasta nem extensão.
type Note struct {
	Path    string
	Content string
}

// sampler sorteia as palavras do texto e o índice da nota que cada wikilink cita.
type sampler struct {
	word   func() string
	target func() int
}

func newRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // G404: o cofre precisa ser reprodutivel, nao imprevisivel
}

func generate(rng *rand.Rand, titles []string, s sampler) []Note {
	vault := make([]Note, 0, len(titles))
	for i, title := range titles {
		path := fmt.Sprintf("pasta-%02d/%s.md", i%folderCount, title)
		vault = append(vault, Note{Path: path, Content: generateContent(rng, title, titles, s)})
	}
	return vault
}

func generateContent(rng *rand.Rand, title string, titles []string, s sampler) string {
	var b strings.Builder
	b.WriteString("# " + title + "\n")
	for range paragraphsPerNote {
		b.WriteString("\n" + generateParagraph(rng, titles, s) + "\n")
	}
	return b.String()
}

// generateParagraph sorteia palavras e troca algumas por wikilinks para títulos do
// cofre.
func generateParagraph(rng *rand.Rand, titles []string, s sampler) string {
	words := make([]string, minWordsPerPara+rng.IntN(maxWordsPerPara-minWordsPerPara+1))
	for i := range words {
		words[i] = s.word()
	}
	for range rng.IntN(maxLinksPerPara + 1) {
		words[rng.IntN(len(words))] = "[[" + titles[s.target()] + "]]"
	}
	return strings.Join(words, " ")
}
