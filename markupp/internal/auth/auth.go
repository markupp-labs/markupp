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
	ErrLocalAuthDisabled    = errors.New("autenticacao local desativada")
	ErrPasswordTooLong      = errors.New("senha excede o tamanho maximo de 72 bytes")
	ErrRefreshTokenRevoked  = errors.New("refresh token revogado")
	ErrRefreshTokenExpired  = errors.New("refresh token expirado")
	ErrRefreshTokenInvalid  = errors.New("refresh token invalido")
	ErrEmptyEmail           = errors.New("email obrigatorio")
	ErrEmptyPassword        = errors.New("senha obrigatoria")
	ErrInviteInvalid        = errors.New("convite inexistente, vencido ou ja aceito")
	ErrMembershipNotFound   = errors.New("usuario sem vinculo com o tenant")
	ErrTenantRequired       = errors.New("usuario de varios tenants precisa escolher um")
	ErrNoTenant             = errors.New("usuario sem nenhum tenant")
	ErrInvalidTenantName    = errors.New("nome de tenant invalido")
)

// User representa uma conta da instância com chave estável (UUID). O acesso a cada
// tenant vem de um Membership.
type User struct {
	ID        string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Member é um usuário visto de dentro do tenant, com o papel dele lá.
type Member struct {
	UserID   string
	Email    string
	Role     string
	JoinedAt time.Time
}

// Membership é o vínculo do usuário com um tenant e o papel dele lá.
type Membership struct {
	TenantID   string
	TenantName string
	Role       string
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

// CreateAccountParams empacota os dados para criação atômica de conta e auditoria. O
// primeiro usuário da instância ainda ganha o tenant BootstrapTenantID, como owner, e
// o cofre inicial nele.
type CreateAccountParams struct {
	User                    User
	BootstrapTenantID       string
	Identity                UserIdentity
	InitialVaultID          string
	InitialVaultName        string
	AllowMemberRegistration bool
	AuditEventID            string
	// SendVerification enfileira o email de verificação quando a conta nasce sem email
	// provado, o que só acontece no auto-registro.
	SendVerification bool
}

// InitialVaultName é o nome do cofre que toda conta ganha ao ser criada.
const InitialVaultName = "Principal"

// PendingInvite é o convite que aguarda resposta da conta convidada.
type PendingInvite struct {
	ID         string
	TenantID   string
	TenantName string
	Role       string
	ExpiresAt  time.Time
}

// AcceptInviteByIDParams é o aceite, pela própria conta, de um convite endereçado ao
// email dela, com o cofre que ela ganha no tenant.
type AcceptInviteByIDParams struct {
	InviteID     string
	UserID       string
	VaultID      string
	AuditEventID string
	Now          time.Time
}

// DeclineInviteParams é a recusa, pela própria conta, de um convite endereçado a ela.
type DeclineInviteParams struct {
	InviteID     string
	UserID       string
	AuditEventID string
	Now          time.Time
}

// CreateTenantParams carrega o tenant novo e o que o criador ganha nele: o vínculo de
// owner e o cofre inicial.
type CreateTenantParams struct {
	TenantID     string
	Name         string
	UserID       string
	VaultID      string
	AuditEventID string
	Now          time.Time
}

// AcceptInviteParams carrega o que a conta do convidado ganha além do que herda do
// convite (tenant, email e papel), que só o banco conhece a partir do hash do token.
type AcceptInviteParams struct {
	TokenHash      string
	UserID         string
	IdentityID     string
	PasswordHash   string
	InitialVaultID string
	AuditEventID   string
	Now            time.Time
}

// Repository é o contrato de persistência consumido pelo Service.
type Repository interface {
	CreateAccountTx(ctx context.Context, params CreateAccountParams) (User, error)
	GetMembership(ctx context.Context, userID, tenantID string) (Membership, error)
	ListMemberships(ctx context.Context, userID string) ([]Membership, error)
	CreateTenantTx(ctx context.Context, params CreateTenantParams) error
	ListPendingInvites(ctx context.Context, userID string, now time.Time) ([]PendingInvite, error)
	AcceptInviteByIDTx(ctx context.Context, params AcceptInviteByIDParams) (Membership, error)
	DeclineInviteTx(ctx context.Context, params DeclineInviteParams) error
	AcceptInviteTx(ctx context.Context, params AcceptInviteParams) (User, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	GetUserIdentity(ctx context.Context, provider, providerUserID string) (UserIdentity, error)
	ConsumeRefreshToken(ctx context.Context, tokenHash string) (string, error)
	IssueSessionTx(ctx context.Context, record RefreshTokenRecord, event AuditEventRecord) error
	GetRefreshToken(ctx context.Context, tokenHash string) (RefreshTokenRecord, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID string) error
	RecordAuditEvent(ctx context.Context, event AuditEventRecord) error
}
