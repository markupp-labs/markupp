// Package benchfixture gera o cofre sintético dos benchmarks de leveza: notas
// markdown com título, texto e wikilinks entre elas, sempre iguais para a mesma
// semente.
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

var vocabulary = strings.Fields(`reuniao cliente projeto prazo entrega relatorio
	orcamento equipe servidor banco indice busca grafo ligacao nota cofre tarefa
	revisao codigo teste deploy cluster fila evento usuario conta senha convite
	contrato reuniao ideia livro capitulo leitura resumo artigo pesquisa dado
	modelo metrica painel alerta incidente plano meta semana mes ano viagem
	receita compra mercado saude treino corrida sono humor familia amigo escola
	curso aula prova nota historia musica filme serie jogo foto jardim casa`)

// GenerateVault gera size notas a partir de seed; a mesma semente gera o mesmo cofre.
func GenerateVault(seed uint64, size int) []Note {
	rng := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // G404: o cofre precisa ser reprodutivel, nao imprevisivel
	titles := generateTitles(rng, size)
	vault := make([]Note, 0, size)
	for i, title := range titles {
		path := fmt.Sprintf("pasta-%02d/%s.md", i%folderCount, title)
		vault = append(vault, Note{Path: path, Content: generateContent(rng, title, titles)})
	}
	return vault
}

// generateTitles sorteia duas palavras por título; o índice no fim o torna único.
func generateTitles(rng *rand.Rand, size int) []string {
	titles := make([]string, size)
	for i := range titles {
		titles[i] = fmt.Sprintf("%s %s %05d", randomWord(rng), randomWord(rng), i)
	}
	return titles
}

func generateContent(rng *rand.Rand, title string, titles []string) string {
	var b strings.Builder
	b.WriteString("# " + title + "\n")
	for range paragraphsPerNote {
		b.WriteString("\n" + generateParagraph(rng, titles) + "\n")
	}
	return b.String()
}

// generateParagraph sorteia palavras do vocabulário e troca algumas por wikilinks
// para títulos do cofre.
func generateParagraph(rng *rand.Rand, titles []string) string {
	words := make([]string, minWordsPerPara+rng.IntN(maxWordsPerPara-minWordsPerPara+1))
	for i := range words {
		words[i] = randomWord(rng)
	}
	for range rng.IntN(maxLinksPerPara + 1) {
		words[rng.IntN(len(words))] = "[[" + titles[rng.IntN(len(titles))] + "]]"
	}
	return strings.Join(words, " ")
}

func randomWord(rng *rand.Rand) string {
	return vocabulary[rng.IntN(len(vocabulary))]
}
