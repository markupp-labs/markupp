package storage

import (
	"context"
	"fmt"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/mention"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// mentionLink é a ligação por menção de source para target, com o título citado e a
// probabilidade a priori de ele apontar para target.
type mentionLink struct {
	source, target, titleKey string
	prior                    float64
}

// refreshNoteMentionLinks troca as ligações por menção que as notas do lote fazem
// pelas do texto atual delas (ADR-0053).
func refreshNoteMentionLinks(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, changed []indexing.NoteChange) error {
	noteIDs := make([]string, 0, len(changed))
	for _, n := range changed {
		noteIDs = append(noteIDs, n.NoteID)
	}
	if err := q.DeleteNoteMentionLinks(ctx, noteIDs); err != nil {
		return fmt.Errorf("delete mention links of %d notes in vault %q: %w", len(noteIDs), vaultID, err)
	}
	return saveMentionLinks(ctx, q, vaultID, searchConfig, changed)
}

// refreshVaultMentionLinks refaz as ligações por menção do cofre inteiro, porque um
// título novo ou renomeado muda o que cada nota cita.
func refreshVaultMentionLinks(ctx context.Context, q *gen.Queries, vaultID, searchConfig string) error {
	if err := q.DeleteVaultMentionLinks(ctx, vaultID); err != nil {
		return fmt.Errorf("delete mention links of vault %q: %w", vaultID, err)
	}
	// shortcut: lê o texto do cofre inteiro de uma vez; num cofre muito grande, ler por páginas.
	rows, err := q.ListVaultNoteContents(ctx, vaultID)
	if err != nil {
		return fmt.Errorf("list note contents of vault %q: %w", vaultID, err)
	}
	return saveMentionLinks(ctx, q, vaultID, searchConfig, convertRows(rows, func(r gen.ListVaultNoteContentsRow) indexing.NoteChange {
		return indexing.NoteChange{NoteID: r.ID, Content: r.Content}
	}))
}

// saveMentionLinks grava as ligações por menção que as notas sources fazem.
func saveMentionLinks(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, sources []indexing.NoteChange) error {
	links, err := deriveMentionLinks(ctx, q, vaultID, searchConfig, sources)
	if err != nil || len(links) == 0 {
		return err
	}
	if err := q.SaveAutomaticLinks(ctx, mentionLinkRows(links)); err != nil {
		return fmt.Errorf("save %d mention links of vault %q: %w", len(links), vaultID, err)
	}
	return nil
}

// deriveMentionLinks acha os títulos do cofre que cada nota de sources cita no texto e
// liga a nota a todas as que têm o título, fora ela mesma. Fica uma ligação por par,
// a do título de maior probabilidade, e o título curto, stopword ou comum não liga,
// como na menção não ligada.
func deriveMentionLinks(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, sources []indexing.NoteChange) ([]mentionLink, error) {
	holders, err := titleHolders(ctx, q, vaultID)
	if err != nil {
		return nil, err
	}
	found := findMentionLinks(holders, sources)
	priors, err := anchorPriors(ctx, q, searchConfig, vaultID, convertRows(found, func(l mentionLink) anchorPair { return anchorPair{l.titleKey, l.target} }))
	if err != nil {
		return nil, fmt.Errorf("weigh %d mentions in vault %q: %w", len(found), vaultID, err)
	}
	return bestMentionPerPair(found, priors), nil
}

// titleHolders devolve, para cada título que vale como menção, as notas que o têm.
func titleHolders(ctx context.Context, q *gen.Queries, vaultID string) (map[string][]string, error) {
	rows, err := q.ListVaultTitleKeys(ctx, vaultID)
	if err != nil {
		return nil, fmt.Errorf("list titles of vault %q: %w", vaultID, err)
	}
	holders := make(map[string][]string)
	for _, r := range rows {
		if mention.Eligible(r.TitleKey) {
			holders[r.TitleKey] = append(holders[r.TitleKey], r.NoteID)
		}
	}
	return holders, nil
}

// findMentionLinks confere no texto de cada nota quais títulos de holders ela cita.
func findMentionLinks(holders map[string][]string, sources []indexing.NoteChange) []mentionLink {
	keys := make([]string, 0, len(holders))
	for k := range holders {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	matcher := mention.NewMatcher(keys)
	var found []mentionLink
	for _, source := range sources {
		for _, key := range matcher.Find(source.Content) {
			for _, target := range holders[key] {
				if target != source.NoteID {
					found = append(found, mentionLink{source: source.NoteID, target: target, titleKey: key})
				}
			}
		}
	}
	return found
}

// bestMentionPerPair fica, para cada par, com o título de maior probabilidade, e
// descarta o par sem probabilidade, de título stopword ou comum.
func bestMentionPerPair(found []mentionLink, priors map[anchorPair]float64) []mentionLink {
	best := make(map[[2]string]mentionLink)
	for _, l := range found {
		prior, ok := priors[anchorPair{l.titleKey, l.target}]
		pair := [2]string{l.source, l.target}
		if current, seen := best[pair]; ok && (!seen || prior > current.prior) {
			l.prior = prior
			best[pair] = l
		}
	}
	out := make([]mentionLink, 0, len(best))
	for _, l := range best {
		out = append(out, l)
	}
	return out
}

func mentionLinkRows(links []mentionLink) gen.SaveAutomaticLinksParams {
	var p gen.SaveAutomaticLinksParams
	for _, l := range links {
		p.SourceIds = append(p.SourceIds, l.source)
		p.TargetIds = append(p.TargetIds, l.target)
		p.Kinds = append(p.Kinds, "mentioned")
		p.CitedTitles = append(p.CitedTitles, l.titleKey)
		p.Scores = append(p.Scores, l.prior)
	}
	return p
}
