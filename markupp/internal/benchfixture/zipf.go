package benchfixture

import (
	"math/rand/v2"
	"strings"
	"time"
)

const (
	// zipfExponent é o expoente da lei de Zipf das palavras e das ligações; a
	// distribuição da biblioteca padrão pede um valor acima de 1.
	zipfExponent      = 1.07
	zipfVocabularyLen = 20000
	maxTitleWords     = 4
	dailyNotePercent  = 15
	dailyNoteSpanDays = 3650
)

// zipfHead são as palavras mais frequentes do cofre de Zipf, em ordem de posto.
var zipfHead = strings.Fields(`nota projeto reuniao cliente prazo entrega relatorio
	orcamento equipe servidor banco indice busca grafo ligacao cofre tarefa revisao
	codigo teste deploy cluster fila evento usuario conta senha convite contrato
	ideia livro capitulo leitura resumo artigo pesquisa dado modelo metrica painel
	alerta incidente plano meta semana mes ano viagem receita compra mercado saude
	treino corrida sono humor familia amigo escola curso aula prova historia musica
	filme serie jogo foto jardim casa`)

var syllables = buildSyllables("bcdfglmnprstv", "aeiou")

var zipfVocabulary = buildZipfVocabulary()

var dailyNoteStart = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

// GenerateVault gera o cofre padrão dos benchmarks: palavras de um vocabulário de 20
// mil com frequência de Zipf, títulos de uma a quatro palavras ou de data, e
// ligações de Zipf, em que poucas notas recebem muitas. A mesma semente gera o mesmo
// cofre.
func GenerateVault(seed uint64, size int) []Note {
	rng := newRand(seed)
	words := newZipf(rng, len(zipfVocabulary))
	targets := newZipf(rng, size)
	s := sampler{
		word:   func() string { return zipfVocabulary[words.Uint64()] },
		target: func() int { return int(targets.Uint64()) }, //nolint:gosec // G115: o indice fica abaixo de size
	}
	return generate(rng, zipfTitles(rng, size, s.word), s)
}

// newZipf sorteia postos de 0 a n-1 com a lei de Zipf do cofre; o posto 0 é o mais
// frequente.
func newZipf(rng *rand.Rand, n int) *rand.Zipf {
	return rand.NewZipf(rng, zipfExponent, 1, uint64(max(n-1, 0)))
}

// zipfTitles sorteia títulos até ter size deles sem repetição, sem diferença por
// caixa, como o cofre compara títulos.
func zipfTitles(rng *rand.Rand, size int, word func() string) []string {
	seen := make(map[string]bool, size)
	titles := make([]string, 0, size)
	for len(titles) < size {
		title := zipfTitle(rng, word)
		if key := strings.ToLower(title); !seen[key] {
			seen[key] = true
			titles = append(titles, title)
		}
	}
	return titles
}

// zipfTitle sorteia uma nota diária com a data como título ou uma a quatro palavras
// com a primeira maiúscula.
func zipfTitle(rng *rand.Rand, word func() string) string {
	if rng.IntN(100) < dailyNotePercent {
		return dailyNoteStart.AddDate(0, 0, rng.IntN(dailyNoteSpanDays)).Format(time.DateOnly)
	}
	words := make([]string, 1+rng.IntN(maxTitleWords))
	for i := range words {
		words[i] = word()
	}
	return strings.ToUpper(words[0][:1]) + strings.Join(words, " ")[1:]
}

// vocabularySeed embaralha as palavras inventadas, para o posto não seguir a ordem
// alfabética das sílabas; é fixa, então o vocabulário não depende da semente do cofre.
const vocabularySeed = 1

// buildZipfVocabulary completa as palavras reais com palavras inventadas, sem repetir,
// até o tamanho do vocabulário.
func buildZipfVocabulary() []string {
	seen := make(map[string]bool, zipfVocabularyLen)
	vocabulary := make([]string, 0, zipfVocabularyLen)
	for _, w := range zipfHead {
		seen[w] = true
		vocabulary = append(vocabulary, w)
	}
	for n := 0; len(vocabulary) < zipfVocabularyLen; n++ {
		if w := inventedWord(n); !seen[w] {
			seen[w] = true
			vocabulary = append(vocabulary, w)
		}
	}
	invented := vocabulary[len(zipfHead):]
	newRand(vocabularySeed).Shuffle(len(invented), func(i, j int) { invented[i], invented[j] = invented[j], invented[i] })
	return vocabulary
}

// inventedWord devolve a n-ésima palavra de sílabas: primeiro as de duas, depois as
// de três, e assim por diante.
func inventedWord(n int) string {
	count, span := 2, len(syllables)*len(syllables)
	for n >= span {
		n -= span
		count++
		span *= len(syllables)
	}
	parts := make([]string, count)
	for i := count - 1; i >= 0; i-- {
		parts[i] = syllables[n%len(syllables)]
		n /= len(syllables)
	}
	return strings.Join(parts, "")
}

func buildSyllables(consonants, vowels string) []string {
	out := make([]string, 0, len(consonants)*len(vowels))
	for _, c := range consonants {
		for _, v := range vowels {
			out = append(out, string(c)+string(v))
		}
	}
	return out
}
