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
// quando o lote mudou títulos, caminhos ou ligações: nota nova, renomeada, movida de
// pasta ou com outras ligações. A exclusão de nota marca o cofre por trigger.
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
// desmarca só a revisão lida: a marcação de um lote concorrente fica para o próximo
// Job. searchConfig é a configuração de texto do índice léxico. As postagens vêm por
// último, porque a repesagem trava as linhas de note_terms até o commit, e um lote
// concorrente que regrava essas notas espera por elas.
func runSecondStage(ctx context.Context, q *gen.Queries, vaultID, searchConfig string) error {
	revision, err := q.SecondStageRevision(ctx, vaultID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ler a marcacao do segundo estagio do cofre %q: %w", vaultID, err)
	}
	if err := refreshVaultGraph(ctx, q, vaultID, searchConfig); err != nil {
		return err
	}
	if err := refreshTerms(ctx, q, vaultID); err != nil {
		return err
	}
	if err := q.UnmarkSecondStage(ctx, gen.UnmarkSecondStageParams{VaultID: vaultID, Revision: revision}); err != nil {
		return fmt.Errorf("desmarcar a revisao %d do segundo estagio do cofre %q: %w", revision, vaultID, err)
	}
	return nil
}

// refreshTerms recalcula o df do cofre e, quando ele ainda não tinha df (primeira vez
// ou depois do reindex), reenfileira as notas do cofre para o lote reescolher os
// termos com o idf de verdade. Reenfileirar não marca o cofre, e o lote que só regrava
// as mesmas notas também não, então o segundo estágio não volta por isso.
func refreshTerms(ctx context.Context, q *gen.Queries, vaultID string) error {
	known, err := q.VaultTermStatsKnown(ctx, vaultID)
	if err != nil {
		return fmt.Errorf("ler se o cofre %q ja tem df: %w", vaultID, err)
	}
	if err := refreshTermFrequencies(ctx, q, vaultID); err != nil || known {
		return err
	}
	if err := q.RequeueVaultNotes(ctx, vaultID); err != nil {
		return fmt.Errorf("reenfileirar as notas do cofre %q para reescolher os termos com o df novo: %w", vaultID, err)
	}
	return nil
}

// refreshTermFrequencies troca o df e o N do cofre pelos do índice léxico atual e
// repesa as postagens das notas com eles.
func refreshTermFrequencies(ctx context.Context, q *gen.Queries, vaultID string) error {
	if err := q.DeleteVaultTermFrequencies(ctx, vaultID); err != nil {
		return fmt.Errorf("apagar o df do cofre %q: %w", vaultID, err)
	}
	if err := q.SaveVaultTermFrequencies(ctx, vaultID); err != nil {
		return fmt.Errorf("gravar o df do cofre %q: %w", vaultID, err)
	}
	if err := q.SaveVaultTermStats(ctx, vaultID); err != nil {
		return fmt.Errorf("gravar o N do df do cofre %q: %w", vaultID, err)
	}
	if err := q.ReweightVaultNoteTerms(ctx, vaultID); err != nil {
		return fmt.Errorf("repesar as postagens do cofre %q: %w", vaultID, err)
	}
	return nil
}
