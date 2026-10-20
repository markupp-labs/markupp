package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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
// desmarca só a revisão lida, que vira a versão do grafo do cofre: a marcação de um
// lote concorrente fica para o próximo Job. searchConfig é a configuração de texto do
// índice léxico, e logger recebe o aviso de cálculo parado no teto.
func runSecondStage(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, logger *slog.Logger) error {
	revision, err := q.SecondStageRevision(ctx, vaultID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("ler a marcacao do segundo estagio do cofre %q: %w", vaultID, err)
	}
	if err := refreshVault(ctx, q, vaultID, searchConfig, logger); err != nil {
		return err
	}
	if err := q.SaveGraphVersion(ctx, gen.SaveGraphVersionParams{VaultID: vaultID, Version: revision}); err != nil {
		return fmt.Errorf("gravar a versao %d do grafo do cofre %q: %w", revision, vaultID, err)
	}
	if err := q.UnmarkSecondStage(ctx, gen.UnmarkSecondStageParams{VaultID: vaultID, Revision: revision}); err != nil {
		return fmt.Errorf("desmarcar a revisao %d do segundo estagio do cofre %q: %w", revision, vaultID, err)
	}
	return nil
}

// refreshVault grava o df e o N do cofre antes do grafo, para o filtro de título
// comum da ligação inferida valer já no primeiro Job, e refaz depois as ligações
// inferidas, o PageRank e as comunidades. As postagens vêm por último, porque a
// repesagem trava as linhas de note_terms até o commit, e um lote concorrente que
// regrava essas notas espera por elas.
func refreshVault(ctx context.Context, q *gen.Queries, vaultID, searchConfig string, logger *slog.Logger) error {
	known, err := vaultTermStatsKnown(ctx, q, vaultID)
	if err != nil {
		return err
	}
	if err := saveTermFrequencies(ctx, q, vaultID); err != nil {
		return err
	}
	if err := refreshVaultGraph(ctx, q, vaultID, searchConfig, logger); err != nil {
		return err
	}
	if err := q.ReweightVaultNoteTerms(ctx, vaultID); err != nil {
		return fmt.Errorf("repesar as postagens do cofre %q: %w", vaultID, err)
	}
	return requeueOnFirstTermStats(ctx, q, vaultID, known)
}

// requeueOnFirstTermStats reenfileira as notas do cofre quando ele ganha df agora
// (primeira vez, depois do reindex ou depois de um df de N zero), para o lote
// reescolher os termos com o idf de verdade; wasKnown diz se ele já tinha df antes.
// Df de N zero não reenfileira: nenhuma nota tem documento, nem termo para
// reescolher, e a nota ainda sem documento já está na fila. Reenfileirar não marca o
// cofre, e o lote que só regrava as mesmas notas também não, então o segundo estágio
// não volta por isso.
func requeueOnFirstTermStats(ctx context.Context, q *gen.Queries, vaultID string, wasKnown bool) error {
	if wasKnown {
		return nil
	}
	if known, err := vaultTermStatsKnown(ctx, q, vaultID); err != nil || !known {
		return err
	}
	if err := q.RequeueVaultNotes(ctx, vaultID); err != nil {
		return fmt.Errorf("reenfileirar as notas do cofre %q para reescolher os termos com o df novo: %w", vaultID, err)
	}
	return nil
}

// vaultTermStatsKnown diz se o cofre tem df de N maior que zero.
func vaultTermStatsKnown(ctx context.Context, q *gen.Queries, vaultID string) (bool, error) {
	known, err := q.VaultTermStatsKnown(ctx, vaultID)
	if err != nil {
		return false, fmt.Errorf("ler se o cofre %q ja tem df: %w", vaultID, err)
	}
	return known, nil
}

// saveTermFrequencies troca o df e o N do cofre pelos do índice léxico atual.
func saveTermFrequencies(ctx context.Context, q *gen.Queries, vaultID string) error {
	if err := q.DeleteVaultTermFrequencies(ctx, vaultID); err != nil {
		return fmt.Errorf("apagar o df do cofre %q: %w", vaultID, err)
	}
	if err := q.SaveVaultTermFrequencies(ctx, vaultID); err != nil {
		return fmt.Errorf("gravar o df do cofre %q: %w", vaultID, err)
	}
	if err := q.SaveVaultTermStats(ctx, vaultID); err != nil {
		return fmt.Errorf("gravar o N do df do cofre %q: %w", vaultID, err)
	}
	return nil
}
