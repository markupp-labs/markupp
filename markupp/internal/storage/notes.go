// Package storage persiste as notas em PostgreSQL e traduz os erros do driver
// nos erros de domínio do pacote notes.
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// uniqueViolationCode é o SQLSTATE do PostgreSQL para violação de unicidade.
const uniqueViolationCode = "23505"

// PostgresNotesRepository persiste notas em PostgreSQL através das queries geradas
// pelo sqlc.
type PostgresNotesRepository struct {
	q *gen.Queries
}

// NewPostgresNotesRepository monta o repositório sobre um pool já aberto.
func NewPostgresNotesRepository(pool *pgxpool.Pool) *PostgresNotesRepository {
	return &PostgresNotesRepository{q: gen.New(pool)}
}

// Save insere a nota, devolvendo notes.ErrDuplicatePath se o path já existir.
func (r *PostgresNotesRepository) Save(ctx context.Context, note notes.Note) error {
	err := r.q.CreateNote(ctx, gen.CreateNoteParams{
		ID:        note.ID,
		Path:      note.Path,
		Content:   note.Content,
		CreatedAt: note.CreatedAt,
		UpdatedAt: note.UpdatedAt,
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
	var err error

	if force {
		row, err = r.q.UpdateNoteForced(ctx, gen.UpdateNoteForcedParams{
			ID:        id,
			Path:      path,
			Content:   content,
			UpdatedAt: updatedAt,
		})
	} else {
		row, err = r.q.UpdateNoteWithVersionCheck(ctx, gen.UpdateNoteWithVersionCheckParams{
			ID:            id,
			Path:          path,
			Content:       content,
			UpdatedAt:     updatedAt,
			PrevUpdatedAt: lastModifiedAt,
		})
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if !force {
				_, checkErr := r.q.GetNoteByID(ctx, id)
				if errors.Is(checkErr, pgx.ErrNoRows) {
					return notes.Note{}, notes.ErrNotFound
				}
				return notes.Note{}, notes.ErrConflict
			}
			return notes.Note{}, notes.ErrNotFound
		}
		if isUniqueConstraintViolation(err) {
			return notes.Note{}, notes.ErrDuplicatePath
		}
		return notes.Note{}, err
	}

	return notes.Note{
		ID:        row.ID,
		Path:      row.Path,
		Content:   row.Content,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// Delete remove a nota de id, devolvendo notes.ErrNotFound se ela não existir.
func (r *PostgresNotesRepository) Delete(ctx context.Context, id string) error {
	rows, err := r.q.DeleteNote(ctx, id)
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
	row, err := r.q.GetNoteByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notes.Note{}, notes.ErrNotFound
		}
		return notes.Note{}, err
	}
	return notes.Note{
		ID:        row.ID,
		Path:      row.Path,
		Content:   row.Content,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// SearchNotes devolve as notas cujo conteúdo casa com query, paginadas por
// offset e limit.
func (r *PostgresNotesRepository) SearchNotes(ctx context.Context, query string, offset, limit int32) ([]notes.SearchResult, error) {
	rows, err := r.q.SearchNotes(ctx, gen.SearchNotesParams{
		Content: "%" + query + "%",
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]notes.SearchResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, notes.SearchResult{
			ID:        row.ID,
			Path:      row.Path,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out, nil
}

// ListNotes devolve todas as notas.
func (r *PostgresNotesRepository) ListNotes(ctx context.Context) ([]notes.Note, error) {
	rows, err := r.q.ListNotes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]notes.Note, 0, len(rows))
	for _, row := range rows {
		out = append(out, notes.Note{
			ID:        row.ID,
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
