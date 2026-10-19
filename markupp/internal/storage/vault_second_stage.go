package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// saveGraphMarkingChange grava o grafo do lote e marca o cofre para o segundo estágio
// quando o lote mudou títulos ou ligações: nota nova, renomeada ou com outras
// ligações. A exclusão de nota marca o cofre por trigger.
func saveGraphMarkingChange(ctx context.Context, q *gen.Queries, cs indexing.Changeset, indexed []indexing.IndexedNote) error {
	noteIDs := batchNoteIDs(indexed)
	before, err := q.NoteGraphFingerprint(ctx, noteIDs)
	if err != nil {
		return fmt.Errorf("ler a impressao do grafo das %d notas do cofre %q: %w", len(noteIDs), cs.VaultID, err)
	}
	if err := saveGraph(ctx, q, cs, indexed); err != nil {
		return err
	}
	after, err := q.NoteGraphFingerprint(ctx, noteIDs)
	if err != nil || before == after {
		return err
	}
	if err := q.MarkSecondStage(ctx, cs.VaultID); err != nil {
		return fmt.Errorf("marcar o cofre %q para o segundo estagio: %w", cs.VaultID, err)
	}
	return nil
}

// runSecondStage recalcula o que depende do cofre inteiro, se ele estiver marcado, e
// desmarca só a revisão lida. searchConfig é a configuração de texto do índice léxico.
func runSecondStage(ctx context.Context, q *gen.Queries, vaultID, searchConfig string) error {
	revision, err := q.SecondStageRevision(ctx, vaultID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ler a marcacao do segundo estagio do cofre %q: %w", vaultID, err)
	}
	if err := refreshTermFrequencies(ctx, q, vaultID); err != nil {
		return err
	}
	if err := refreshVaultGraph(ctx, q, vaultID, searchConfig); err != nil {
		return err
	}
	if err := q.UnmarkSecondStage(ctx, gen.UnmarkSecondStageParams{VaultID: vaultID, Revision: revision}); err != nil {
		return fmt.Errorf("desmarcar a revisao %d do segundo estagio do cofre %q: %w", revision, vaultID, err)
	}
	return nil
}

// refreshTermFrequencies troca o df do cofre pelo do índice léxico atual.
func refreshTermFrequencies(ctx context.Context, q *gen.Queries, vaultID string) error {
	if err := q.DeleteVaultTermFrequencies(ctx, vaultID); err != nil {
		return fmt.Errorf("apagar o df do cofre %q: %w", vaultID, err)
	}
	if err := q.SaveVaultTermFrequencies(ctx, vaultID); err != nil {
		return fmt.Errorf("gravar o df do cofre %q: %w", vaultID, err)
	}
	return nil
}
