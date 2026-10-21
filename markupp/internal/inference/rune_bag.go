package inference

// bagBuckets é o número de grupos em que as runas de um título são contadas.
const bagBuckets = 32

// runeBag conta as runas de um título por grupo (runa módulo 32). A diferença entre
// duas contagens é um limite inferior barato da distância de edição: cada inserção,
// remoção ou troca muda a sobra de um lado em no máximo 1, a transposição não muda
// nada, e juntar runas no mesmo grupo só diminui a diferença.
type runeBag [bagBuckets]uint16

func newRuneBag(runes []rune) runeBag {
	var b runeBag
	for _, r := range runes {
		b[r%bagBuckets]++
	}
	return b
}

// lowerBound devolve o maior entre o que sobra em b e o que sobra em other.
func (b runeBag) lowerBound(other runeBag) int {
	extra, missing := 0, 0
	for i := range b {
		diff := int(b[i]) - int(other[i])
		if diff > 0 {
			extra += diff
		} else {
			missing -= diff
		}
	}
	return max(extra, missing)
}
