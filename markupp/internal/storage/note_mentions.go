package storage

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/mention"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// mentionCandidate é uma menção achada no texto, ainda sem a probabilidade: a outra
// nota, o título citado e a nota que tem esse título.
type mentionCandidate struct {
	other     notes.LinkedNote
	titleKey  string
	targetID  string
	direction string
}

// anchorPair identifica um título e uma nota que o tem, a unidade da probabilidade.
type anchorPair struct {
	titleKey string
	targetID string
}

// Mentions devolve as menções não ligadas da nota noteID nos dois sentidos, uma por
// nota e sentido, da maior probabilidade a priori para a menor, ou notes.ErrNotFound
// quando o ator não enxerga a nota. Tudo é calculado na leitura.
func (g *PostgresNoteGraph) Mentions(ctx context.Context, noteID string, offset, limit int) ([]notes.UnlinkedMention, error) {
	var out []notes.UnlinkedMention
	err := g.inNote(ctx, noteID, func(q *gen.Queries) error {
		note, err := q.GetNoteByID(ctx, noteID)
		if err != nil {
			return err
		}
		found, err := g.findMentions(ctx, q, note)
		if err != nil {
			return err
		}
		ranked, err := g.rankMentions(ctx, q, note.VaultID, found)
		out = pageOf(ranked, offset, limit)
		return err
	})
	return out, err
}

func (g *PostgresNoteGraph) findMentions(ctx context.Context, q *gen.Queries, note gen.Note) ([]mentionCandidate, error) {
	incoming, err := g.mentionedBy(ctx, q, note)
	if err != nil {
		return nil, err
	}
	outgoing, err := mentionsOf(ctx, q, note)
	return append(incoming, outgoing...), err
}

// mentionedBy acha as notas que citam um título de note: o índice léxico dá as
// candidatas pelo título e pela forma sem acento, e o Matcher confere cada uma.
func (g *PostgresNoteGraph) mentionedBy(ctx context.Context, q *gen.Queries, note gen.Note) ([]mentionCandidate, error) {
	keys, err := q.ListNoteTitleKeys(ctx, note.ID)
	keys = slices.DeleteFunc(keys, func(k string) bool { return !mention.Eligible(k) })
	if err != nil || len(keys) == 0 {
		return nil, err
	}
	// ponytail: o GIN não tira acento, então o corpo acentuado não acha título sem
	// acento; resolve com unaccent na configuração de texto da instalação.
	phrases := append(slices.Clone(keys), convertRows(keys, mention.Fold)...)
	rows, err := q.ListMentioningNotes(ctx, gen.ListMentioningNotesParams{
		VaultID: note.VaultID, NoteID: note.ID, SearchConfig: g.searchConfig, Phrases: phrases,
	})
	return matchMentioningNotes(rows, keys, note.ID), err
}

// matchMentioningNotes confere em cada candidata do índice quais títulos de targetID
// ela cita de fato.
func matchMentioningNotes(rows []gen.ListMentioningNotesRow, keys []string, targetID string) []mentionCandidate {
	matcher := mention.NewMatcher(keys)
	var found []mentionCandidate
	for _, row := range rows {
		other := notes.LinkedNote{ID: row.ID, VaultID: row.VaultID, Path: row.Path, UpdatedAt: row.UpdatedAt}
		for _, key := range matcher.Find(row.Content) {
			found = append(found, mentionCandidate{other: other, titleKey: key, targetID: targetID, direction: notes.MentionIncoming})
		}
	}
	return found
}

// mentionsOf acha os títulos das outras notas do cofre que note cita no texto.
func mentionsOf(ctx context.Context, q *gen.Queries, note gen.Note) ([]mentionCandidate, error) {
	rows, err := q.ListMentionableTitles(ctx, gen.ListMentionableTitlesParams{VaultID: note.VaultID, NoteID: note.ID})
	if err != nil {
		return nil, err
	}
	cited := make(map[string]bool)
	for _, key := range mention.NewMatcher(eligibleUniqueKeys(rows)).Find(note.Content) {
		cited[key] = true
	}
	var found []mentionCandidate
	for _, row := range rows {
		if cited[row.TitleKey] {
			other := notes.LinkedNote{ID: row.ID, VaultID: row.VaultID, Path: row.Path, UpdatedAt: row.UpdatedAt}
			found = append(found, mentionCandidate{other: other, titleKey: row.TitleKey, targetID: row.ID, direction: notes.MentionOutgoing})
		}
	}
	return found, nil
}

func eligibleUniqueKeys(rows []gen.ListMentionableTitlesRow) []string {
	keys := convertRows(rows, func(r gen.ListMentionableTitlesRow) string { return r.TitleKey })
	slices.Sort(keys)
	return slices.DeleteFunc(slices.Compact(keys), func(k string) bool { return !mention.Eligible(k) })
}

// rankMentions descarta os títulos stopword, fica com a menção de maior
// probabilidade por nota e sentido e ordena pela probabilidade, depois pelo caminho.
func (g *PostgresNoteGraph) rankMentions(ctx context.Context, q *gen.Queries, vaultID string, found []mentionCandidate) ([]notes.UnlinkedMention, error) {
	priors, err := g.anchorPriors(ctx, q, vaultID, found)
	if err != nil {
		return nil, err
	}
	best := make(map[[2]string]notes.UnlinkedMention)
	for _, c := range found {
		prior, ok := priors[anchorPair{c.titleKey, c.targetID}]
		key := [2]string{c.other.ID, c.direction}
		if ok && prior >= best[key].Prior {
			best[key] = notes.UnlinkedMention{LinkedNote: c.other, Title: c.titleKey, Direction: c.direction, Prior: prior}
		}
	}
	ranked := slices.Collect(maps.Values(best))
	slices.SortFunc(ranked, compareMentions)
	return ranked, nil
}

func compareMentions(a, b notes.UnlinkedMention) int {
	return cmp.Or(cmp.Compare(b.Prior, a.Prior), cmp.Compare(a.Path, b.Path), cmp.Compare(a.ID, b.ID), cmp.Compare(a.Direction, b.Direction))
}

// anchorPriors calcula, num comando, a probabilidade de cada par título e nota
// achado; o par cujo título é stopword fica de fora.
func (g *PostgresNoteGraph) anchorPriors(ctx context.Context, q *gen.Queries, vaultID string, found []mentionCandidate) (map[anchorPair]float64, error) {
	params := gen.MentionAnchorStatsParams{SearchConfig: g.searchConfig, VaultID: vaultID}
	seen := make(map[anchorPair]bool)
	for _, c := range found {
		pair := anchorPair{c.titleKey, c.targetID}
		if !seen[pair] {
			seen[pair] = true
			params.TitleKeys, params.TargetIds = append(params.TitleKeys, pair.titleKey), append(params.TargetIds, pair.targetID)
		}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	rows, err := q.MentionAnchorStats(ctx, params)
	priors := make(map[anchorPair]float64, len(rows))
	for _, r := range rows {
		if !r.Stopword {
			priors[anchorPair{r.TitleKey, r.TargetID}] = mention.Prior(int(r.AnchorLinks), int(r.Occurrences), int(r.Candidates))
		}
	}
	return priors, err
}
