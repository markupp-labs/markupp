package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresInstanceIPBlocks guarda os bloqueios de IP da instância (ADR-0061). Vale acima
// dos tenants e audita sem tenant, então roda como o dono do banco, sem ator.
type PostgresInstanceIPBlocks struct {
	pool *pgxpool.Pool
}

var _ instance.IPBlockStore = (*PostgresInstanceIPBlocks)(nil)

// NewInstanceIPBlocks monta o repositório sobre um pool aberto.
func NewInstanceIPBlocks(pool *pgxpool.Pool) *PostgresInstanceIPBlocks {
	return &PostgresInstanceIPBlocks{pool: pool}
}

// ListIPBlocks devolve os bloqueios na ordem de criação.
func (r *PostgresInstanceIPBlocks) ListIPBlocks(ctx context.Context) ([]instance.IPBlock, error) {
	rows, err := gen.New(r.pool).ListInstanceIPBlocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("read instance ip blocks: %w", err)
	}
	blocks := make([]instance.IPBlock, 0, len(rows))
	for _, row := range rows {
		blocks = append(blocks, instance.IPBlock{ID: row.ID, Prefix: row.Cidr, Note: row.Note, CreatedAt: row.CreatedAt})
	}
	return blocks, nil
}

// CreateIPBlock grava o bloqueio e audita, e recusa a faixa já bloqueada.
func (r *PostgresInstanceIPBlocks) CreateIPBlock(ctx context.Context, block instance.IPBlock, event auth.AuditEventRecord) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		created, err := q.CreateInstanceIPBlock(ctx, gen.CreateInstanceIPBlockParams{
			ID: block.ID, Cidr: block.Prefix, Note: block.Note, CreatedAt: block.CreatedAt,
		})
		if err != nil {
			return fmt.Errorf("create ip block %s: %w", block.Prefix, err)
		}
		if created == 0 {
			return fmt.Errorf("%w: %s", instance.ErrIPBlockExists, block.Prefix)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(ctx, event))
	})
}

// DeleteIPBlock apaga o bloqueio e audita.
func (r *PostgresInstanceIPBlocks) DeleteIPBlock(ctx context.Context, id string, event auth.AuditEventRecord) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		deleted, err := q.DeleteInstanceIPBlock(ctx, id)
		if err != nil {
			return fmt.Errorf("delete ip block %q: %w", id, err)
		}
		if deleted == 0 {
			return fmt.Errorf("%w: %q", instance.ErrIPBlockNotFound, id)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(ctx, event))
	})
}
