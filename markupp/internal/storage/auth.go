// Package storage persiste entidades de autenticação, contas e auditoria.
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

// PostgresAuthRepository persiste contas, identidades, cofres e auditoria em PostgreSQL.
type PostgresAuthRepository struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

// NewPostgresAuthRepository monta o repositório de autenticação sobre um pool aberto.
func NewPostgresAuthRepository(pool *pgxpool.Pool) *PostgresAuthRepository {
	return &PostgresAuthRepository{
		pool: pool,
		q:    gen.New(pool),
	}
}

// CountUsers devolve a quantidade total de usuários cadastrados.
func (r *PostgresAuthRepository) CountUsers(ctx context.Context) (int64, error) {
	return r.q.CountUsers(ctx)
}

// CreateAccountTx cria atomicamente usuário, identidade, cofre padrão e auditoria.
func (r *PostgresAuthRepository) CreateAccountTx(ctx context.Context, p auth.CreateAccountParams) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)
	if err := insertAccountRows(ctx, qtx, p); err != nil {
		if isUniqueConstraintViolation(err) {
			return auth.ErrUserAlreadyExists
		}
		return fmt.Errorf("gravar conta: %w", err)
	}
	return tx.Commit(ctx)
}

func insertAccountRows(ctx context.Context, q *gen.Queries, p auth.CreateAccountParams) error {
	if err := q.CreateUser(ctx, toCreateUserParams(p.User)); err != nil {
		return err
	}
	if err := q.CreateUserIdentity(ctx, toCreateIdentityParams(p.Identity)); err != nil {
		return err
	}
	if err := q.CreateVault(ctx, gen.CreateVaultParams{
		ID:        p.InitialVaultID,
		TenantID:  p.User.TenantID,
		Name:      p.InitialVaultName,
		CreatedAt: p.User.CreatedAt,
	}); err != nil {
		return err
	}
	if err := q.AddVaultMember(ctx, gen.AddVaultMemberParams{
		VaultID:   p.InitialVaultID,
		UserID:    p.User.ID,
		CreatedAt: p.User.CreatedAt,
	}); err != nil {
		return err
	}
	return q.CreateAuditEvent(ctx, toCreateAuditEventParams(p.AuditEvent))
}

// GetUserByID busca a conta por ID, retornando ErrUserNotFound se inexistente.
func (r *PostgresAuthRepository) GetUserByID(ctx context.Context, id string) (auth.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, auth.ErrUserNotFound
		}
		return auth.User{}, err
	}
	return auth.User{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Email:     row.Email,
		Role:      row.Role,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// GetUserByEmail busca conta por tenant e email.
func (r *PostgresAuthRepository) GetUserByEmail(ctx context.Context, tenantID, email string) (auth.User, error) {
	row, err := r.q.GetUserByEmail(ctx, gen.GetUserByEmailParams{
		TenantID: tenantID,
		Email:    email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, auth.ErrUserNotFound
		}
		return auth.User{}, err
	}
	return auth.User{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Email:     row.Email,
		Role:      row.Role,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

// GetUserIdentity busca o vínculo de autenticação externa ou credencial local.
func (r *PostgresAuthRepository) GetUserIdentity(ctx context.Context, provider, providerUserID string) (auth.UserIdentity, error) {
	row, err := r.q.GetUserIdentity(ctx, gen.GetUserIdentityParams{
		Provider:       provider,
		ProviderUserID: providerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.UserIdentity{}, auth.ErrIdentityNotFound
		}
		return auth.UserIdentity{}, err
	}
	var hash string
	if row.PasswordHash != nil {
		hash = *row.PasswordHash
	}
	return auth.UserIdentity{
		ID:             row.ID,
		UserID:         row.UserID,
		Provider:       row.Provider,
		ProviderUserID: row.ProviderUserID,
		PasswordHash:   hash,
		CreatedAt:      row.CreatedAt,
	}, nil
}

// SaveRefreshToken persiste o hash e a validade de um refresh token.
func (r *PostgresAuthRepository) SaveRefreshToken(ctx context.Context, rec auth.RefreshTokenRecord) error {
	return r.q.SaveRefreshToken(ctx, gen.SaveRefreshTokenParams{
		TokenHash: rec.TokenHash,
		UserID:    rec.UserID,
		ExpiresAt: rec.ExpiresAt,
		Revoked:   rec.Revoked,
		CreatedAt: rec.CreatedAt,
	})
}

// GetRefreshToken lê o registro de refresh token pelo seu hash.
func (r *PostgresAuthRepository) GetRefreshToken(ctx context.Context, tokenHash string) (auth.RefreshTokenRecord, error) {
	row, err := r.q.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.RefreshTokenRecord{}, auth.ErrRefreshTokenExpired
		}
		return auth.RefreshTokenRecord{}, err
	}
	return auth.RefreshTokenRecord{
		TokenHash: row.TokenHash,
		UserID:    row.UserID,
		ExpiresAt: row.ExpiresAt,
		Revoked:   row.Revoked,
		CreatedAt: row.CreatedAt,
	}, nil
}

// RevokeRefreshToken marca o refresh token como revogado.
func (r *PostgresAuthRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	rows, err := r.q.RevokeRefreshToken(ctx, tokenHash)
	if err != nil {
		return err
	}
	if rows == 0 {
		return auth.ErrRefreshTokenExpired
	}
	return nil
}

// RecordAuditEvent insere uma linha de auditoria no plano de registro.
func (r *PostgresAuthRepository) RecordAuditEvent(ctx context.Context, e auth.AuditEventRecord) error {
	return r.q.CreateAuditEvent(ctx, toCreateAuditEventParams(e))
}

func toCreateUserParams(u auth.User) gen.CreateUserParams {
	return gen.CreateUserParams{
		ID:        u.ID,
		TenantID:  u.TenantID,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func toCreateIdentityParams(i auth.UserIdentity) gen.CreateUserIdentityParams {
	var pass *string
	if i.PasswordHash != "" {
		pass = &i.PasswordHash
	}
	return gen.CreateUserIdentityParams{
		ID:             i.ID,
		UserID:         i.UserID,
		Provider:       i.Provider,
		ProviderUserID: i.ProviderUserID,
		PasswordHash:   pass,
		CreatedAt:      i.CreatedAt,
	}
}

func toCreateAuditEventParams(e auth.AuditEventRecord) gen.CreateAuditEventParams {
	var actor *string
	if e.ActorID != "" {
		actor = &e.ActorID
	}
	return gen.CreateAuditEventParams{
		ID:         e.ID,
		TenantID:   e.TenantID,
		ActorID:    actor,
		Action:     e.Action,
		Target:     e.Target,
		Result:     e.Result,
		OccurredAt: e.OccurredAt,
	}
}
