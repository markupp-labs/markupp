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

// anchorKey identifica o par (chave de título, nota com o título) da probabilidade a
// priori da âncora.
type anchorKey struct {
	titleKey string
	noteID   string
}

// inferLinks grava uma ligação inferida para cada título sem nota citado no cofre,
// quando há candidata, e devolve as arestas gravadas. O grafo g é o das ligações
// resolvidas, sem inferidas, e dá o parentesco entre a nota que cita e a candidata.
func inferLinks(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, g *vaultgraph.Graph) ([]vaultgraph.Edge, error) {
	citations, err := q.ListUnresolvedCitations(ctx, vaultID)
	if err != nil || len(citations) == 0 {
		return nil, wrapVault(err, "listar as ligacoes nao resolvidas", vaultID)
	}
	candidates, err := citationCandidates(ctx, q, vaultID, citations)
	if err != nil {
		return nil, err
	}
	priors, err := anchorPriors(ctx, q, vaultID, searchConfig, candidates)
	if err != nil {
		return nil, err
	}
	params, edges := chooseTargets(citations, candidates, priors, g)
	if err := q.SaveInferredLinks(ctx, params); err != nil {
		return nil, fmt.Errorf("gravar as %d ligacoes inferidas do cofre %q: %w", len(edges), vaultID, err)
	}
	return edges, nil
}

// citationCandidates devolve, na ordem das citações, as notas com título perto do
// citado, fora a própria nota que cita. O mesmo título citado é comparado uma vez.
func citationCandidates(ctx context.Context, q *gen.Queries, vaultID string, citations []gen.ListUnresolvedCitationsRow) ([][]inference.Candidate, error) {
	rows, err := q.ListVaultTitleKeys(ctx, vaultID)
	if err != nil {
		return nil, fmt.Errorf("listar os titulos do cofre %q: %w", vaultID, err)
	}
	titles := make([]inference.Title, len(rows))
	for i, r := range rows {
		titles[i] = inference.Title{NoteID: r.NoteID, Key: r.TitleKey}
	}
	idx := inference.NewTitleIndex(titles)
	byKey := make(map[string][]inference.Candidate)
	out := make([][]inference.Candidate, len(citations))
	for i, c := range citations {
		if _, ok := byKey[c.TitleKey]; !ok {
			byKey[c.TitleKey] = idx.Candidates("", c.TitleKey)
		}
		out[i] = slices.DeleteFunc(slices.Clone(byKey[c.TitleKey]), func(k inference.Candidate) bool { return k.NoteID == c.NoteID })
	}
	return out, nil
}

// anchorPriors devolve a probabilidade a priori, com Laplace, de cada título
// candidato apontar para a nota dele. Só as citações com mais de uma candidata
// precisam dela.
func anchorPriors(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, candidates [][]inference.Candidate) (map[anchorKey]float64, error) {
	params := gen.MentionAnchorStatsParams{SearchConfig: searchConfig, VaultID: vaultID}
	seen := make(map[anchorKey]bool)
	for _, cs := range candidates {
		for _, c := range cs {
			key := anchorKey{titleKey: c.TitleKey, noteID: c.NoteID}
			if len(cs) > 1 && !seen[key] {
				seen[key] = true
				params.TitleKeys, params.TargetIds = append(params.TitleKeys, c.TitleKey), append(params.TargetIds, c.NoteID)
			}
		}
	}
	return queryAnchorPriors(ctx, q, params)
}

func queryAnchorPriors(ctx context.Context, q *gen.Queries, params gen.MentionAnchorStatsParams) (map[anchorKey]float64, error) {
	priors := make(map[anchorKey]float64, len(params.TitleKeys))
	if len(params.TitleKeys) == 0 {
		return priors, nil
	}
	rows, err := q.MentionAnchorStats(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("contar as ancoras de %d titulos candidatos do cofre %q: %w", len(params.TitleKeys), params.VaultID, err)
	}
	for _, r := range rows {
		priors[anchorKey{titleKey: r.TitleKey, noteID: r.TargetID}] = mention.Prior(int(r.AnchorLinks), int(r.Occurrences), int(r.Candidates))
	}
	return priors, nil
}

// chooseTargets escolhe a nota de cada citação e monta as linhas e as arestas das
// ligações inferidas.
func chooseTargets(citations []gen.ListUnresolvedCitationsRow, candidates [][]inference.Candidate,
	priors map[anchorKey]float64, g *vaultgraph.Graph) (gen.SaveInferredLinksParams, []vaultgraph.Edge) {
	var params gen.SaveInferredLinksParams
	var edges []vaultgraph.Edge
	for i, c := range citations {
		scoreCandidates(c.NoteID, candidates[i], priors, g)
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

// scoreCandidates preenche a probabilidade a priori e o parentesco com a nota
// sourceID de cada candidata.
func scoreCandidates(sourceID string, candidates []inference.Candidate, priors map[anchorKey]float64, g *vaultgraph.Graph) {
	for i := range candidates {
		k := &candidates[i]
		k.Prior = priors[anchorKey{titleKey: k.TitleKey, noteID: k.NoteID}]
		k.Relatedness = g.ResourceAllocation(sourceID, k.NoteID)
	}
}

// wrapVault descreve o erro da operação no cofre vaultID, ou devolve nil.
func wrapVault(err error, operation, vaultID string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s do cofre %q: %w", operation, vaultID, err)
}
