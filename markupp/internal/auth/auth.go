package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Erros de domínio do pacote auth.
var (
	ErrUserNotFound         = errors.New("user not found")
	ErrIdentityNotFound     = errors.New("identity not found")
	ErrUserAlreadyExists    = errors.New("user already exists")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrRegistrationDisabled = errors.New("self-registration is disabled")
	ErrLocalAuthDisabled    = errors.New("local authentication is disabled")
	ErrPasswordTooLong      = errors.New("password exceeds the maximum length of 72 bytes")
	ErrPasswordTooShort     = errors.New("password is shorter than the minimum of 15 characters")
	ErrCommonPassword       = errors.New("password is in the list of common passwords")
	ErrRefreshTokenRevoked  = errors.New("refresh token revoked")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
	ErrRefreshTokenInvalid  = errors.New("invalid refresh token")
	ErrEmptyEmail           = errors.New("email is required")
	ErrEmptyPassword        = errors.New("password is required")
	ErrInviteInvalid        = errors.New("invite not found, expired or already accepted")
	ErrMembershipNotFound   = errors.New("user is not a member of the tenant")
	ErrTenantRequired       = errors.New("user belongs to several tenants and must choose one")
	ErrNoTenant             = errors.New("user does not belong to any tenant")
	ErrInvalidTenantName    = errors.New("invalid tenant name")
)

// User representa uma conta da instância com chave estável (UUID). O acesso a cada
// tenant vem de um Membership.
type User struct {
	ID    string
	Email string
	// EmailVerified diz se a pessoa já provou o email; na criação da conta quem decide é o caminho de cadastro, não este campo.
	EmailVerified bool
	// PasswordLogin diz se a conta pode entrar por email e senha neste servidor; só GetAccount preenche.
	PasswordLogin bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
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

// TenantSummary é o vínculo com o resumo das notas que a conta lê no tenant:
// quantas são e quando a mais recente mudou, nil sem nota.
type TenantSummary struct {
	Membership
	NotesCount int
	LastNoteAt *time.Time
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

// normalizeEmail é a forma do email que o banco guarda e compara: sem espaço nas pontas
// e em minúsculas.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// InitialVaultName é o nome do cofre que toda conta ganha ao entrar num tenant. O painel
// é em inglês, e o nome nasce na mesma língua.
const InitialVaultName = "Default"

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
	ListTenantSummaries(ctx context.Context, userID string) ([]TenantSummary, error)
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
