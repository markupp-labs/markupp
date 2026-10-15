package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

// ListMembers devolve os membros do tenant do ator, por email, com o papel de cada um.
func (r *PostgresAdminRepository) ListMembers(ctx context.Context) ([]auth.Member, error) {
	var rows []gen.ListMembersRow
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		var qErr error
		rows, qErr = q.ListMembers(ctx)
		return qErr
	})
	out := make([]auth.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, auth.Member{UserID: row.ID, Email: row.Email, Role: row.Role, JoinedAt: row.CreatedAt})
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
		InvitedBy: pgtype.Text{String: inv.InvitedBy, Valid: inv.InvitedBy != ""},
		ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt,
	}
}

func toInvite(row gen.ListInvitesRow) auth.Invite {
	inv := auth.Invite{
		ID: row.ID, TenantID: row.TenantID, Email: row.Email, Role: row.Role,
		InvitedBy: row.InvitedBy.String, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}
	if row.AcceptedAt.Valid {
		inv.AcceptedAt = &row.AcceptedAt.Time
	}
	return inv
}

// ChangeMembershipTx aplica a mudança de vínculo depois que check aprova o estado lido
// com trava (papel do alvo, papel de quem altera e owners) e audita junto.
func (r *PostgresAdminRepository) ChangeMembershipTx(ctx context.Context, c auth.MembershipChange, check func(auth.MembershipState) error) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		st, err := lockMembershipState(ctx, q, c)
		if err != nil {
			return err
		}
		if err := check(st); err != nil {
			return err
		}
		return applyMembershipChange(ctx, q, c)
	})
}

// lockMembershipState trava os owners, o vínculo do alvo e o de quem altera, para que
// nenhuma mudança simultânea invalide a regra entre a leitura e a escrita.
func lockMembershipState(ctx context.Context, q *gen.Queries, c auth.MembershipChange) (auth.MembershipState, error) {
	owners, err := q.LockTenantOwners(ctx)
	if err != nil {
		return auth.MembershipState{}, fmt.Errorf("travar owners do tenant: %w", err)
	}
	current, err := lockedRole(ctx, q, c.UserID, c.Event.TenantID)
	if err != nil {
		return auth.MembershipState{}, err
	}
	actorRole, err := lockedRole(ctx, q, c.Event.ActorID, c.Event.TenantID)
	if err != nil {
		return auth.MembershipState{}, err
	}
	return auth.MembershipState{Current: current, ActorRole: actorRole, Owners: len(owners)}, nil
}

func lockedRole(ctx context.Context, q *gen.Queries, userID, tenantID string) (string, error) {
	role, err := q.GetMemberRole(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w: usuario %q no tenant %q", auth.ErrMembershipNotFound, userID, tenantID)
	}
	if err != nil {
		return "", fmt.Errorf("ler papel de %q: %w", userID, err)
	}
	return role, nil
}

func applyMembershipChange(ctx context.Context, q *gen.Queries, c auth.MembershipChange) error {
	var err error
	if c.NewRole == "" {
		err = q.DeleteMember(ctx, c.UserID)
	} else {
		err = q.UpdateMemberRole(ctx, gen.UpdateMemberRoleParams{UserID: c.UserID, Role: c.NewRole})
	}
	if err != nil {
		return fmt.Errorf("mudar vinculo de %q para %q: %w", c.UserID, c.NewRole, err)
	}
	return q.CreateAuditEvent(ctx, toCreateAuditEventParams(c.Event))
}

// RevokeInviteTx apaga o convite ainda sem resposta, auditado junto, ou devolve
// auth.ErrInviteInvalid.
func (r *PostgresAdminRepository) RevokeInviteTx(ctx context.Context, inviteID string, event auth.AuditEventRecord) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		rows, err := q.RevokeInvite(ctx, inviteID)
		if err != nil {
			return fmt.Errorf("revogar convite %q: %w", inviteID, err)
		}
		if rows == 0 {
			return fmt.Errorf("%w: %q", auth.ErrInviteInvalid, inviteID)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}
