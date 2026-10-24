// Package storage persiste as notas em PostgreSQL e traduz os erros do driver
// nos erros de domínio do pacote notes.
package storage

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// SQLSTATEs do PostgreSQL traduzidos em erro de domínio.
const (
	uniqueViolationCode       = "23505"
	foreignKeyViolationCode   = "23503"
	insufficientPrivilegeCode = "42501"
	noDataFoundCode           = "P0002"
)

// PostgresNotesRepository persiste notas em PostgreSQL através das queries geradas
// pelo sqlc. Toda operação roda em transação do ator do contexto, sob RLS.
type PostgresNotesRepository struct {
	pool         *pgxpool.Pool
	searchConfig string
}

// NewPostgresNotesRepository monta o repositório sobre um pool já aberto. A busca
// usa a configuração de texto searchConfig, a mesma do indexador.
func NewPostgresNotesRepository(pool *pgxpool.Pool, searchConfig string) *PostgresNotesRepository {
	return &PostgresNotesRepository{pool: pool, searchConfig: searchConfig}
}

func (r *PostgresNotesRepository) run(ctx context.Context, fn func(q *gen.Queries) error) error {
	return inActorTx(ctx, r.pool, fn)
}

// Save insere a nota no cofre note.VaultID e devolve a linha gravada, devolvendo
// notes.ErrDuplicatePath se o path já existir no cofre e notes.ErrVaultForbidden se
// o ator não for membro dele.
func (r *PostgresNotesRepository) Save(ctx context.Context, note notes.Note) (notes.Note, error) {
	p := gen.CreateNoteParams{ID: note.ID, VaultID: note.VaultID, Path: note.Path, Content: note.Content, CreatedAt: note.CreatedAt, UpdatedAt: note.UpdatedAt}
	var row gen.Note
	err := r.run(ctx, func(q *gen.Queries) error {
		var qErr error
		row, qErr = q.CreateNote(ctx, p)
		return qErr
	})
	if err != nil {
		return notes.Note{}, mapWriteErr(err)
	}
	return toNote(row), nil
}

// DefaultVaultID devolve o cofre mais antigo do ator, ou notes.ErrNoVault quando ele
// não é membro de nenhum.
func (r *PostgresNotesRepository) DefaultVaultID(ctx context.Context) (string, error) {
	var vaultID string
	err := r.run(ctx, func(q *gen.Queries) error {
		var qErr error
		vaultID, qErr = q.DefaultVaultID(ctx)
		return qErr
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", notes.ErrNoVault
	}
	return vaultID, err
}

// Update grava a nota. Com force falso a escrita é condicionada a
// lastModifiedAt e devolve notes.ErrConflict se a versão não bater. A nota volta com
// a organização derivada que o último segundo estágio gravou.
func (r *PostgresNotesRepository) Update(ctx context.Context, id, path, content string, updatedAt, lastModifiedAt time.Time, force bool) (notes.Note, error) {
	p := gen.UpdateNoteWithVersionCheckParams{ID: id, Path: path, Content: content, UpdatedAt: updatedAt, PrevUpdatedAt: lastModifiedAt}
	var note notes.Note
	err := r.run(ctx, func(q *gen.Queries) error {
		row, qErr := updateNote(ctx, q, p, force)
		if qErr != nil {
			return qErr
		}
		note, qErr = withRank(ctx, q, toNote(row))
		return qErr
	})
	if err != nil {
		return notes.Note{}, mapWriteErr(err)
	}
	return note, nil
}

func updateNote(ctx context.Context, q *gen.Queries, p gen.UpdateNoteWithVersionCheckParams, force bool) (gen.Note, error) {
	if force {
		row, err := q.UpdateNoteForced(ctx, gen.UpdateNoteForcedParams{ID: p.ID, Path: p.Path, Content: p.Content, UpdatedAt: p.UpdatedAt})
		return row, notFoundOnNoRows(err)
	}
	row, err := q.UpdateNoteWithVersionCheck(ctx, p)
	if !errors.Is(err, pgx.ErrNoRows) {
		return row, err
	}
	return row, classifyMissedUpdate(ctx, q, p.ID)
}

// classifyMissedUpdate distingue a nota que não existe para o ator da nota que
// mudou desde a versão que o cliente leu.
func classifyMissedUpdate(ctx context.Context, q *gen.Queries, id string) error {
	if _, err := q.GetNoteByID(ctx, id); err != nil {
		return notFoundOnNoRows(err)
	}
	return notes.ErrConflict
}

// Delete remove a nota de id, devolvendo notes.ErrNotFound se ela não existir.
func (r *PostgresNotesRepository) Delete(ctx context.Context, id string) error {
	return r.run(ctx, func(q *gen.Queries) error {
		rows, err := q.DeleteNote(ctx, id)
		if err == nil && rows == 0 {
			return notes.ErrNotFound
		}
		return err
	})
}

// GetNoteByID lê a nota de id com a organização derivada dela, devolvendo
// notes.ErrNotFound se ela não existir.
func (r *PostgresNotesRepository) GetNoteByID(ctx context.Context, id string) (notes.Note, error) {
	var note notes.Note
	err := r.run(ctx, func(q *gen.Queries) error {
		row, qErr := q.GetNoteByID(ctx, id)
		if qErr != nil {
			return notFoundOnNoRows(qErr)
		}
		note, qErr = withRank(ctx, q, toNote(row))
		return qErr
	})
	return note, err
}

// SearchNotes devolve as notas indexadas do cofre vaultID, ou de todos os cofres do
// ator quando vazio, que casam com query, na sintaxe de websearch_to_tsquery, da mais
// relevante para a menos, paginadas por offset e limit; notes.ErrVaultForbidden
// quando o ator não enxerga o cofre.
func (r *PostgresNotesRepository) SearchNotes(ctx context.Context, query, vaultID string, offset, limit int32) ([]notes.SearchResult, error) {
	var rows []gen.SearchNotesRow
	err := r.run(ctx, func(q *gen.Queries) error {
		if vaultID != "" {
			if err := requireVisibleVault(ctx, q, vaultID); err != nil {
				return err
			}
		}
		var qErr error
		rows, qErr = q.SearchNotes(ctx, gen.SearchNotesParams{SearchConfig: r.searchConfig, Query: query, VaultID: vaultID, PageLimit: limit, PageOffset: offset})
		return qErr
	})
	return convertRows(rows, toSearchResult), err
}

func toSearchResult(row gen.SearchNotesRow) notes.SearchResult {
	return notes.SearchResult{ID: row.ID, VaultID: row.VaultID, Path: row.Path, UpdatedAt: row.UpdatedAt, Snippet: row.Snippet, Rank: row.Rank}
}

// ListNotes devolve todas as notas dos cofres do ator.
func (r *PostgresNotesRepository) ListNotes(ctx context.Context) ([]notes.Note, error) {
	var rows []gen.Note
	err := r.run(ctx, func(q *gen.Queries) error {
		var qErr error
		rows, qErr = q.ListNotes(ctx)
		return qErr
	})
	out := make([]notes.Note, 0, len(rows))
	for _, row := range rows {
		out = append(out, toNote(row))
	}
	return out, err
}

func toNote(row gen.Note) notes.Note {
	return notes.Note{
		ID:            row.ID,
		VaultID:       row.VaultID,
		Path:          row.Path,
		Content:       row.Content,
		ContentSHA256: hex.EncodeToString(row.ContentSha256),
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func notFoundOnNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notes.ErrNotFound
	}
	return err
}

// mapWriteErr traduz a escrita recusada pelo banco: caminho repetido no cofre, ou
// cofre fora do alcance do ator, barrado pela política de RLS (42501) ou pela chave
// do cofre no tenant. O SQLSTATE basta: a mensagem muda com o lc_messages do servidor.
func mapWriteErr(err error) error {
	switch pgErrorCode(err) {
	case uniqueViolationCode:
		return notes.ErrDuplicatePath
	case foreignKeyViolationCode, insufficientPrivilegeCode:
		return notes.ErrVaultForbidden
	}
	return err
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return ""
	}
	return pgErr.Code
}

func isUniqueConstraintViolation(err error) bool {
	return pgErrorCode(err) == uniqueViolationCode
}
