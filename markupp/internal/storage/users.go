// Package storage persiste entidades de usuários e convites em PostgreSQL.
package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/users"
)

// PostgresUserRepository persiste contas de usuários e convites no PostgreSQL.
type PostgresUserRepository struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

// NewPostgresUserRepository monta o repositório de usuários sobre o pool de conexões.
func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{
		pool: pool,
		q:    gen.New(pool),
	}
}

// CreateUser insere um novo usuário no banco.
func (r *PostgresUserRepository) CreateUser(ctx context.Context, u users.User) error {
	err := r.q.CreateUser(ctx, gen.CreateUserParams{
		ID:        u.ID,
		TenantID:  u.TenantID,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	})
	if err == nil {
		return nil
	}
	if isUniqueConstraintViolation(err) {
		return users.ErrUserAlreadyExists
	}
	return err
}

// CountUsersByTenant conta a quantidade de usuários existentes para o tenant.
func (r *PostgresUserRepository) CountUsersByTenant(ctx context.Context, tenantID string) (int64, error) {
	return r.q.CountUsersByTenant(ctx, tenantID)
}

// GetUserByID busca um usuário pelo ID.
func (r *PostgresUserRepository) GetUserByID(ctx context.Context, id string) (users.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return users.User{}, users.ErrUserNotFound
		}
		return users.User{}, err
	}
	return toUserDomain(row), nil
}

// GetUserByEmail busca um usuário por tenant e email.
func (r *PostgresUserRepository) GetUserByEmail(ctx context.Context, tenantID, email string) (users.User, error) {
	row, err := r.q.GetUserByTenantAndEmail(ctx, gen.GetUserByTenantAndEmailParams{
		TenantID: tenantID,
		Email:    email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return users.User{}, users.ErrUserNotFound
		}
		return users.User{}, err
	}
	return toUserDomain(row), nil
}

// ListUsers lista todos os usuários de um tenant.
func (r *PostgresUserRepository) ListUsers(ctx context.Context, tenantID string) ([]users.User, error) {
	rows, err := r.q.ListUsersByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]users.User, 0, len(rows))
	for _, row := range rows {
		out = append(out, toUserDomain(row))
	}
	return out, nil
}

// CreateInvite registra um novo convite de usuário.
func (r *PostgresUserRepository) CreateInvite(ctx context.Context, inv users.UserInvite) error {
	err := r.q.CreateUserInvite(ctx, gen.CreateUserInviteParams{
		ID:        inv.ID,
		TenantID:  inv.TenantID,
		Email:     inv.Email,
		Role:      inv.Role,
		InvitedBy: inv.InvitedBy,
		Status:    inv.Status,
		CreatedAt: inv.CreatedAt,
	})
	if err == nil {
		return nil
	}
	if isUniqueConstraintViolation(err) {
		return users.ErrInviteAlreadyExists
	}
	return err
}

// GetInviteByEmail busca um convite por tenant e email.
func (r *PostgresUserRepository) GetInviteByEmail(ctx context.Context, tenantID, email string) (users.UserInvite, error) {
	row, err := r.q.GetUserInviteByTenantAndEmail(ctx, gen.GetUserInviteByTenantAndEmailParams{
		TenantID: tenantID,
		Email:    email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return users.UserInvite{}, users.ErrInviteNotFound
		}
		return users.UserInvite{}, err
	}
	return users.UserInvite{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Email:     row.Email,
		Role:      row.Role,
		InvitedBy: row.InvitedBy,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
	}, nil
}

// ListInvites lista os convites de um tenant.
func (r *PostgresUserRepository) ListInvites(ctx context.Context, tenantID string) ([]users.UserInvite, error) {
	rows, err := r.q.ListUserInvitesByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]users.UserInvite, 0, len(rows))
	for _, row := range rows {
		out = append(out, users.UserInvite{
			ID:        row.ID,
			TenantID:  row.TenantID,
			Email:     row.Email,
			Role:      row.Role,
			InvitedBy: row.InvitedBy,
			Status:    row.Status,
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

// UpdateInviteStatus atualiza o status de um convite.
func (r *PostgresUserRepository) UpdateInviteStatus(ctx context.Context, tenantID, email, status string) error {
	rows, err := r.q.UpdateUserInviteStatus(ctx, gen.UpdateUserInviteStatusParams{
		TenantID: tenantID,
		Email:    email,
		Status:   status,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return users.ErrInviteNotFound
	}
	return nil
}

func toUserDomain(row gen.User) users.User {
	return users.User{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Email:     row.Email,
		Role:      row.Role,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
