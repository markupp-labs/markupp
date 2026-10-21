// Package suggestion funde as listas de sugestão de ligação de uma nota, cada uma
// ordenada por um sinal, por Reciprocal Rank Fusion (Cormack, Clarke e Büttcher, 2009).
package suggestion

// rrfK é a constante do RRF, o valor canônico do artigo.
const rrfK = 60

// Fuse devolve a pontuação RRF de cada id das listas: a soma de 1/(60 + posição), com
// a posição contada a partir de 1, em cada lista em que o id aparece.
func Fuse(rankings ...[]string) map[string]float64 {
	fused := make(map[string]float64)
	for _, ranking := range rankings {
		for i, id := range ranking {
			fused[id] += 1 / float64(rrfK+i+1)
		}
	}
	return fused
}
