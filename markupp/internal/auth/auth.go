package auth

import (
	"context"
	"errors"
	"time"
)

// Erros de domínio do pacote auth.
var (
	ErrUserNotFound         = errors.New("usuario nao encontrado")
	ErrIdentityNotFound     = errors.New("identidade nao encontrada")
	ErrUserAlreadyExists    = errors.New("usuario ja existe")
	ErrInvalidCredentials   = errors.New("credenciais invalidas")
	ErrRegistrationDisabled = errors.New("auto-registro desativado")
	ErrRefreshTokenRevoked  = errors.New("refresh token revogado")
	ErrRefreshTokenExpired  = errors.New("refresh token expirado")
	ErrEmptyEmail           = errors.New("email obrigatorio")
	ErrEmptyPassword        = errors.New("senha obrigatoria")
)

// User representa uma conta com chave estável (UUID).
type User struct {
	ID        string
	TenantID  string
	Email     string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserIdentity isola o vínculo de autenticação externa ou credencial local.
type UserIdentity struct {
	ID             string
	UserID         string
	Provider       string
	ProviderUserID string
	PasswordHash   string
	CreatedAt      time.Time
}

// RefreshTokenRecord armazena a sessão e validade do refresh token.
type RefreshTokenRecord struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}

// AuditEventRecord armazena o evento de auditoria insert-only.
type AuditEventRecord struct {
	ID         string
	TenantID   string
	ActorID    string
	Action     string
	Target     string
	Result     string
	OccurredAt time.Time
}

// CreateAccountParams empacota os dados para criação atômica de conta, cofre e auditoria.
type CreateAccountParams struct {
	User             User
	Identity         UserIdentity
	InitialVaultID   string
	InitialVaultName string
	AuditEvent       AuditEventRecord
}

// Repository é o contrato de persistência consumido pelo Service.
type Repository interface {
	CountUsers(ctx context.Context) (int64, error)
	CreateAccountTx(ctx context.Context, params CreateAccountParams) error
	GetUserByID(ctx context.Context, id string) (User, error)
	GetUserByEmail(ctx context.Context, tenantID, email string) (User, error)
	GetUserIdentity(ctx context.Context, provider, providerUserID string) (UserIdentity, error)
	SaveRefreshToken(ctx context.Context, record RefreshTokenRecord) error
	GetRefreshToken(ctx context.Context, tokenHash string) (RefreshTokenRecord, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RecordAuditEvent(ctx context.Context, event AuditEventRecord) error
}
