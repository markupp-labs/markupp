package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaults"
)

// PostgresVaultsRepository persiste a gestão de cofres na transação do ator, sob RLS:
// só os cofres de que ele é membro no tenant dele existem para estas consultas.
type PostgresVaultsRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresVaultsRepository monta o repositório sobre um pool já aberto.
func NewPostgresVaultsRepository(pool *pgxpool.Pool) *PostgresVaultsRepository {
	return &PostgresVaultsRepository{pool: pool}
}

// List devolve os cofres do ator no tenant dele, por nome.
func (r *PostgresVaultsRepository) List(ctx context.Context) ([]vaults.Vault, error) {
	var rows []gen.ListVaultsRow
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		var qErr error
		rows, qErr = q.ListVaults(ctx)
		return qErr
	})
	out := make([]vaults.Vault, 0, len(rows))
	for _, row := range rows {
		out = append(out, vaults.Vault{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt})
	}
	return out, err
}

// CreateTx grava o cofre com o ator como primeiro membro, e a auditoria junto.
func (r *PostgresVaultsRepository) CreateTx(ctx context.Context, v vaults.Vault, event auth.AuditEventRecord) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		err := q.CreateMemberVault(ctx, gen.CreateMemberVaultParams{ID: v.ID, Name: v.Name, CreatedAt: v.CreatedAt})
		if err != nil {
			return fmt.Errorf("criar cofre %q: %w", v.Name, err)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

// RenameTx troca o nome do cofre id, ou devolve vaults.ErrVaultNotFound.
func (r *PostgresVaultsRepository) RenameTx(ctx context.Context, id, name string, event auth.AuditEventRecord) (vaults.Vault, error) {
	var v vaults.Vault
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		row, err := q.RenameVault(ctx, gen.RenameVaultParams{ID: id, Name: name})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %q", vaults.ErrVaultNotFound, id)
		}
		if err != nil {
			return fmt.Errorf("renomear cofre %q: %w", id, err)
		}
		v = vaults.Vault{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
	return v, err
}

// DeleteTx apaga o cofre id com as notas dele, ou devolve vaults.ErrVaultNotFound.
func (r *PostgresVaultsRepository) DeleteTx(ctx context.Context, id string, event auth.AuditEventRecord) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		n, err := q.DeleteVault(ctx, id)
		if err != nil {
			return fmt.Errorf("apagar cofre %q: %w", id, err)
		}
		if n == 0 {
			return fmt.Errorf("%w: %q", vaults.ErrVaultNotFound, id)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

// ListMembers devolve os membros do cofre vaultID, por email.
func (r *PostgresVaultsRepository) ListMembers(ctx context.Context, vaultID string) ([]vaults.Member, error) {
	var rows []gen.ListVaultMembersRow
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		if err := requireMemberVault(ctx, q, vaultID); err != nil {
			return err
		}
		var qErr error
		rows, qErr = q.ListVaultMembers(ctx, vaultID)
		return qErr
	})
	out := make([]vaults.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, toVaultMember(gen.GetVaultMemberRow(row)))
	}
	return out, err
}

// AddMemberTx põe a conta userID, que precisa ser do tenant, no cofre vaultID.
func (r *PostgresVaultsRepository) AddMemberTx(ctx context.Context, vaultID, userID string, at time.Time, event auth.AuditEventRecord) (vaults.Member, error) {
	var m vaults.Member
	err := inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		if err := requireAddableMember(ctx, q, vaultID, userID); err != nil {
			return err
		}
		if err := insertVaultMember(ctx, q, vaultID, userID, at); err != nil {
			return err
		}
		row, err := q.GetVaultMember(ctx, gen.GetVaultMemberParams{VaultID: vaultID, UserID: userID})
		m = toVaultMember(row)
		if err != nil {
			return fmt.Errorf("ler membro %q do cofre %q: %w", userID, vaultID, err)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
	return m, err
}

// RemoveMemberTx tira userID do cofre vaultID, ou devolve vaults.ErrMemberNotFound.
func (r *PostgresVaultsRepository) RemoveMemberTx(ctx context.Context, vaultID, userID string, event auth.AuditEventRecord) error {
	return inActorTx(ctx, r.pool, func(q *gen.Queries) error {
		if err := requireMemberVault(ctx, q, vaultID); err != nil {
			return err
		}
		n, err := q.RemoveVaultMember(ctx, gen.RemoveVaultMemberParams{VaultID: vaultID, UserID: userID})
		if err != nil {
			return fmt.Errorf("tirar %q do cofre %q: %w", userID, vaultID, err)
		}
		if n == 0 {
			return fmt.Errorf("%w: %q no cofre %q", vaults.ErrMemberNotFound, userID, vaultID)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(event))
	})
}

// requireMemberVault é requireVisibleVault no vocabulário da gestão de cofres.
func requireMemberVault(ctx context.Context, q *gen.Queries, vaultID string) error {
	err := requireVisibleVault(ctx, q, vaultID)
	if errors.Is(err, notes.ErrVaultForbidden) {
		return fmt.Errorf("%w: %q", vaults.ErrVaultNotFound, vaultID)
	}
	return err
}

func requireAddableMember(ctx context.Context, q *gen.Queries, vaultID, userID string) error {
	if err := requireMemberVault(ctx, q, vaultID); err != nil {
		return err
	}
	ok, err := q.IsTenantAccount(ctx, userID)
	if err != nil {
		return fmt.Errorf("conferir vinculo de %q: %w", userID, err)
	}
	if !ok {
		return fmt.Errorf("%w: %q", vaults.ErrNotTenantMember, userID)
	}
	return nil
}

func insertVaultMember(ctx context.Context, q *gen.Queries, vaultID, userID string, at time.Time) error {
	err := q.AddVaultMember(ctx, gen.AddVaultMemberParams{VaultID: vaultID, UserID: userID, CreatedAt: at})
	if isUniqueConstraintViolation(err) {
		return fmt.Errorf("%w: %q no cofre %q", vaults.ErrAlreadyVaultMember, userID, vaultID)
	}
	if err != nil {
		return fmt.Errorf("pôr %q no cofre %q: %w", userID, vaultID, err)
	}
	return nil
}

func toVaultMember(row gen.GetVaultMemberRow) vaults.Member {
	return vaults.Member{UserID: row.UserID, Email: row.Email, Role: row.Role, JoinedAt: row.CreatedAt}
}
