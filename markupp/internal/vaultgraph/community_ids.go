package vaultgraph

// minInheritJaccard é a sobreposição mínima, por Jaccard, para a comunidade nova
// herdar o id da anterior.
const minInheritJaccard = 0.5

// InheritCommunityIDs devolve o id de cada comunidade de current: o da comunidade
// anterior com mais notas em comum, se o Jaccard entre as duas for pelo menos 0,5 e
// o id ainda não tiver sido herdado, ou zero para id novo. previous dá o id
// anterior de cada nota; as comunidades são visitadas na ordem de current.
func InheritCommunityIDs(current [][]string, previous map[string]int64) []int64 {
	sizes := make(map[int64]int)
	for _, id := range previous {
		sizes[id]++
	}
	taken := make(map[int64]bool)
	ids := make([]int64, len(current))
	for i, members := range current {
		id, overlap := largestOverlap(members, previous)
		union := len(members) + sizes[id] - overlap
		if id != 0 && !taken[id] && float64(overlap) >= minInheritJaccard*float64(union) {
			ids[i], taken[id] = id, true
		}
	}
	return ids
}

// largestOverlap devolve a comunidade anterior com mais notas de members e quantas
// são; no empate, a de menor id.
func largestOverlap(members []string, previous map[string]int64) (id int64, overlap int) {
	counts := make(map[int64]int)
	for _, m := range members {
		if old, ok := previous[m]; ok {
			counts[old]++
		}
	}
	for old, n := range counts {
		if n > overlap || (n == overlap && old < id) {
			id, overlap = old, n
		}
	}
	return id, overlap
}
