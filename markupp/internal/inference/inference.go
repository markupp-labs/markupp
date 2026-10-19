// Package inference escolhe a ligação inferida de uma ligação não resolvida: entre
// as notas cujo título está perto do título citado, a de melhor fusão por RRF de
// três sinais, distância de edição, probabilidade a priori da âncora e parentesco
// no grafo.
package inference

import (
	"cmp"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/mention"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/suggestion"
)

// editRatioDenominator dá o limiar da distância de edição normalizada, 1/5 = 0,2,
// em conta inteira para o limite não depender de arredondamento.
const editRatioDenominator = 5

// Title é uma chave de título de uma nota do cofre.
type Title struct {
	NoteID string
	Key    string
}

// Candidate é a nota que pode ser o destino da ligação não resolvida: o título dela
// mais perto do citado, a distância normalizada até ele e os outros dois sinais,
// que quem chama preenche.
type Candidate struct {
	NoteID      string
	TitleKey    string
	Distance    float64
	Prior       float64
	Relatedness float64
}

// TitleIndex guarda os títulos do cofre já sem caixa e sem acento, agrupados pelo
// número de runas, para comparar só os de tamanho compatível. Reaproveita a tabela
// da distância entre chamadas, então não serve a duas goroutines ao mesmo tempo.
type TitleIndex struct {
	byLength map[int][]foldedTitle
	scratch  []int
}

type foldedTitle struct {
	Title
	runes []rune
}

// NewTitleIndex prepara os títulos do cofre para Candidates.
func NewTitleIndex(titles []Title) *TitleIndex {
	idx := &TitleIndex{byLength: make(map[int][]foldedTitle)}
	for _, t := range titles {
		runes := []rune(mention.Fold(t.Key))
		idx.byLength[len(runes)] = append(idx.byLength[len(runes)], foldedTitle{Title: t, runes: runes})
	}
	return idx
}

// Candidates devolve as notas, fora sourceID, com algum título a no máximo 0,2 de
// distância de edição do título citado, normalizada pelo tamanho do maior dos dois,
// sem caixa e sem acento. Cada nota vem uma vez, pelo título mais perto, e a lista
// sai por distância e id.
func (idx *TitleIndex) Candidates(sourceID, citedKey string) []Candidate {
	cited := []rune(mention.Fold(citedKey))
	best := make(map[string]Candidate)
	for length, titles := range idx.byLength {
		longest := max(length, len(cited))
		if abs(length-len(cited))*editRatioDenominator > longest {
			continue
		}
		for _, t := range titles {
			idx.keepCloser(best, sourceID, t, cited, longest)
		}
	}
	return sortedCandidates(best)
}

// keepCloser guarda t em best se ele estiver no limiar e for o título mais perto da
// nota dele até aqui.
func (idx *TitleIndex) keepCloser(best map[string]Candidate, sourceID string, t foldedTitle, cited []rune, longest int) {
	if t.NoteID == sourceID || longest == 0 {
		return
	}
	d := distance(cited, t.runes, longest/editRatioDenominator, &idx.scratch)
	if d*editRatioDenominator > longest {
		return
	}
	c := Candidate{NoteID: t.NoteID, TitleKey: t.Key, Distance: float64(d) / float64(longest)}
	if old, ok := best[t.NoteID]; !ok || closer(c, old) {
		best[t.NoteID] = c
	}
}

func closer(a, b Candidate) bool {
	return cmp.Or(cmp.Compare(a.Distance, b.Distance), cmp.Compare(a.TitleKey, b.TitleKey)) < 0
}

func sortedCandidates(best map[string]Candidate) []Candidate {
	out := make([]Candidate, 0, len(best))
	for _, c := range best {
		out = append(out, c)
	}
	slices.SortFunc(out, byDistance)
	return out
}

func byDistance(a, b Candidate) int {
	return cmp.Or(cmp.Compare(a.Distance, b.Distance), cmp.Compare(a.NoteID, b.NoteID))
}

// Choose devolve a candidata de maior RRF (k=60) entre três listas: por distância
// crescente, por probabilidade a priori decrescente e por parentesco decrescente.
// Empate num sinal ordena pela distância e depois pelo id, e o mesmo desempate vale
// para a pontuação fundida. Sem candidata, devolve false.
func Choose(candidates []Candidate) (Candidate, bool) {
	if len(candidates) == 0 {
		return Candidate{}, false
	}
	fused := suggestion.Fuse(
		rankBy(candidates, func(c Candidate) float64 { return -c.Distance }),
		rankBy(candidates, func(c Candidate) float64 { return c.Prior }),
		rankBy(candidates, func(c Candidate) float64 { return c.Relatedness }),
	)
	return slices.MinFunc(candidates, func(a, b Candidate) int {
		return cmp.Or(cmp.Compare(fused[b.NoteID], fused[a.NoteID]), byDistance(a, b))
	}), true
}

// rankBy devolve os ids das candidatas pelo sinal decrescente.
func rankBy(candidates []Candidate, signal func(Candidate) float64) []string {
	sorted := slices.Clone(candidates)
	slices.SortFunc(sorted, func(a, b Candidate) int {
		return cmp.Or(cmp.Compare(signal(b), signal(a)), byDistance(a, b))
	})
	ids := make([]string, len(sorted))
	for i, c := range sorted {
		ids[i] = c.NoteID
	}
	return ids
}
