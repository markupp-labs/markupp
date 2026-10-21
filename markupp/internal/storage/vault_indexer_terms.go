package storage

import (
	"context"
	"fmt"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// termsPerNote é quantos termos de maior tf-idf cada nota guarda para a similaridade
// léxica.
const termsPerNote = 50

// saveTerms troca os termos das notas do lote pelos de maior tf-idf do documento que
// saveDocuments acabou de gravar, no analisador da busca e com o df vigente.
func saveTerms(ctx context.Context, q *gen.Queries, cs indexing.Changeset, indexed []indexing.IndexedNote) error {
	noteIDs := batchNoteIDs(indexed)
	if err := q.DeleteNoteTerms(ctx, noteIDs); err != nil {
		return fmt.Errorf("apagar os termos das %d notas do cofre %q: %w", len(noteIDs), cs.VaultID, err)
	}
	params := gen.SaveNoteTermsParams{TermsPerNote: termsPerNote, VaultID: cs.VaultID, NoteIds: noteIDs}
	if err := q.SaveNoteTerms(ctx, params); err != nil {
		return fmt.Errorf("gravar os termos das %d notas do cofre %q: %w", len(noteIDs), cs.VaultID, err)
	}
	return nil
}
