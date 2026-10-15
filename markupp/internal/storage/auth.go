// Package storage persiste entidades de autenticação, contas e auditoria.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

const firstAdminLockKey = 422601

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

// CreateAccountTx cria atomicamente usuário, identidade, cofre padrão e auditoria com lock para primeiro admin.
func (r *PostgresAuthRepository) CreateAccountTx(ctx context.Context, p auth.CreateAccountParams) (auth.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("iniciar transacao para %q: %w", p.User.Email, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", firstAdminLockKey); err != nil {
		return auth.User{}, fmt.Errorf("adquirir lock de bootstrap para %q: %w", p.User.Email, err)
	}

	user, err := r.commitAccountTx(ctx, r.q.WithTx(tx), p)
	if err != nil {
		return auth.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, fmt.Errorf("comitar conta para %q: %w", p.User.Email, err)
	}
	return user, nil
}

func (r *PostgresAuthRepository) commitAccountTx(ctx context.Context, qtx *gen.Queries, p auth.CreateAccountParams) (auth.User, error) {
	user, action, err := r.resolveRoleAndAction(ctx, qtx, p)
	if err != nil {
		return auth.User{}, err
	}
	if err := insertAccountRows(ctx, qtx, user, p, action); err != nil {
		if isUniqueConstraintViolation(err) {
			return auth.User{}, auth.ErrUserAlreadyExists
		}
		return auth.User{}, fmt.Errorf("gravar conta para %q: %w", p.User.Email, err)
	}
	return user, nil
}

func (r *PostgresAuthRepository) resolveRoleAndAction(ctx context.Context, qtx *gen.Queries, p auth.CreateAccountParams) (auth.User, string, error) {
	userCount, err := qtx.CountUsers(ctx)
	if err != nil {
		return auth.User{}, "", fmt.Errorf("contar usuarios para %q: %w", p.User.Email, err)
	}
	user := p.User
	if userCount == 0 {
		user.Role = "admin"
		return user, "auth.register_admin", nil
	}
	if !p.AllowMemberRegistration {
		return auth.User{}, "", auth.ErrRegistrationDisabled
	}
	user.Role = "member"
	return user, "auth.register", nil
}

func insertAccountRows(ctx context.Context, q *gen.Queries, u auth.User, p auth.CreateAccountParams, action string) error {
	if err := q.CreateUser(ctx, toCreateUserParams(u)); err != nil {
		return err
	}
	if err := q.CreateUserIdentity(ctx, toCreateIdentityParams(p.Identity)); err != nil {
		return err
	}
	if err := insertInitialVault(ctx, q, p.InitialVaultID, u.TenantID, p.InitialVaultName, u.ID, u.CreatedAt); err != nil {
		return err
	}
	return q.CreateAuditEvent(ctx, gen.CreateAuditEventParams{
		ID:         p.AuditEventID,
		TenantID:   u.TenantID,
		ActorID:    pgtype.Text{String: u.ID, Valid: true},
		Action:     action,
		Target:     u.Email,
		Result:     "success",
		OccurredAt: u.CreatedAt,
	})
}

func insertInitialVault(ctx context.Context, q *gen.Queries, vaultID, tenantID, name, userID string, now time.Time) error {
	if err := q.CreateVault(ctx, gen.CreateVaultParams{
		ID:        vaultID,
		TenantID:  tenantID,
		Name:      name,
		CreatedAt: now,
	}); err != nil {
		return err
	}
	return q.AddVaultMember(ctx, gen.AddVaultMemberParams{
		VaultID:   vaultID,
		UserID:    userID,
		CreatedAt: now,
	})
}

// GetUserByID busca a conta por ID, retornando ErrUserNotFound se inexistente.
func (r *PostgresAuthRepository) GetUserByID(ctx context.Context, id string) (auth.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, auth.ErrUserNotFound
		}
		return auth.User{}, fmt.Errorf("buscar usuario %q: %w", id, err)
	}
	return toAuthUser(row), nil
}

func toAuthUser(row gen.User) auth.User {
	return auth.User{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Email:     row.Email,
		Role:      row.Role,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
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
		return auth.UserIdentity{}, fmt.Errorf("buscar identidade %s:%s: %w", provider, providerUserID, err)
	}
	return toAuthIdentity(row), nil
}

func toAuthIdentity(row gen.UserIdentity) auth.UserIdentity {
	return auth.UserIdentity{
		ID:             row.ID,
		UserID:         row.UserID,
		Provider:       row.Provider,
		ProviderUserID: row.ProviderUserID,
		PasswordHash:   row.PasswordHash.String,
		CreatedAt:      row.CreatedAt,
	}
}

// ConsumeRefreshToken consome atomicamente o refresh token se valido e nao expirado, retornando o userID.
func (r *PostgresAuthRepository) ConsumeRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	userID, err := r.q.ConsumeRefreshToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", auth.ErrRefreshTokenInvalid
		}
		return "", fmt.Errorf("consumir refresh token: %w", err)
	}
	return userID, nil
}

// IssueSessionTx grava o refresh token e a auditoria de login juntos na mesma transacao.
func (r *PostgresAuthRepository) IssueSessionTx(ctx context.Context, rec auth.RefreshTokenRecord, event auth.AuditEventRecord) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao de sessao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)
	if err := r.saveSessionRows(ctx, qtx, rec, event); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresAuthRepository) saveSessionRows(ctx context.Context, qtx *gen.Queries, rec auth.RefreshTokenRecord, event auth.AuditEventRecord) error {
	if err := qtx.SaveRefreshToken(ctx, gen.SaveRefreshTokenParams{
		TokenHash: rec.TokenHash,
		UserID:    rec.UserID,
		ExpiresAt: rec.ExpiresAt,
		Revoked:   rec.Revoked,
		CreatedAt: rec.CreatedAt,
	}); err != nil {
		return fmt.Errorf("salvar refresh token: %w", err)
	}
	if err := qtx.CreateAuditEvent(ctx, toCreateAuditEventParams(event)); err != nil {
		return fmt.Errorf("gravar auditoria de sessao: %w", err)
	}
	return nil
}

// GetRefreshToken lê o registro de refresh token pelo seu hash.
func (r *PostgresAuthRepository) GetRefreshToken(ctx context.Context, tokenHash string) (auth.RefreshTokenRecord, error) {
	row, err := r.q.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.RefreshTokenRecord{}, auth.ErrRefreshTokenInvalid
		}
		return auth.RefreshTokenRecord{}, fmt.Errorf("buscar refresh token: %w", err)
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
		return fmt.Errorf("revogar refresh token: %w", err)
	}
	if rows == 0 {
		return auth.ErrRefreshTokenInvalid
	}
	return nil
}

// RevokeAllUserRefreshTokens revoga todas as sessões ativas do usuário.
func (r *PostgresAuthRepository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	if err := r.q.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("revogar sessoes do usuario %q: %w", userID, err)
	}
	return nil
}

// RecordAuditEvent insere uma linha de auditoria no plano de registro.
func (r *PostgresAuthRepository) RecordAuditEvent(ctx context.Context, e auth.AuditEventRecord) error {
	if err := r.q.CreateAuditEvent(ctx, toCreateAuditEventParams(e)); err != nil {
		return fmt.Errorf("registrar auditoria para %q: %w", e.Target, err)
	}
	return nil
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
	return gen.CreateUserIdentityParams{
		ID:             i.ID,
		UserID:         i.UserID,
		Provider:       i.Provider,
		ProviderUserID: i.ProviderUserID,
		PasswordHash:   pgtype.Text{String: i.PasswordHash, Valid: i.PasswordHash != ""},
		CreatedAt:      i.CreatedAt,
	}
}

func toCreateAuditEventParams(e auth.AuditEventRecord) gen.CreateAuditEventParams {
	return gen.CreateAuditEventParams{
		ID:         e.ID,
		TenantID:   e.TenantID,
		ActorID:    pgtype.Text{String: e.ActorID, Valid: e.ActorID != ""},
		Action:     e.Action,
		Target:     e.Target,
		Result:     e.Result,
		OccurredAt: e.OccurredAt,
	}
}
