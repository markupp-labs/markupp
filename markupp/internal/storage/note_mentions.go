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
	other    notes.NoteSummary
	titleKey string
	targetID string
}

// anchorPair identifica um título e uma nota que o tem, a unidade da probabilidade.
type anchorPair struct {
	titleKey string
	targetID string
}

// findMentions acha as menções de um sentido no texto, para a nota lida, com a
// probabilidade a priori de cada par título e nota achado; o par sem probabilidade
// tem título stopword ou comum e não conta.
type findMentions func(ctx context.Context, q *gen.Queries, note gen.Note) ([]mentionCandidate, map[anchorPair]float64, error)

// Mentions devolve os títulos de outras notas que a nota noteID cita sem ligar, uma
// menção por nota citada, ou notes.ErrNotFound quando o ator não enxerga a nota.
func (g *PostgresNoteGraph) Mentions(ctx context.Context, noteID string, p notes.Page) ([]notes.Mention, error) {
	return g.readMentions(ctx, noteID, p, g.mentionsOf)
}

// Backmentions devolve as notas que citam um título da nota noteID sem ligar, uma
// menção por nota que cita, ou notes.ErrNotFound quando o ator não enxerga a nota.
func (g *PostgresNoteGraph) Backmentions(ctx context.Context, noteID string, p notes.Page) ([]notes.Mention, error) {
	return g.readMentions(ctx, noteID, p, g.mentionedBy)
}

// readMentions calcula na leitura as menções que find acha, da maior probabilidade a
// priori para a menor.
func (g *PostgresNoteGraph) readMentions(ctx context.Context, noteID string, p notes.Page, find findMentions) ([]notes.Mention, error) {
	var out []notes.Mention
	err := g.inNote(ctx, noteID, func(q *gen.Queries) error {
		note, err := q.GetNoteByID(ctx, noteID)
		if err != nil {
			return err
		}
		found, priors, err := find(ctx, q, note)
		out = pageOf(rankMentions(found, priors), p.Offset, p.Limit)
		return err
	})
	return out, err
}

// mentionedBy acha as notas que citam um título de note que vale como menção: o
// índice léxico dá as candidatas pelo título e pela forma sem acento, e o Matcher
// confere cada uma.
func (g *PostgresNoteGraph) mentionedBy(ctx context.Context, q *gen.Queries, note gen.Note) ([]mentionCandidate, map[anchorPair]float64, error) {
	keys, priors, err := g.mentionableKeys(ctx, q, note)
	if err != nil || len(keys) == 0 {
		return nil, nil, err
	}
	// ponytail: o GIN não tira acento, então o corpo acentuado não acha título sem
	// acento; resolve com unaccent na configuração de texto da instalação.
	rows, err := q.ListMentioningNotes(ctx, gen.ListMentioningNotesParams{
		VaultID: note.VaultID, NoteID: note.ID, SearchConfig: g.searchConfig, Phrases: append(slices.Clone(keys), convertRows(keys, mention.Fold)...),
	})
	return matchMentioningNotes(rows, keys, note.ID), priors, err
}

// mentionableKeys devolve os títulos de note que valem como menção, com a
// probabilidade de cada um: fora os curtos, os stopword e os comuns, que nem chegam
// à busca no índice léxico.
func (g *PostgresNoteGraph) mentionableKeys(ctx context.Context, q *gen.Queries, note gen.Note) ([]string, map[anchorPair]float64, error) {
	keys, err := q.ListNoteTitleKeys(ctx, note.ID)
	keys = slices.DeleteFunc(keys, func(k string) bool { return !mention.Eligible(k) })
	if err != nil || len(keys) == 0 {
		return nil, nil, err
	}
	priors, err := g.anchorPriors(ctx, q, note.VaultID, convertRows(keys, func(k string) anchorPair { return anchorPair{k, note.ID} }))
	return slices.DeleteFunc(keys, func(k string) bool { _, ok := priors[anchorPair{k, note.ID}]; return !ok }), priors, err
}

// matchMentioningNotes confere em cada candidata do índice quais títulos de targetID
// ela cita de fato.
func matchMentioningNotes(rows []gen.ListMentioningNotesRow, keys []string, targetID string) []mentionCandidate {
	matcher := mention.NewMatcher(keys)
	var found []mentionCandidate
	for _, row := range rows {
		other := notes.Summarize(row.ID, row.Path)
		for _, key := range matcher.Find(row.Content) {
			found = append(found, mentionCandidate{other: other, titleKey: key, targetID: targetID})
		}
	}
	return found
}

// mentionsOf acha os títulos das outras notas do cofre que note cita no texto.
func (g *PostgresNoteGraph) mentionsOf(ctx context.Context, q *gen.Queries, note gen.Note) ([]mentionCandidate, map[anchorPair]float64, error) {
	rows, err := q.ListMentionableTitles(ctx, gen.ListMentionableTitlesParams{VaultID: note.VaultID, NoteID: note.ID})
	if err != nil {
		return nil, nil, err
	}
	cited := make(map[string]bool)
	for _, key := range mention.NewMatcher(eligibleUniqueKeys(rows)).Find(note.Content) {
		cited[key] = true
	}
	var found []mentionCandidate
	for _, row := range rows {
		if cited[row.TitleKey] {
			found = append(found, mentionCandidate{other: notes.Summarize(row.ID, row.Path), titleKey: row.TitleKey, targetID: row.ID})
		}
	}
	priors, err := g.anchorPriors(ctx, q, note.VaultID, convertRows(found, func(c mentionCandidate) anchorPair { return anchorPair{c.titleKey, c.targetID} }))
	return found, priors, err
}

func eligibleUniqueKeys(rows []gen.ListMentionableTitlesRow) []string {
	keys := convertRows(rows, func(r gen.ListMentionableTitlesRow) string { return r.TitleKey })
	slices.Sort(keys)
	return slices.DeleteFunc(slices.Compact(keys), func(k string) bool { return !mention.Eligible(k) })
}

// rankMentions fica com a menção de maior probabilidade por nota, descartando os
// pares sem probabilidade, e ordena pela probabilidade, depois pelo caminho.
func rankMentions(found []mentionCandidate, priors map[anchorPair]float64) []notes.Mention {
	best := make(map[string]notes.Mention)
	for _, c := range found {
		prior, ok := priors[anchorPair{c.titleKey, c.targetID}]
		if ok && prior >= best[c.other.ID].Prior {
			best[c.other.ID] = notes.Mention{Note: c.other, CitedTitle: c.titleKey, Prior: prior}
		}
	}
	ranked := slices.Collect(maps.Values(best))
	slices.SortFunc(ranked, compareMentions)
	return ranked
}

func compareMentions(a, b notes.Mention) int {
	return cmp.Or(cmp.Compare(b.Prior, a.Prior), cmp.Compare(a.Note.Path, b.Note.Path), cmp.Compare(a.Note.ID, b.Note.ID))
}

// anchorPriors calcula, num comando, a probabilidade de cada par título e nota; o
// par cujo título é stopword ou comum, em mais de 5% das notas do cofre (Mihalcea e
// Csomai, 2007), fica de fora.
func (g *PostgresNoteGraph) anchorPriors(ctx context.Context, q *gen.Queries, vaultID string, pairs []anchorPair) (map[anchorPair]float64, error) {
	params := gen.MentionAnchorStatsParams{SearchConfig: g.searchConfig, VaultID: vaultID}
	for _, pair := range uniquePairs(pairs) {
		params.TitleKeys, params.TargetIds = append(params.TitleKeys, pair.titleKey), append(params.TargetIds, pair.targetID)
	}
	if len(params.TitleKeys) == 0 {
		return nil, nil
	}
	rows, err := q.MentionAnchorStats(ctx, params)
	priors := make(map[anchorPair]float64, len(rows))
	for _, r := range rows {
		if !r.Stopword && !r.Common {
			priors[anchorPair{r.TitleKey, r.TargetID}] = mention.Prior(int(r.AnchorLinks), int(r.Occurrences), int(r.Candidates))
		}
	}
	return priors, err
}

// uniquePairs devolve os pares sem repetição, na ordem em que aparecem.
func uniquePairs(pairs []anchorPair) []anchorPair {
	seen := make(map[anchorPair]bool, len(pairs))
	return slices.DeleteFunc(slices.Clone(pairs), func(p anchorPair) bool {
		dup := seen[p]
		seen[p] = true
		return dup
	})
}
