package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresAuditHistory lê o histórico para o administrador da instância (ADR-0061). O
// histórico vale acima dos tenants, então lê como o dono do banco, sem ator.
type PostgresAuditHistory struct {
	q *gen.Queries
}

// NewPostgresAuditHistory monta a leitura sobre o pool.
func NewPostgresAuditHistory(pool *pgxpool.Pool) *PostgresAuditHistory {
	return &PostgresAuditHistory{q: gen.New(pool)}
}

// ListAuditEvents lê os eventos do filtro, do mais novo para trás.
func (r *PostgresAuditHistory) ListAuditEvents(ctx context.Context, q auth.AuditQuery) ([]auth.AuditEntry, error) {
	rows, err := r.q.ListAuditEvents(ctx, toListAuditEventsParams(q))
	if err != nil {
		return nil, fmt.Errorf("list audit events with %+v: %w", q, err)
	}
	entries := make([]auth.AuditEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, auth.AuditEntry{ID: row.ID, ActorID: row.ActorID, ActorEmail: row.ActorEmail,
			Action: row.Action, Target: row.Target, Result: row.Result, IP: row.ClientIp, UserAgent: row.UserAgent,
			OccurredAt: row.OccurredAt})
	}
	return entries, nil
}

func toListAuditEventsParams(q auth.AuditQuery) gen.ListAuditEventsParams {
	return gen.ListAuditEventsParams{
		Account: q.Account, Action: q.Action, Result: q.Result, Ip: q.IP,
		HasSince: q.Since != nil, Since: timeOrZero(q.Since), HasUntil: q.Until != nil, Until: timeOrZero(q.Until),
		RowOffset: int32(q.Offset), RowLimit: int32(q.Limit), //nolint:gosec // G115: a API limita offset e limit
	}
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
