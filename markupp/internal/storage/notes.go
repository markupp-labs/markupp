// Package storage persiste as notas em SQLite e traduz os erros do driver
// nos erros de domínio do pacote notes.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sqlite "modernc.org/sqlite"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

const sqliteUniqueConstraintCode = 2067

// SqliteNotesRepository persiste notas em SQLite através das queries geradas
// pelo sqlc.
type SqliteNotesRepository struct {
	q *gen.Queries
}

// NewSqliteNotesRepository monta o repositório sobre uma conexão já aberta.
func NewSqliteNotesRepository(db *sql.DB) *SqliteNotesRepository {
	return &SqliteNotesRepository{q: gen.New(db)}
}

// Save insere a nota, devolvendo notes.ErrDuplicatePath se o path já existir.
func (r *SqliteNotesRepository) Save(ctx context.Context, note notes.Note) error {
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
func (r *SqliteNotesRepository) Update(ctx context.Context, id, path, content string, updatedAt, lastModifiedAt time.Time, force bool) (notes.Note, error) {
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
		if errors.Is(err, sql.ErrNoRows) {
			if !force {
				_, checkErr := r.q.GetNoteByID(ctx, id)
				if errors.Is(checkErr, sql.ErrNoRows) {
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
func (r *SqliteNotesRepository) Delete(ctx context.Context, id string) error {
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
func (r *SqliteNotesRepository) GetNoteByID(ctx context.Context, id string) (notes.Note, error) {
	row, err := r.q.GetNoteByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
func (r *SqliteNotesRepository) SearchNotes(ctx context.Context, query string, offset, limit int32) ([]notes.SearchResult, error) {
	rows, err := r.q.SearchNotes(ctx, gen.SearchNotesParams{
		Content: "%" + query + "%",
		Limit:   int64(limit),
		Offset:  int64(offset),
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
func (r *SqliteNotesRepository) ListNotes(ctx context.Context) ([]notes.Note, error) {
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
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code() == sqliteUniqueConstraintCode
}
