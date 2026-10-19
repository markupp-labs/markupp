package storage

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresNoteGraph lê o grafo de ligações derivado pelo indexador, resolvendo cada
// ligação pelos títulos do cofre no momento da leitura. Toda leitura roda em
// transação do ator do contexto, sob RLS.
type PostgresNoteGraph struct {
	pool         *pgxpool.Pool
	searchConfig string
}

// NewPostgresNoteGraph monta a leitura do grafo sobre um pool já aberto; a menção não
// ligada consulta o índice léxico com searchConfig, a mesma do indexador.
func NewPostgresNoteGraph(pool *pgxpool.Pool, searchConfig string) *PostgresNoteGraph {
	return &PostgresNoteGraph{pool: pool, searchConfig: searchConfig}
}

// Links devolve as ligações que a nota noteID faz, ou notes.ErrNotFound quando o ator
// não enxerga a nota.
func (g *PostgresNoteGraph) Links(ctx context.Context, noteID string, f notes.LinkFilter) ([]notes.Link, error) {
	var out []notes.Link
	err := g.inNote(ctx, noteID, func(q *gen.Queries) error {
		rows, err := q.ListOutgoingLinks(ctx, gen.ListOutgoingLinksParams{NoteID: noteID, Kind: f.Type, PageOffset: toInt32(f.Offset), PageLimit: toInt32(f.Limit)})
		out = convertRows(rows, toLink)
		return err
	})
	return out, err
}

// Backlinks devolve as ligações resolvidas para a nota noteID, ou notes.ErrNotFound
// quando o ator não enxerga a nota.
func (g *PostgresNoteGraph) Backlinks(ctx context.Context, noteID string, f notes.LinkFilter) ([]notes.Link, error) {
	var out []notes.Link
	err := g.inNote(ctx, noteID, func(q *gen.Queries) error {
		rows, err := q.ListIncomingLinks(ctx, gen.ListIncomingLinksParams{NoteID: noteID, Kind: f.Type, PageOffset: toInt32(f.Offset), PageLimit: toInt32(f.Limit)})
		out = convertRows(rows, toLink)
		return err
	})
	return out, err
}

// VaultLinks devolve as ligações das notas do cofre vaultID, ou
// notes.ErrVaultForbidden quando o ator não enxerga o cofre.
func (g *PostgresNoteGraph) VaultLinks(ctx context.Context, vaultID string, vq notes.VaultLinkQuery) ([]notes.Link, error) {
	var out []notes.Link
	err := g.inVault(ctx, vaultID, func(q *gen.Queries) error {
		rows, err := q.ListVaultLinks(ctx, gen.ListVaultLinksParams{
			VaultID: vaultID, Kind: vq.Type, Status: vq.Status, PageOffset: toInt32(vq.Offset), PageLimit: toInt32(vq.Limit),
		})
		out = convertRows(rows, toLink)
		return err
	})
	return out, err
}

// Communities devolve as comunidades do cofre vaultID, das maiores para as menores,
// ou notes.ErrVaultForbidden quando o ator não enxerga o cofre.
func (g *PostgresNoteGraph) Communities(ctx context.Context, vaultID string, p notes.Page) ([]notes.Community, error) {
	var out []notes.Community
	err := g.inVault(ctx, vaultID, func(q *gen.Queries) error {
		rows, err := q.ListCommunities(ctx, gen.ListCommunitiesParams{VaultID: vaultID, PageOffset: toInt32(p.Offset), PageLimit: toInt32(p.Limit)})
		out = convertRows(rows, toCommunity)
		return err
	})
	return out, err
}

// Community devolve a comunidade communityID do cofre vaultID, notes.ErrCommunityNotFound
// quando o cofre não a tem ou notes.ErrVaultForbidden quando o ator não enxerga o cofre.
func (g *PostgresNoteGraph) Community(ctx context.Context, vaultID string, communityID int64) (notes.Community, error) {
	var out notes.Community
	err := g.inVault(ctx, vaultID, func(q *gen.Queries) error {
		rows, err := q.ListCommunities(ctx, gen.ListCommunitiesParams{
			VaultID: vaultID, CommunityID: pgtype.Int8{Int64: communityID, Valid: true}, PageLimit: 1,
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("%w: id %d no cofre %q", notes.ErrCommunityNotFound, communityID, vaultID)
		}
		out = toCommunity(rows[0])
		return nil
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
		if err := requireVisibleVault(ctx, q, vaultID); err != nil {
			return err
		}
		return read(q)
	})
}

// requireVisibleVault devolve notes.ErrVaultForbidden quando o ator não enxerga o
// cofre vaultID.
func requireVisibleVault(ctx context.Context, q *gen.Queries, vaultID string) error {
	visible, err := q.VaultVisible(ctx, vaultID)
	if err != nil {
		return err
	}
	if !visible {
		return notes.ErrVaultForbidden
	}
	return nil
}

func toLink(r gen.NoteLinkItem) notes.Link {
	link := notes.Link{
		Source: notes.Summarize(r.SourceID, r.SourcePath), CitedTitle: r.CitedTitle, Type: r.Kind,
		Target: optionalSummary(r.TargetID, r.TargetPath), InferredTarget: optionalSummary(r.InferredTargetID, r.InferredTargetPath),
	}
	if r.Predicate.Valid {
		link.Predicate = &r.Predicate.String
	}
	return link
}

// optionalSummary resume a nota que a coluna nula pode não trazer.
func optionalSummary(id, notePath pgtype.Text) *notes.NoteSummary {
	if !id.Valid {
		return nil
	}
	summary := notes.Summarize(id.String, notePath.String)
	return &summary
}

func toCommunity(r gen.ListCommunitiesRow) notes.Community {
	return notes.Community{ID: r.CommunityID, Size: int(r.Size), Hub: notes.Summarize(r.HubID, r.HubPath)}
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
