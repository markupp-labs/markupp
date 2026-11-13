package storage

import (
	"context"
	"fmt"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// A ligação por texto parecido (ADR-0053) liga duas notas quando cada uma está entre
// as relatedNeighbors de maior cosseno tf-idf da outra (kNN mútuo) e o cosseno chega a
// relatedMinScore.
const (
	relatedNeighbors = 5
	relatedMinScore  = 0.2
)

// relatedLink é a ligação por texto parecido de source para target, com o cosseno.
type relatedLink struct {
	source, target string
	score          float64
}

// refreshNoteRelatedLinks troca as ligações por texto parecido das notas do lote pelas
// dos termos que o lote acabou de gravar. A vizinha delas entra na conta para conferir
// o kNN mútuo, mas o par entre duas notas de fora do lote fica como estava até o
// segundo estágio, que o lote marca quando a ligação de uma nota dele muda.
func refreshNoteRelatedLinks(ctx context.Context, q *gen.Queries, vaultID string, noteIDs []string) error {
	if err := q.DeleteNoteRelatedLinks(ctx, noteIDs); err != nil {
		return fmt.Errorf("delete related links of %d notes in vault %q: %w", len(noteIDs), vaultID, err)
	}
	rows, err := lexicalNeighbors(ctx, q, vaultID, noteIDs)
	if err != nil {
		return err
	}
	batch := idSet(noteIDs)
	more, err := lexicalNeighbors(ctx, q, vaultID, neighborsOutside(rows, batch))
	if err != nil {
		return err
	}
	return saveRelatedLinks(ctx, q, vaultID, mutualNeighbors(append(rows, more...), batch))
}

// refreshVaultRelatedLinks refaz as ligações por texto parecido do cofre inteiro, porque
// a nota nova ou mudada tira ou põe outra entre as vizinhas de quem nem estava no lote.
func refreshVaultRelatedLinks(ctx context.Context, q *gen.Queries, vaultID string) error {
	if err := q.DeleteVaultRelatedLinks(ctx, vaultID); err != nil {
		return fmt.Errorf("delete related links of vault %q: %w", vaultID, err)
	}
	noteIDs, err := q.ListVaultNoteIDs(ctx, vaultID)
	if err != nil {
		return fmt.Errorf("list notes of vault %q: %w", vaultID, err)
	}
	// shortcut: uma consulta pelas vizinhas do cofre inteiro; num cofre muito grande, ler por páginas.
	rows, err := lexicalNeighbors(ctx, q, vaultID, noteIDs)
	if err != nil {
		return err
	}
	return saveRelatedLinks(ctx, q, vaultID, mutualNeighbors(rows, idSet(noteIDs)))
}

// lexicalNeighbors lê as vizinhas de cada nota de noteIDs pelo cosseno.
func lexicalNeighbors(ctx context.Context, q *gen.Queries, vaultID string, noteIDs []string) ([]gen.ListLexicalNeighborsRow, error) {
	if len(noteIDs) == 0 {
		return nil, nil
	}
	rows, err := q.ListLexicalNeighbors(ctx, gen.ListLexicalNeighborsParams{VaultID: vaultID, NoteIds: noteIDs, K: relatedNeighbors, MinScore: relatedMinScore})
	if err != nil {
		return nil, fmt.Errorf("list lexical neighbors of %d notes in vault %q: %w", len(noteIDs), vaultID, err)
	}
	return rows, nil
}

// neighborsOutside devolve, sem repetir, as vizinhas que não estão em batch.
func neighborsOutside(rows []gen.ListLexicalNeighborsRow, batch map[string]bool) []string {
	seen := make(map[string]bool)
	var out []string
	for _, r := range rows {
		if !batch[r.NeighborID] && !seen[r.NeighborID] {
			seen[r.NeighborID] = true
			out = append(out, r.NeighborID)
		}
	}
	return out
}

// mutualNeighbors liga a nota à vizinha que também a tem entre as vizinhas, nos dois
// sentidos, e só no par que toca uma nota de touching.
func mutualNeighbors(rows []gen.ListLexicalNeighborsRow, touching map[string]bool) []relatedLink {
	listed := make(map[[2]string]bool, len(rows))
	for _, r := range rows {
		listed[[2]string{r.NoteID, r.NeighborID}] = true
	}
	var out []relatedLink
	for _, r := range rows {
		if listed[[2]string{r.NeighborID, r.NoteID}] && (touching[r.NoteID] || touching[r.NeighborID]) {
			out = append(out, relatedLink{source: r.NoteID, target: r.NeighborID, score: r.Score})
		}
	}
	return out
}

func saveRelatedLinks(ctx context.Context, q *gen.Queries, vaultID string, links []relatedLink) error {
	if len(links) == 0 {
		return nil
	}
	var p gen.SaveAutomaticLinksParams
	for _, l := range links {
		p.SourceIds = append(p.SourceIds, l.source)
		p.TargetIds = append(p.TargetIds, l.target)
		p.Kinds = append(p.Kinds, "related")
		p.CitedTitles = append(p.CitedTitles, "")
		p.Scores = append(p.Scores, l.score)
	}
	if err := q.SaveAutomaticLinks(ctx, p); err != nil {
		return fmt.Errorf("save %d related links of vault %q: %w", len(links), vaultID, err)
	}
	return nil
}

func idSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
