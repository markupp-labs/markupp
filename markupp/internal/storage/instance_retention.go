package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresInstanceRetention guarda o prazo de guarda do histórico e roda a limpeza
// (ADR-0061). Vale acima dos tenants, então roda como o dono do banco, sem ator.
type PostgresInstanceRetention struct {
	pool *pgxpool.Pool
}

var _ instance.RetentionStore = (*PostgresInstanceRetention)(nil)

// NewInstanceRetention monta o repositório sobre um pool aberto.
func NewInstanceRetention(pool *pgxpool.Pool) *PostgresInstanceRetention {
	return &PostgresInstanceRetention{pool: pool}
}

// LoadRetentionDays devolve a escolha do administrador, zero sem escolha.
func (r *PostgresInstanceRetention) LoadRetentionDays(ctx context.Context) (int, error) {
	days, err := gen.New(r.pool).GetInstanceRetentionDays(ctx)
	if err != nil {
		return 0, fmt.Errorf("read retention: %w", err)
	}
	return int(days), nil
}

// SaveRetentionDays guarda a escolha e audita, na mesma transação.
func (r *PostgresInstanceRetention) SaveRetentionDays(ctx context.Context, days int, event auth.AuditEventRecord) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		stored := pgtype.Int4{Int32: int32(days), Valid: true} //nolint:gosec // G115: o serviço limita a 3650
		if err := q.SetInstanceRetentionDays(ctx, stored); err != nil {
			return fmt.Errorf("save retention of %d days: %w", days, err)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(ctx, event))
	})
}

// CleanupIfDue limpa numa transação só se pegar a trava e a última limpeza estiver vencida.
func (r *PostgresInstanceRetention) CleanupIfDue(ctx context.Context, c instance.RetentionCleanup) (instance.RetentionResult, error) {
	var result instance.RetentionResult
	err := inTx(ctx, r.pool, func(q *gen.Queries) error {
		due, err := claimCleanup(ctx, q, c)
		if err != nil || !due {
			return err
		}
		result, err = deleteExpired(ctx, q, c)
		return err
	})
	if err != nil {
		return instance.RetentionResult{}, fmt.Errorf("clean up history before %s: %w", c.Cutoff, err)
	}
	return result, nil
}

// claimCleanup diz se esta réplica limpa agora: pegou a trava e marcou a limpeza vencida.
func claimCleanup(ctx context.Context, q *gen.Queries, c instance.RetentionCleanup) (bool, error) {
	locked, err := q.TryRetentionCleanupLock(ctx)
	if err != nil || !locked {
		return false, err
	}
	marked, err := q.MarkRetentionCleanupIfDue(ctx, gen.MarkRetentionCleanupIfDueParams{Now: c.Now, DueAfter: c.DueAfter})
	return marked > 0, err
}

func deleteExpired(ctx context.Context, q *gen.Queries, c instance.RetentionCleanup) (instance.RetentionResult, error) {
	if err := q.EnableAuditRetention(ctx); err != nil {
		return instance.RetentionResult{}, err
	}
	events, err := q.DeleteAuditEventsBefore(ctx, c.Cutoff)
	if err != nil {
		return instance.RetentionResult{}, err
	}
	cutoff := pgtype.Timestamptz{Time: c.Cutoff, Valid: true}
	sessions, err := q.ClearEndedSessionOrigins(ctx, cutoff)
	if err != nil {
		return instance.RetentionResult{}, err
	}
	apps, err := q.ClearRevokedAuthorizationOrigins(ctx, cutoff)
	if err != nil {
		return instance.RetentionResult{}, err
	}
	return instance.RetentionResult{Ran: true, AuditEvents: events, Origins: sessions + apps}, nil
}
