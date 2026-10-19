package inference

// Distance é a distância de Damerau-Levenshtein, na variante de alinhamento ótimo
// de cadeias (Damerau, 1964): inserção, remoção, troca e transposição de duas runas
// vizinhas custam 1. Passou de limit, para de calcular e devolve limit+1.
func Distance(a, b []rune, limit int) int {
	if abs(len(a)-len(b)) > limit {
		return limit + 1
	}
	prev2, prev, cur := make([]int, len(b)+1), make([]int, len(b)+1), make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		if fillRow(a, b, i, prev2, prev, cur) > limit {
			return limit + 1
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return min(prev[len(b)], limit+1)
}

// fillRow calcula a linha i da tabela a partir das duas anteriores e devolve o
// menor valor dela, que nenhuma linha seguinte consegue diminuir.
func fillRow(a, b []rune, i int, prev2, prev, cur []int) int {
	cur[0] = i
	lowest := i
	for j := 1; j <= len(b); j++ {
		cost := 1
		if a[i-1] == b[j-1] {
			cost = 0
		}
		cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
			cur[j] = min(cur[j], prev2[j-2]+1)
		}
		lowest = min(lowest, cur[j])
	}
	return lowest
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
