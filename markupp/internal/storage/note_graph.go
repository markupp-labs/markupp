package storage

import (
	"context"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresNoteGraph lê o grafo de ligações derivado pelo indexador, resolvendo cada
// ligação pelos títulos do cofre no momento da leitura. Toda leitura roda em
// transação do ator do contexto, sob RLS.
type PostgresNoteGraph struct {
	pool *pgxpool.Pool
}

// NewPostgresNoteGraph monta a leitura do grafo sobre um pool já aberto.
func NewPostgresNoteGraph(pool *pgxpool.Pool) *PostgresNoteGraph {
	return &PostgresNoteGraph{pool: pool}
}

// Backlinks devolve as notas que ligam à nota noteID, por caminho, ou
// notes.ErrNotFound quando o ator não enxerga a nota.
func (g *PostgresNoteGraph) Backlinks(ctx context.Context, noteID string, offset, limit int) ([]notes.LinkedNote, error) {
	var out []notes.LinkedNote
	err := g.inNote(ctx, noteID, func(q *gen.Queries) error {
		rows, err := q.ListBacklinks(ctx, gen.ListBacklinksParams{NoteID: noteID, PageOffset: toInt32(offset), PageLimit: toInt32(limit)})
		out = convertRows(rows, func(r gen.ListBacklinksRow) notes.LinkedNote { return notes.LinkedNote(r) })
		return err
	})
	return out, err
}

// Neighbors devolve as notas ligadas à nota noteID em qualquer sentido, por
// caminho, ou notes.ErrNotFound quando o ator não enxerga a nota.
func (g *PostgresNoteGraph) Neighbors(ctx context.Context, noteID string, offset, limit int) ([]notes.Neighbor, error) {
	var out []notes.Neighbor
	err := g.inNote(ctx, noteID, func(q *gen.Queries) error {
		rows, err := q.ListNeighbors(ctx, gen.ListNeighborsParams{NoteID: noteID, PageOffset: toInt32(offset), PageLimit: toInt32(limit)})
		out = convertRows(rows, toNeighbor)
		return err
	})
	return out, err
}

// Orphans devolve as notas indexadas do cofre vaultID sem ligação em sentido algum,
// por caminho, ou notes.ErrVaultForbidden quando o ator não enxerga o cofre.
func (g *PostgresNoteGraph) Orphans(ctx context.Context, vaultID string, offset, limit int) ([]notes.LinkedNote, error) {
	var out []notes.LinkedNote
	err := g.inVault(ctx, vaultID, func(q *gen.Queries) error {
		rows, err := q.ListOrphans(ctx, gen.ListOrphansParams{VaultID: vaultID, PageOffset: toInt32(offset), PageLimit: toInt32(limit)})
		out = convertRows(rows, func(r gen.ListOrphansRow) notes.LinkedNote { return notes.LinkedNote(r) })
		return err
	})
	return out, err
}

// UnresolvedLinks devolve as ligações do cofre vaultID para títulos sem nota, por
// caminho da nota que liga, ou notes.ErrVaultForbidden quando o ator não enxerga o
// cofre.
func (g *PostgresNoteGraph) UnresolvedLinks(ctx context.Context, vaultID string, offset, limit int) ([]notes.UnresolvedLink, error) {
	var out []notes.UnresolvedLink
	err := g.inVault(ctx, vaultID, func(q *gen.Queries) error {
		rows, err := q.ListUnresolvedLinks(ctx, gen.ListUnresolvedLinksParams{VaultID: vaultID, PageOffset: toInt32(offset), PageLimit: toInt32(limit)})
		out = convertRows(rows, toUnresolvedLink)
		return err
	})
	return out, err
}

// inNote roda read depois de conferir que o ator enxerga a nota noteID.
func (g *PostgresNoteGraph) inNote(ctx context.Context, noteID string, read func(q *gen.Queries) error) error {
	return inActorTx(ctx, g.pool, func(q *gen.Queries) error {
		if _, err := q.GetNoteByID(ctx, noteID); err != nil {
			return notFoundOnNoRows(err)
		}
		return read(q)
	})
}

// inVault roda read depois de conferir que o ator enxerga o cofre vaultID.
func (g *PostgresNoteGraph) inVault(ctx context.Context, vaultID string, read func(q *gen.Queries) error) error {
	return inActorTx(ctx, g.pool, func(q *gen.Queries) error {
		visible, err := q.VaultVisible(ctx, vaultID)
		if err != nil {
			return err
		}
		if !visible {
			return notes.ErrVaultForbidden
		}
		return read(q)
	})
}

func toNeighbor(r gen.ListNeighborsRow) notes.Neighbor {
	return notes.Neighbor{
		LinkedNote: notes.LinkedNote{ID: r.ID, VaultID: r.VaultID, Path: r.Path, UpdatedAt: r.UpdatedAt},
		Incoming:   r.Incoming,
		Outgoing:   r.Outgoing,
	}
}

func toUnresolvedLink(r gen.ListUnresolvedLinksRow) notes.UnresolvedLink {
	return notes.UnresolvedLink{NoteID: r.NoteID, Path: r.Path, Title: r.CitedTitle, Kind: r.Kind}
}

// convertRows converte as linhas do sqlc no tipo de domínio.
func convertRows[R, T any](rows []R, convert func(R) T) []T {
	out := make([]T, 0, len(rows))
	for _, row := range rows {
		out = append(out, convert(row))
	}
	return out
}

// toInt32 satura n no intervalo de int32; a API já limita offset e limit bem abaixo.
func toInt32(n int) int32 {
	return int32(min(max(n, math.MinInt32), math.MaxInt32))
}
