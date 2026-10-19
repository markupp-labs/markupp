package storage

import (
	"context"
	"fmt"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// saveGraph troca os títulos e as ligações das notas do lote pelos que o indexador
// acabou de derivar. Roda depois de saveDocuments, que já conferiu o lote.
func saveGraph(ctx context.Context, q *gen.Queries, cs indexing.Changeset, indexed []indexing.IndexedNote) error {
	noteIDs := batchNoteIDs(indexed)
	if err := q.DeleteNoteGraph(ctx, noteIDs); err != nil {
		return fmt.Errorf("apagar o grafo das %d notas do cofre %q: %w", len(noteIDs), cs.VaultID, err)
	}
	if err := q.SaveNoteTitles(ctx, titleRows(indexed)); err != nil {
		return fmt.Errorf("gravar os titulos das %d notas do cofre %q: %w", len(noteIDs), cs.VaultID, err)
	}
	if err := q.SaveNoteLinks(ctx, linkRows(indexed)); err != nil {
		return fmt.Errorf("gravar as ligacoes das %d notas do cofre %q: %w", len(noteIDs), cs.VaultID, err)
	}
	return nil
}

func batchNoteIDs(indexed []indexing.IndexedNote) []string {
	noteIDs := make([]string, 0, len(indexed))
	for _, note := range indexed {
		noteIDs = append(noteIDs, note.Search.NoteID)
	}
	return noteIDs
}

func titleRows(indexed []indexing.IndexedNote) gen.SaveNoteTitlesParams {
	var p gen.SaveNoteTitlesParams
	for _, note := range indexed {
		for _, key := range note.TitleKeys {
			p.NoteIds = append(p.NoteIds, note.Search.NoteID)
			p.TitleKeys = append(p.TitleKeys, key)
		}
	}
	return p
}

func linkRows(indexed []indexing.IndexedNote) gen.SaveNoteLinksParams {
	var p gen.SaveNoteLinksParams
	for _, note := range indexed {
		for _, link := range note.Links {
			p.NoteIds = append(p.NoteIds, note.Search.NoteID)
			p.CitedTitles = append(p.CitedTitles, link.CitedTitle)
			p.TitleKeys = append(p.TitleKeys, link.TitleKey)
			p.TargetPaths = append(p.TargetPaths, link.TargetPath)
			p.Kinds = append(p.Kinds, string(link.Kind))
			p.Predicates = append(p.Predicates, link.Predicate)
		}
	}
	return p
}
