package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresAdminRepository persiste o que o administrador faz no tenant, sempre na
// transação do ator e sob RLS.
type PostgresAdminRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAdminRepository monta o repositório sobre um pool já aberto.
func NewPostgresAdminRepository(pool *pgxpool.Pool) *PostgresAdminRepository {
	return &PostgresAdminRepository{pool: pool}
}

// ListUsers devolve os usuários do tenant do ator, por email.
func (r *PostgresAdminRepository) ListUsers(ctx context.Context) ([]auth.User, error) {
	var rows []gen.User
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		var qErr error
		rows, qErr = q.ListTenantUsers(ctx)
		return qErr
	})
	out := make([]auth.User, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAuthUser(row))
	}
	return out, err
}

// ListInvites devolve os convites do tenant do ator, do mais recente ao mais antigo.
func (r *PostgresAdminRepository) ListInvites(ctx context.Context) ([]auth.Invite, error) {
	var rows []gen.ListInvitesRow
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		var qErr error
		rows, qErr = q.ListInvites(ctx)
		return qErr
	})
	out := make([]auth.Invite, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInvite(row))
	}
	return out, err
}

// CreateInviteTx grava o convite e a auditoria juntos, ou devolve
// auth.ErrAlreadyMember quando o email já é de um usuário do tenant.
func (r *PostgresAdminRepository) CreateInviteTx(ctx context.Context, inv auth.Invite, tokenHash string, event auth.AuditEventRecord) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		rows, err := q.CreateInvite(ctx, toCreateInviteParams(inv, tokenHash))
		if err != nil {
			return fmt.Errorf("inserir convite de %q: %w", inv.Email, err)
		}
		if rows == 0 {
			return fmt.Errorf("%w: %q", auth.ErrAlreadyMember, inv.Email)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

func toCreateInviteParams(inv auth.Invite, tokenHash string) gen.CreateInviteParams {
	return gen.CreateInviteParams{
		ID: inv.ID, TenantID: inv.TenantID, Email: inv.Email, Role: inv.Role, TokenHash: tokenHash,
		InvitedBy: inv.InvitedBy, ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt,
	}
}

func toInvite(row gen.ListInvitesRow) auth.Invite {
	inv := auth.Invite{
		ID: row.ID, TenantID: row.TenantID, Email: row.Email, Role: row.Role,
		InvitedBy: row.InvitedBy, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}
	if row.AcceptedAt.Valid {
		inv.AcceptedAt = &row.AcceptedAt.Time
	}
	return inv
}
