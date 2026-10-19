package storage

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/suggestion"
)

// suggestedNote é uma linha de uma das listas de sugestão: a nota e o sinal dela.
type suggestedNote struct {
	note  notes.LinkedNote
	score float64
}

// Suggestions devolve as notas que a nota noteID poderia ligar, fora ela e as já
// ligadas: a lista da alocação de recursos e a do cosseno tf-idf fundidas por RRF, da
// maior pontuação para a menor, ou notes.ErrNotFound quando o ator não enxerga a nota.
// Tudo é calculado na leitura.
func (g *PostgresNoteGraph) Suggestions(ctx context.Context, noteID string, offset, limit int) ([]notes.LinkSuggestion, error) {
	var out []notes.LinkSuggestion
	err := g.inNote(ctx, noteID, func(q *gen.Queries) error {
		structural, lexical, err := suggestionLists(ctx, q, noteID)
		out = pageOf(fuseSuggestions(structural, lexical), offset, limit)
		return err
	})
	return out, err
}

func suggestionLists(ctx context.Context, q *gen.Queries, noteID string) (structural, lexical []suggestedNote, err error) {
	note, err := q.GetNoteByID(ctx, noteID)
	if err != nil {
		return nil, nil, err
	}
	ra, err := q.ListStructuralSuggestions(ctx, noteID)
	if err != nil {
		return nil, nil, err
	}
	cos, err := q.ListLexicalSuggestions(ctx, gen.ListLexicalSuggestionsParams{VaultID: note.VaultID, NoteID: noteID})
	return convertRows(ra, toSuggestedNote), convertRows(cos, func(r gen.ListLexicalSuggestionsRow) suggestedNote {
		return toSuggestedNote(gen.ListStructuralSuggestionsRow(r))
	}), err
}

func toSuggestedNote(r gen.ListStructuralSuggestionsRow) suggestedNote {
	return suggestedNote{note: notes.LinkedNote{ID: r.ID, VaultID: r.VaultID, Path: r.Path, UpdatedAt: r.UpdatedAt}, score: r.Score}
}

// fuseSuggestions junta as duas listas, já ordenadas pelo próprio sinal, numa só,
// ordenada pela pontuação RRF e depois pelo caminho.
func fuseSuggestions(structural, lexical []suggestedNote) []notes.LinkSuggestion {
	fused := suggestion.Fuse(suggestedIDs(structural), suggestedIDs(lexical))
	byID := make(map[string]notes.LinkSuggestion, len(fused))
	for _, s := range structural {
		byID[s.note.ID] = notes.LinkSuggestion{LinkedNote: s.note, Score: fused[s.note.ID], ResourceAllocation: s.score}
	}
	for _, s := range lexical {
		merged := byID[s.note.ID]
		merged.LinkedNote, merged.Score, merged.Cosine = s.note, fused[s.note.ID], s.score
		byID[s.note.ID] = merged
	}
	ranked := slices.Collect(maps.Values(byID))
	slices.SortFunc(ranked, func(a, b notes.LinkSuggestion) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.Path, b.Path), cmp.Compare(a.ID, b.ID))
	})
	return ranked
}

func suggestedIDs(list []suggestedNote) []string {
	return convertRows(list, func(s suggestedNote) string { return s.note.ID })
}

// pageOf recorta a página que começa em offset e tem até limit itens.
func pageOf[T any](items []T, offset, limit int) []T {
	return items[min(offset, len(items)):min(offset+limit, len(items))]
}
