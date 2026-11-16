package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/linkrefusals"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresLinkRefusals grava as recusas de ligação no plano de registro, em
// transação do ator do contexto, sob RLS. Criar ou desfazer marca o cofre para o
// segundo estágio, por trigger.
type PostgresLinkRefusals struct {
	pool *pgxpool.Pool
}

// NewPostgresLinkRefusals monta as recusas sobre um pool já aberto.
func NewPostgresLinkRefusals(pool *pgxpool.Pool) *PostgresLinkRefusals {
	return &PostgresLinkRefusals{pool: pool}
}

// Create recusa o par noteA e noteB do cofre vaultID e devolve a recusa, a mesma se
// o par já estava recusado; linkrefusals.ErrNoteOutsideVault quando alguma das notas
// não é do cofre.
func (r *PostgresLinkRefusals) Create(ctx context.Context, vaultID, noteA, noteB string) (linkrefusals.Refusal, error) {
	var out linkrefusals.Refusal
	err := r.inVault(ctx, vaultID, func(q *gen.Queries) error {
		if err := requireVaultNotes(ctx, q, vaultID, noteA, noteB); err != nil {
			return err
		}
		if err := q.CreateLinkRefusal(ctx, gen.CreateLinkRefusalParams{VaultID: vaultID, NoteA: noteA, NoteB: noteB}); err != nil {
			return fmt.Errorf("refuse link between %q and %q in vault %q: %w", noteA, noteB, vaultID, err)
		}
		row, err := q.GetLinkRefusalByPair(ctx, gen.GetLinkRefusalByPairParams{NoteA: noteA, NoteB: noteB})
		out = toRefusal(gen.ListLinkRefusalsRow(row))
		return err
	})
	return out, err
}

// List devolve as recusas do cofre vaultID, da mais nova para a mais antiga, ou
// notes.ErrVaultForbidden quando o ator não enxerga o cofre.
func (r *PostgresLinkRefusals) List(ctx context.Context, vaultID string, p notes.Page) ([]linkrefusals.Refusal, error) {
	var out []linkrefusals.Refusal
	err := r.inVault(ctx, vaultID, func(q *gen.Queries) error {
		rows, err := q.ListLinkRefusals(ctx, gen.ListLinkRefusalsParams{VaultID: vaultID, PageLimit: toInt32(p.Limit), PageOffset: toInt32(p.Offset)})
		out = convertRows(rows, toRefusal)
		return err
	})
	return out, err
}

// Delete desfaz a recusa id do cofre vaultID, ou devolve
// linkrefusals.ErrRefusalNotFound.
func (r *PostgresLinkRefusals) Delete(ctx context.Context, vaultID, id string) error {
	return r.inVault(ctx, vaultID, func(q *gen.Queries) error {
		deleted, err := q.DeleteLinkRefusal(ctx, gen.DeleteLinkRefusalParams{VaultID: vaultID, ID: id})
		if err != nil {
			return fmt.Errorf("delete link refusal %q of vault %q: %w", id, vaultID, err)
		}
		if deleted == 0 {
			return fmt.Errorf("%w: %q in vault %q", linkrefusals.ErrRefusalNotFound, id, vaultID)
		}
		return nil
	})
}

func (r *PostgresLinkRefusals) inVault(ctx context.Context, vaultID string, fn func(q *gen.Queries) error) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		if err := requireVisibleVault(ctx, q, vaultID); err != nil {
			return err
		}
		return fn(q)
	})
}

// requireVaultNotes devolve linkrefusals.ErrNoteOutsideVault quando noteA ou noteB
// não é nota do cofre vaultID que o ator enxerga.
func requireVaultNotes(ctx context.Context, q *gen.Queries, vaultID, noteA, noteB string) error {
	found, err := q.CountVaultNotes(ctx, gen.CountVaultNotesParams{VaultID: vaultID, NoteIds: []string{noteA, noteB}})
	if err != nil {
		return fmt.Errorf("check notes %q and %q in vault %q: %w", noteA, noteB, vaultID, err)
	}
	if found != 2 {
		return fmt.Errorf("%w: expected notes %q and %q in vault %q, found %d", linkrefusals.ErrNoteOutsideVault, noteA, noteB, vaultID, found)
	}
	return nil
}

func toRefusal(r gen.ListLinkRefusalsRow) linkrefusals.Refusal {
	return linkrefusals.Refusal{
		ID: r.ID, CreatedAt: r.CreatedAt,
		Notes: [2]notes.NoteSummary{notes.Summarize(r.LowID, r.LowPath), notes.Summarize(r.HighID, r.HighPath)},
	}
}
