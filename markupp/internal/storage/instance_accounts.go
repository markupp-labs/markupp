package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// PostgresInstanceAccountsRepository é a gestão das contas pelo administrador da
// instância (ADR-0061). Ela vale acima dos tenants, então roda como o dono do banco, sem ator.
type PostgresInstanceAccountsRepository struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

// NewPostgresInstanceAccountsRepository monta o repositório sobre o pool.
func NewPostgresInstanceAccountsRepository(pool *pgxpool.Pool) *PostgresInstanceAccountsRepository {
	return &PostgresInstanceAccountsRepository{pool: pool, q: gen.New(pool)}
}

// ListAccounts lista as contas pelo filtro, da criada mais recentemente para a mais antiga.
func (r *PostgresInstanceAccountsRepository) ListAccounts(ctx context.Context, q auth.AccountQuery) ([]auth.AccountSummary, error) {
	rows, err := r.q.ListInstanceAccounts(ctx, gen.ListInstanceAccountsParams{
		Email: q.Email, Status: q.Status, RowOffset: int32(q.Offset), RowLimit: int32(q.Limit), //nolint:gosec // G115: a API limita offset e limit
	})
	if err != nil {
		return nil, fmt.Errorf("list instance accounts with %+v: %w", q, err)
	}
	accounts := make([]auth.AccountSummary, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, toAccountSummary(row))
	}
	return accounts, nil
}

func toAccountSummary(row gen.InstanceAccountSummary) auth.AccountSummary {
	summary := auth.AccountSummary{ID: row.ID, Email: row.Email, EmailVerified: row.EmailVerified,
		LoginMethods: row.LoginMethods, CreatedAt: row.CreatedAt, Blocked: row.Blocked}
	if row.HasLastAccess {
		at := row.LastAccessAt
		summary.LastAccessAt = &at
	}
	return summary
}

// GetAccountDetail devolve a conta com os tenants de que ela é a única owner.
func (r *PostgresInstanceAccountsRepository) GetAccountDetail(ctx context.Context, id string) (auth.AccountDetail, error) {
	row, err := r.q.GetInstanceAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.AccountDetail{}, fmt.Errorf("%w: %q", auth.ErrUserNotFound, id)
	}
	if err != nil {
		return auth.AccountDetail{}, fmt.Errorf("get instance account %q: %w", id, err)
	}
	tenants, err := r.soleOwnerTenants(ctx, id)
	if err != nil {
		return auth.AccountDetail{}, err
	}
	return auth.AccountDetail{AccountSummary: toAccountSummary(row), SoleOwnerTenants: tenants}, nil
}

func (r *PostgresInstanceAccountsRepository) soleOwnerTenants(ctx context.Context, id string) ([]auth.OwnedTenant, error) {
	rows, err := r.q.ListSoleOwnerTenants(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list tenants owned only by %q: %w", id, err)
	}
	tenants := make([]auth.OwnedTenant, 0, len(rows))
	for _, row := range rows {
		tenants = append(tenants, auth.OwnedTenant{ID: row.ID, Name: row.Name, Notes: row.Notes})
	}
	return tenants, nil
}

// SetAccountBlocked bloqueia ou desbloqueia a conta e audita; bloquear também encerra as
// sessões e revoga os apps autorizados, na mesma transação.
func (r *PostgresInstanceAccountsRepository) SetAccountBlocked(ctx context.Context, id string, blocked bool, event auth.AuditEventRecord) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		rows, err := q.SetUserBlocked(ctx, gen.SetUserBlockedParams{ID: id, Blocked: blocked})
		if err != nil {
			return fmt.Errorf("set blocked=%t on account %q: %w", blocked, id, err)
		}
		if rows == 0 {
			return fmt.Errorf("%w: %q", auth.ErrUserNotFound, id)
		}
		if blocked {
			if err := cutAccountAccess(ctx, q, id); err != nil {
				return err
			}
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

// cutAccountAccess encerra as sessões e revoga os apps autorizados da conta.
func cutAccountAccess(ctx context.Context, q *gen.Queries, id string) error {
	if err := q.RevokeAllUserRefreshTokens(ctx, id); err != nil {
		return fmt.Errorf("end sessions of account %q: %w", id, err)
	}
	if err := q.RevokeUserOAuthAuthorizations(ctx, id); err != nil {
		return fmt.Errorf("revoke authorized apps of account %q: %w", id, err)
	}
	return nil
}

// DeleteAccount apaga os tenants de que a conta é a única owner, depois a conta, e audita.
// As chaves estrangeiras levam o resto: vínculos, sessões, apps e o conteúdo dos tenants.
func (r *PostgresInstanceAccountsRepository) DeleteAccount(ctx context.Context, id string, event auth.AuditEventRecord) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		if err := q.DeleteSoleOwnerTenants(ctx, id); err != nil {
			return fmt.Errorf("delete tenants owned only by %q: %w", id, err)
		}
		rows, err := q.DeleteUser(ctx, id)
		if err != nil {
			return fmt.Errorf("delete account %q: %w", id, err)
		}
		if rows == 0 {
			return fmt.Errorf("%w: %q", auth.ErrUserNotFound, id)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}
