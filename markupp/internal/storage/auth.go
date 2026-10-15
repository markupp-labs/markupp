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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
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

// CreateAccountTx cria conta, identidade e auditoria sob o lock do primeiro usuário:
// só ele ganha o tenant inicial, como owner, com o cofre inicial nele.
func (r *PostgresAuthRepository) CreateAccountTx(ctx context.Context, p auth.CreateAccountParams) (auth.User, error) {
	err := r.inTx(ctx, func(qtx *gen.Queries) error {
		if err := qtx.LockFirstAdmin(ctx, firstAdminLockKey); err != nil {
			return fmt.Errorf("adquirir lock de bootstrap para %q: %w", p.User.Email, err)
		}
		return commitAccount(ctx, qtx, p)
	})
	if err != nil {
		return auth.User{}, err
	}
	return p.User, nil
}

func commitAccount(ctx context.Context, qtx *gen.Queries, p auth.CreateAccountParams) error {
	userCount, err := qtx.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("contar usuarios para %q: %w", p.User.Email, err)
	}
	if userCount > 0 && !p.AllowMemberRegistration {
		return auth.ErrRegistrationDisabled
	}
	if err := insertUserRows(ctx, qtx, p.User, p.Identity); err != nil {
		return err
	}
	if userCount > 0 {
		return qtx.CreateAuditEvent(ctx, accountEvent(p.AuditEventID, "", "auth.register", p.User))
	}
	return bootstrapFirstTenant(ctx, qtx, p)
}

// bootstrapFirstTenant entrega ao primeiro usuário da instância o tenant inicial, como
// owner, e os cofres que já existiam, que só podem ser os das notas da v1.0.0-rc.1.
func bootstrapFirstTenant(ctx context.Context, qtx *gen.Queries, p auth.CreateAccountParams) error {
	tenantID, u := p.BootstrapTenantID, p.User
	if err := qtx.EnsureTenant(ctx, gen.EnsureTenantParams{ID: tenantID, CreatedAt: u.CreatedAt}); err != nil {
		return fmt.Errorf("criar tenant %q do primeiro usuario: %w", tenantID, err)
	}
	join := tenantJoin{tenantID: tenantID, userID: u.ID, role: identity.RoleOwner, vaultID: p.InitialVaultID, vaultName: p.InitialVaultName, at: u.CreatedAt}
	if err := joinTenant(ctx, qtx, join); err != nil {
		return err
	}
	if err := adoptExistingVaults(ctx, qtx, tenantID, u); err != nil {
		return err
	}
	return qtx.CreateAuditEvent(ctx, accountEvent(p.AuditEventID, tenantID, "auth.register_admin", u))
}

// adoptExistingVaults move para o tenant do primeiro usuário os cofres da migração. O
// vínculo nasce 1 ms depois da conta para o cofre criado com ela continuar o padrão.
func adoptExistingVaults(ctx context.Context, qtx *gen.Queries, tenantID string, u auth.User) error {
	err := qtx.AdoptLegacyVaults(ctx, gen.AdoptLegacyVaultsParams{
		TenantID: tenantID, UserID: u.ID, CreatedAt: u.CreatedAt.Add(time.Millisecond),
	})
	if err != nil {
		return fmt.Errorf("dar ao usuario %q os cofres do tenant %q: %w", u.ID, tenantID, err)
	}
	return nil
}

func insertUserRows(ctx context.Context, q *gen.Queries, u auth.User, ident auth.UserIdentity) error {
	err := q.CreateUser(ctx, gen.CreateUserParams{ID: u.ID, Email: u.Email, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt})
	if err == nil {
		err = q.CreateUserIdentity(ctx, toCreateIdentityParams(ident))
	}
	if isUniqueConstraintViolation(err) {
		return fmt.Errorf("%w: %q", auth.ErrUserAlreadyExists, u.Email)
	}
	if err != nil {
		return fmt.Errorf("gravar conta para %q: %w", u.Email, err)
	}
	return nil
}

// tenantJoin é a entrada de um usuário num tenant: o vínculo com o papel e o cofre
// que ele ganha lá dentro.
type tenantJoin struct {
	tenantID, userID, role, vaultID, vaultName string
	at                                         time.Time
}

func joinTenant(ctx context.Context, q *gen.Queries, j tenantJoin) error {
	err := q.AddMembership(ctx, gen.AddMembershipParams{TenantID: j.tenantID, UserID: j.userID, Role: j.role, CreatedAt: j.at})
	if err != nil {
		return fmt.Errorf("vincular %q ao tenant %q como %s: %w", j.userID, j.tenantID, j.role, err)
	}
	return insertInitialVault(ctx, q, j)
}

func insertInitialVault(ctx context.Context, q *gen.Queries, j tenantJoin) error {
	err := q.CreateVault(ctx, gen.CreateVaultParams{ID: j.vaultID, TenantID: j.tenantID, Name: j.vaultName, CreatedAt: j.at})
	if err != nil {
		return fmt.Errorf("criar cofre de %q no tenant %q: %w", j.userID, j.tenantID, err)
	}
	return q.AddVaultMember(ctx, gen.AddVaultMemberParams{VaultID: j.vaultID, UserID: j.userID, CreatedAt: j.at})
}

func accountEvent(id, tenantID, action string, u auth.User) gen.CreateAuditEventParams {
	return toCreateAuditEventParams(auth.AuditEventRecord{
		ID: id, TenantID: tenantID, ActorID: u.ID, Action: action,
		Target: u.Email, Result: "success", OccurredAt: u.CreatedAt,
	})
}

// CreateTenantTx cria o tenant com o criador como owner, o cofre inicial dele e a
// auditoria, juntos.
func (r *PostgresAuthRepository) CreateTenantTx(ctx context.Context, p auth.CreateTenantParams) error {
	return r.inTx(ctx, func(qtx *gen.Queries) error {
		if err := qtx.CreateTenant(ctx, gen.CreateTenantParams{ID: p.TenantID, Name: p.Name, CreatedAt: p.Now}); err != nil {
			return fmt.Errorf("criar tenant %q: %w", p.Name, err)
		}
		join := tenantJoin{tenantID: p.TenantID, userID: p.UserID, role: identity.RoleOwner, vaultID: p.VaultID, vaultName: auth.InitialVaultName, at: p.Now}
		if err := joinTenant(ctx, qtx, join); err != nil {
			return err
		}
		return qtx.CreateAuditEvent(ctx, toCreateAuditEventParams(auth.AuditEventRecord{
			ID: p.AuditEventID, TenantID: p.TenantID, ActorID: p.UserID, Action: "tenants.create",
			Target: p.TenantID, Result: "success", OccurredAt: p.Now,
		}))
	})
}

// AcceptInviteTx consome o convite e cria a conta que ele descreve, já vinculada ao
// tenant, numa transação só: se a conta falhar, o convite continua pendente.
func (r *PostgresAuthRepository) AcceptInviteTx(ctx context.Context, p auth.AcceptInviteParams) (auth.User, error) {
	user := auth.User{ID: p.UserID, CreatedAt: p.Now, UpdatedAt: p.Now}
	err := r.inTx(ctx, func(qtx *gen.Queries) error {
		inv, err := qtx.ClaimInvite(ctx, gen.ClaimInviteParams{TokenHash: p.TokenHash, Now: pgtype.Timestamptz{Time: p.Now, Valid: true}})
		if err != nil {
			return claimErr(err)
		}
		user.Email = inv.Email
		return createInvitedAccount(ctx, qtx, inv, user, p)
	})
	if err != nil {
		return auth.User{}, err
	}
	return user, nil
}

func createInvitedAccount(ctx context.Context, qtx *gen.Queries, inv gen.ClaimInviteRow, u auth.User, p auth.AcceptInviteParams) error {
	ident := auth.UserIdentity{ID: p.IdentityID, UserID: u.ID, Provider: "local", ProviderUserID: inv.Email, PasswordHash: p.PasswordHash, CreatedAt: p.Now}
	if err := insertUserRows(ctx, qtx, u, ident); err != nil {
		return err
	}
	join := tenantJoin{tenantID: inv.TenantID, userID: u.ID, role: inv.Role, vaultID: p.InitialVaultID, vaultName: auth.InitialVaultName, at: p.Now}
	if err := joinTenant(ctx, qtx, join); err != nil {
		return err
	}
	return qtx.CreateAuditEvent(ctx, accountEvent(p.AuditEventID, inv.TenantID, "auth.accept_invite", u))
}

func claimErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInviteInvalid
	}
	return fmt.Errorf("consumir convite: %w", err)
}

func (r *PostgresAuthRepository) inTx(ctx context.Context, fn func(qtx *gen.Queries) error) error {
	return inTx(ctx, r.pool, fn)
}

// GetMembership devolve o vínculo de userID com tenantID, ou auth.ErrMembershipNotFound.
func (r *PostgresAuthRepository) GetMembership(ctx context.Context, userID, tenantID string) (auth.Membership, error) {
	row, err := r.q.GetMembership(ctx, gen.GetMembershipParams{UserID: userID, TenantID: tenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Membership{}, fmt.Errorf("%w: usuario %q, tenant %q", auth.ErrMembershipNotFound, userID, tenantID)
	}
	if err != nil {
		return auth.Membership{}, fmt.Errorf("buscar vinculo de %q com %q: %w", userID, tenantID, err)
	}
	return auth.Membership(row), nil
}

// ListMemberships devolve os tenants de userID, por nome.
func (r *PostgresAuthRepository) ListMemberships(ctx context.Context, userID string) ([]auth.Membership, error) {
	rows, err := r.q.ListMemberships(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listar tenants de %q: %w", userID, err)
	}
	out := make([]auth.Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, auth.Membership(row))
	}
	return out, nil
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
	return auth.User(row)
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
	return r.inTx(ctx, func(qtx *gen.Queries) error {
		return r.saveSessionRows(ctx, qtx, rec, event)
	})
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
		TenantID:   pgtype.Text{String: e.TenantID, Valid: e.TenantID != ""},
		ActorID:    pgtype.Text{String: e.ActorID, Valid: e.ActorID != ""},
		Action:     e.Action,
		Target:     e.Target,
		Result:     e.Result,
		OccurredAt: e.OccurredAt,
	}
}
