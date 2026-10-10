// Package storage persiste as notas em PostgreSQL e traduz os erros do driver
// nos erros de domínio do pacote notes.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/tenant"
)

// uniqueViolationCode é o SQLSTATE do PostgreSQL para violação de unicidade.
const uniqueViolationCode = "23505"

// PostgresNotesRepository persiste notas em PostgreSQL através das queries geradas
// pelo sqlc.
type PostgresNotesRepository struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

// NewPostgresNotesRepository monta o repositório sobre um pool já aberto.
func NewPostgresNotesRepository(pool *pgxpool.Pool) *PostgresNotesRepository {
	return &PostgresNotesRepository{
		pool: pool,
		q:    gen.New(pool),
	}
}

func (r *PostgresNotesRepository) withTenantTx(ctx context.Context, fn func(q *gen.Queries, tenantID string) error) error {
	tenantID := tenant.IDFromContext(ctx, "default")
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT set_config('markupp.current_tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("definir tenant_id: %w", err)
	}
	if err := fn(r.q.WithTx(tx), tenantID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Save insere a nota, devolvendo notes.ErrDuplicatePath se o path já existir.
func (r *PostgresNotesRepository) Save(ctx context.Context, note notes.Note) error {
	tenantID := note.TenantID
	if tenantID == "" {
		tenantID = tenant.IDFromContext(ctx, "default")
	}
	vaultID := note.VaultID
	if vaultID == "" {
		vaultID = "default"
	}

	err := r.withTenantTx(ctx, func(q *gen.Queries, _ string) error {
		return q.CreateNote(ctx, gen.CreateNoteParams{
			ID:        note.ID,
			TenantID:  tenantID,
			VaultID:   vaultID,
			Path:      note.Path,
			Content:   note.Content,
			CreatedAt: note.CreatedAt,
			UpdatedAt: note.UpdatedAt,
		})
	})
	if err == nil {
		return nil
	}
	if isUniqueConstraintViolation(err) {
		return notes.ErrDuplicatePath
	}
	return err
}

// Update grava a nota. Com force falso a escrita é condicionada a
// lastModifiedAt e devolve notes.ErrConflict se a versão não bater.
func (r *PostgresNotesRepository) Update(ctx context.Context, id, path, content string, updatedAt, lastModifiedAt time.Time, force bool) (notes.Note, error) {
	var row gen.Note
	err := r.withTenantTx(ctx, func(q *gen.Queries, tid string) error {
		var txErr error
		if force {
			row, txErr = q.UpdateNoteForced(ctx, gen.UpdateNoteForcedParams{
				Path:      path,
				Content:   content,
				UpdatedAt: updatedAt,
				ID:        id,
				TenantID:  tid,
			})
		} else {
			row, txErr = q.UpdateNoteWithVersionCheck(ctx, gen.UpdateNoteWithVersionCheckParams{
				Path:          path,
				Content:       content,
				UpdatedAt:     updatedAt,
				ID:            id,
				TenantID:      tid,
				PrevUpdatedAt: lastModifiedAt,
			})
		}
		if txErr != nil && errors.Is(txErr, pgx.ErrNoRows) && !force {
			_, checkErr := q.GetNoteByID(ctx, gen.GetNoteByIDParams{ID: id, TenantID: tid})
			if errors.Is(checkErr, pgx.ErrNoRows) {
				return notes.ErrNotFound
			}
			return notes.ErrConflict
		}
		return txErr
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notes.Note{}, notes.ErrNotFound
		}
		if errors.Is(err, notes.ErrNotFound) || errors.Is(err, notes.ErrConflict) {
			return notes.Note{}, err
		}
		if isUniqueConstraintViolation(err) {
			return notes.Note{}, notes.ErrDuplicatePath
		}
		return notes.Note{}, err
	}

	return notes.Note{
		ID:        row.ID,
		TenantID:  row.TenantID,
		VaultID:   row.VaultID,
		Path:      row.Path,
		Content:   row.Content,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// Delete remove a nota de id, devolvendo notes.ErrNotFound se ela não existir.
func (r *PostgresNotesRepository) Delete(ctx context.Context, id string) error {
	var rows int64
	err := r.withTenantTx(ctx, func(q *gen.Queries, tid string) error {
		var txErr error
		rows, txErr = q.DeleteNote(ctx, gen.DeleteNoteParams{ID: id, TenantID: tid})
		return txErr
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return notes.ErrNotFound
	}
	return nil
}

// GetNoteByID lê a nota de id, devolvendo notes.ErrNotFound se ela não existir.
func (r *PostgresNotesRepository) GetNoteByID(ctx context.Context, id string) (notes.Note, error) {
	var row gen.Note
	err := r.withTenantTx(ctx, func(q *gen.Queries, tid string) error {
		var txErr error
		row, txErr = q.GetNoteByID(ctx, gen.GetNoteByIDParams{ID: id, TenantID: tid})
		return txErr
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notes.Note{}, notes.ErrNotFound
		}
		return notes.Note{}, err
	}
	return notes.Note{
		ID:        row.ID,
		TenantID:  row.TenantID,
		VaultID:   row.VaultID,
		Path:      row.Path,
		Content:   row.Content,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// SearchNotes devolve as notas cujo conteúdo casa com query, paginadas por
// offset e limit.
func (r *PostgresNotesRepository) SearchNotes(ctx context.Context, query string, offset, limit int32) ([]notes.SearchResult, error) {
	var rows []gen.SearchNotesRow
	err := r.withTenantTx(ctx, func(q *gen.Queries, tid string) error {
		var txErr error
		rows, txErr = q.SearchNotes(ctx, gen.SearchNotesParams{
			TenantID: tid,
			Content:  "%" + query + "%",
			Limit:    limit,
			Offset:   offset,
		})
		return txErr
	})
	if err != nil {
		return nil, err
	}
	out := make([]notes.SearchResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, notes.SearchResult{
			ID:        row.ID,
			TenantID:  row.TenantID,
			VaultID:   row.VaultID,
			Path:      row.Path,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out, nil
}

// ListNotes devolve todas as notas do tenant.
func (r *PostgresNotesRepository) ListNotes(ctx context.Context) ([]notes.Note, error) {
	var rows []gen.Note
	err := r.withTenantTx(ctx, func(q *gen.Queries, tid string) error {
		var txErr error
		rows, txErr = q.ListNotes(ctx, tid)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	out := make([]notes.Note, 0, len(rows))
	for _, row := range rows {
		out = append(out, notes.Note{
			ID:        row.ID,
			TenantID:  row.TenantID,
			VaultID:   row.VaultID,
			Path:      row.Path,
			Content:   row.Content,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out, nil
}

func isUniqueConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == uniqueViolationCode
}
