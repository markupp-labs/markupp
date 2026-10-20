package storage

import (
	"context"
	"fmt"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/inference"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/mention"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaultgraph"
)

// vaultCitations são os títulos do cofre e o que as notas citam, lidos uma vez e
// cruzados em memória: a anti-junção no banco, sob RLS e sem estatística da carga
// recém-gravada, levava segundos num cofre de 10 mil notas.
type vaultCitations struct {
	titles     []inference.Title
	titleNotes map[string]int             // notas com cada chave de título
	citers     map[string]map[string]bool // notas que citam cada chave de título
	unresolved []gen.ListVaultCitationsRow
}

// titleOccurrence é quantas notas do cofre têm o título no corpo, ou common quando o
// título está em mais de max(5% delas, 5) e é palavra comum, sem prior (Mihalcea e
// Csomai, 2007).
type titleOccurrence struct {
	count  int
	common bool
}

// inferLinks grava uma ligação inferida para cada título sem nota citado no cofre,
// quando há candidata, e devolve as arestas gravadas. O grafo g é o das ligações
// resolvidas, sem inferidas, e dá o parentesco entre a nota que cita e a candidata.
func inferLinks(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, g *vaultgraph.Graph) ([]vaultgraph.Edge, error) {
	vc, err := loadCitations(ctx, q, vaultID)
	if err != nil || len(vc.unresolved) == 0 {
		return nil, err
	}
	candidates := vc.candidates()
	occurrences, err := countOccurrences(ctx, q, vaultID, searchConfig, candidates)
	if err != nil {
		return nil, err
	}
	params, edges := vc.chooseTargets(candidates, occurrences, g)
	if err := q.SaveInferredLinks(ctx, params); err != nil {
		return nil, fmt.Errorf("gravar as %d ligacoes inferidas do cofre %q: %w", len(edges), vaultID, err)
	}
	return edges, nil
}

// loadCitations lê os títulos e as citações do cofre e separa as citações de título
// sem nota, uma por nota e título, com a primeira grafia.
func loadCitations(ctx context.Context, q *gen.Queries, vaultID string) (vaultCitations, error) {
	titles, err := q.ListVaultTitleKeys(ctx, vaultID)
	if err != nil {
		return vaultCitations{}, fmt.Errorf("listar os titulos do cofre %q: %w", vaultID, err)
	}
	cited, err := q.ListVaultCitations(ctx, vaultID)
	if err != nil {
		return vaultCitations{}, fmt.Errorf("listar as ligacoes do cofre %q: %w", vaultID, err)
	}
	vc := vaultCitations{titleNotes: make(map[string]int), citers: make(map[string]map[string]bool)}
	for _, t := range titles {
		vc.titles = append(vc.titles, inference.Title{NoteID: t.NoteID, Key: t.TitleKey})
		vc.titleNotes[t.TitleKey]++
	}
	for _, c := range cited {
		vc.addCitation(c)
	}
	return vc, nil
}

// addCitation conta a nota que cita o título e guarda a citação se o título não é de
// nota nenhuma e a nota ainda não o tinha citado; as citações chegam por nota e
// título, então a repetida vem logo depois.
func (vc *vaultCitations) addCitation(c gen.ListVaultCitationsRow) {
	if vc.citers[c.TitleKey] == nil {
		vc.citers[c.TitleKey] = make(map[string]bool)
	}
	vc.citers[c.TitleKey][c.NoteID] = true
	last := len(vc.unresolved) - 1
	repeated := last >= 0 && vc.unresolved[last].NoteID == c.NoteID && vc.unresolved[last].TitleKey == c.TitleKey
	if vc.titleNotes[c.TitleKey] == 0 && !repeated {
		vc.unresolved = append(vc.unresolved, c)
	}
}

// candidates devolve, na ordem das citações sem nota, as notas com título perto do
// citado, fora a própria nota que cita. O mesmo título citado é comparado uma vez.
func (vc *vaultCitations) candidates() [][]inference.Candidate {
	idx := inference.NewTitleIndex(vc.titles)
	byKey := make(map[string][]inference.Candidate)
	out := make([][]inference.Candidate, len(vc.unresolved))
	for i, c := range vc.unresolved {
		if _, ok := byKey[c.TitleKey]; !ok {
			byKey[c.TitleKey] = idx.Candidates("", c.TitleKey)
		}
		out[i] = slices.DeleteFunc(slices.Clone(byKey[c.TitleKey]), func(k inference.Candidate) bool { return k.NoteID == c.NoteID })
	}
	return out
}

// countOccurrences conta as notas com cada título candidato no corpo, ou o marca
// comum. Só as citações com mais de uma candidata precisam da probabilidade a priori,
// e com ela da conta.
func countOccurrences(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, candidates [][]inference.Candidate) (map[string]titleOccurrence, error) {
	params := gen.CountTitleOccurrencesParams{SearchConfig: searchConfig, VaultID: vaultID}
	seen := make(map[string]bool)
	for _, cs := range candidates {
		for _, c := range cs {
			if len(cs) > 1 && !seen[c.TitleKey] {
				seen[c.TitleKey] = true
				params.TitleKeys = append(params.TitleKeys, c.TitleKey)
			}
		}
	}
	occurrences := make(map[string]titleOccurrence, len(params.TitleKeys))
	if len(params.TitleKeys) == 0 {
		return occurrences, nil
	}
	rows, err := q.CountTitleOccurrences(ctx, params)
	for _, r := range rows {
		occurrences[r.TitleKey] = titleOccurrence{count: int(r.Occurrences), common: r.Common}
	}
	return occurrences, wrapVault(err, fmt.Sprintf("contar as ocorrencias de %d titulos candidatos", len(params.TitleKeys)), vaultID)
}

// chooseTargets escolhe a nota de cada citação sem nota e monta as linhas e as
// arestas das ligações inferidas.
func (vc *vaultCitations) chooseTargets(candidates [][]inference.Candidate, occurrences map[string]titleOccurrence,
	g *vaultgraph.Graph) (gen.SaveInferredLinksParams, []vaultgraph.Edge) {
	var params gen.SaveInferredLinksParams
	var edges []vaultgraph.Edge
	for i, c := range vc.unresolved {
		vc.scoreCandidates(c.NoteID, candidates[i], occurrences, g)
		target, ok := inference.Choose(candidates[i])
		if !ok {
			continue
		}
		params.NoteIds, params.CitedTitles = append(params.NoteIds, c.NoteID), append(params.CitedTitles, c.CitedTitle)
		params.TitleKeys, params.TargetIds = append(params.TitleKeys, target.TitleKey), append(params.TargetIds, target.NoteID)
		edges = append(edges, vaultgraph.Edge{Source: c.NoteID, Target: target.NoteID})
	}
	return params, edges
}

// scoreCandidates preenche a probabilidade a priori, com Laplace, de o título de cada
// candidata apontar para ela, e o parentesco dela com a nota sourceID. Título comum
// fica com prior zero, o último no sinal.
func (vc *vaultCitations) scoreCandidates(sourceID string, candidates []inference.Candidate, occurrences map[string]titleOccurrence, g *vaultgraph.Graph) {
	for i := range candidates {
		k := &candidates[i]
		if o := occurrences[k.TitleKey]; !o.common {
			k.Prior = mention.Prior(vc.anchorLinks(k.TitleKey, k.NoteID), o.count, vc.titleNotes[k.TitleKey])
		}
		k.Relatedness = g.ResourceAllocation(sourceID, k.NoteID)
	}
}

// anchorLinks são as notas que citam titleKey, fora a própria nota noteID.
func (vc *vaultCitations) anchorLinks(titleKey, noteID string) int {
	links := len(vc.citers[titleKey])
	if vc.citers[titleKey][noteID] {
		links--
	}
	return links
}

// wrapVault descreve o erro da operação no cofre vaultID, ou devolve nil.
func wrapVault(err error, operation, vaultID string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s do cofre %q: %w", operation, vaultID, err)
}
