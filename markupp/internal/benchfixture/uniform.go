package benchfixture

import (
	"fmt"
	"strings"
)

// uniformVocabulary é o vocabulário curto do cofre uniforme.
var uniformVocabulary = strings.Fields(`reuniao cliente projeto prazo entrega relatorio
	orcamento equipe servidor banco indice busca grafo ligacao nota cofre tarefa
	revisao codigo teste deploy cluster fila evento usuario conta senha convite
	contrato reuniao ideia livro capitulo leitura resumo artigo pesquisa dado
	modelo metrica painel alerta incidente plano meta semana mes ano viagem
	receita compra mercado saude treino corrida sono humor familia amigo escola
	curso aula prova nota historia musica filme serie jogo foto jardim casa`)

// GenerateUniformVault gera o cofre de pior caso: poucas palavras sorteadas com a
// mesma chance, títulos "palavra palavra NNNNN" e ligações para qualquer nota com a
// mesma chance. A mesma semente gera o mesmo cofre.
func GenerateUniformVault(seed uint64, size int) []Note {
	rng := newRand(seed)
	word := func() string { return uniformVocabulary[rng.IntN(len(uniformVocabulary))] }
	titles := uniformTitles(size, word)
	return generate(rng, titles, sampler{word: word, target: func() int { return rng.IntN(size) }})
}

// uniformTitles sorteia duas palavras por título; o índice no fim o torna único.
func uniformTitles(size int, word func() string) []string {
	titles := make([]string, size)
	for i := range titles {
		titles[i] = fmt.Sprintf("%s %s %05d", word(), word(), i)
	}
	return titles
}
