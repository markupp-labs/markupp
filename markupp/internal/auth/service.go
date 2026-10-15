package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

const maxPasswordLength = 72

// TokenIssuer define o contrato de geração e validação de tokens para o serviço.
type TokenIssuer interface {
	GenerateTokenPair(userID string) (TokenPair, error)
	ValidateToken(tokenString string) (Claims, error)
}

// ServiceConfig define os parâmetros operacionais da autenticação.
type ServiceConfig struct {
	DefaultTenantID         string
	RegistrationEnabled     bool
	LocalEnabled            bool
	RefreshExpirationPeriod time.Duration
}

// Service orquestra as regras de negócio de contas, credenciais e sessões.
type Service struct {
	repo   Repository
	tokens TokenIssuer
	hasher PasswordHasher
	cfg    ServiceConfig
	clock  func() time.Time
	newID  func() string
}

// NewService constrói uma instância de Service com as dependências fornecidas.
func NewService(repo Repository, tokens TokenIssuer, hasher PasswordHasher, cfg ServiceConfig, clock func() time.Time) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
		hasher: hasher,
		cfg:    cfg,
		clock:  clock,
		newID:  uuid.NewString,
	}
}

// LoginLocal autentica via email e senha ou inicializa a primeira conta de administrador.
func (s *Service) LoginLocal(ctx context.Context, email, password string) (TokenPair, error) {
	if !s.cfg.LocalEnabled {
		return TokenPair{}, ErrLocalAuthDisabled
	}
	normEmail, err := s.validateCredentials(email, password)
	if err != nil {
		return TokenPair{}, err
	}
	identity, err := s.repo.GetUserIdentity(ctx, "local", normEmail)
	if err != nil {
		if errors.Is(err, ErrIdentityNotFound) {
			return s.handleUnknownUser(ctx, normEmail, password)
		}
		return TokenPair{}, fmt.Errorf("buscar identidade local %q: %w", normEmail, err)
	}
	return s.verifyPasswordAndIssue(ctx, identity, normEmail, password)
}

func (s *Service) validateCredentials(email, password string) (string, error) {
	if strings.TrimSpace(email) == "" {
		return "", ErrEmptyEmail
	}
	if err := validatePassword(password); err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(email)), nil
}

func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return ErrEmptyPassword
	}
	if len(password) > maxPasswordLength {
		return fmt.Errorf("senha com %d bytes, maximo %d: %w", len(password), maxPasswordLength, ErrPasswordTooLong)
	}
	return nil
}

// AcceptInvite troca o token do convite e a senha escolhida por uma conta local no
// tenant e com o papel do convite, e abre a sessão.
func (s *Service) AcceptInvite(ctx context.Context, token, password string) (TokenPair, error) {
	if !s.cfg.LocalEnabled {
		return TokenPair{}, ErrLocalAuthDisabled
	}
	if err := validatePassword(password); err != nil {
		return TokenPair{}, err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return TokenPair{}, fmt.Errorf("hashear senha do convite: %w", err)
	}
	user, err := s.repo.AcceptInviteTx(ctx, s.acceptInviteParams(token, hash))
	if err != nil {
		return TokenPair{}, fmt.Errorf("aceitar convite: %w", err)
	}
	return s.issueAndPersist(ctx, user, "auth.login")
}

func (s *Service) acceptInviteParams(token, passwordHash string) AcceptInviteParams {
	return AcceptInviteParams{
		TokenHash:      HashOpaqueToken(token),
		UserID:         s.newID(),
		IdentityID:     s.newID(),
		PasswordHash:   passwordHash,
		InitialVaultID: s.newID(),
		AuditEventID:   s.newID(),
		Now:            s.clock(),
	}
}

func (s *Service) handleUnknownUser(ctx context.Context, email, password string) (TokenPair, error) {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return TokenPair{}, fmt.Errorf("hashear senha para %q: %w", email, err)
	}
	p := s.buildAccountParams(email, hash)
	user, err := s.repo.CreateAccountTx(ctx, p)
	if err != nil {
		if errors.Is(err, ErrRegistrationDisabled) {
			return s.rejectUnknownUser(ctx, email)
		}
		return TokenPair{}, fmt.Errorf("criar conta para %q: %w", email, err)
	}
	return s.issueAndPersist(ctx, user, "auth.login")
}

// rejectUnknownUser nao roda bcrypt de novo: o Hash em handleUnknownUser ja igualou o
// custo ao de uma senha errada, e um segundo bcrypt denunciaria o email inexistente.
func (s *Service) rejectUnknownUser(ctx context.Context, email string) (TokenPair, error) {
	if err := s.auditLoginAttempt(ctx, "", email, "failure"); err != nil {
		return TokenPair{}, fmt.Errorf("auditar login recusado para %q: %w", email, err)
	}
	return TokenPair{}, ErrInvalidCredentials
}

func (s *Service) buildAccountParams(email, hash string) CreateAccountParams {
	now := s.clock()
	userID := s.newID()
	return CreateAccountParams{
		User:                    User{ID: userID, Email: email, CreatedAt: now, UpdatedAt: now},
		BootstrapTenantID:       s.cfg.DefaultTenantID,
		Identity:                UserIdentity{ID: s.newID(), UserID: userID, Provider: "local", ProviderUserID: email, PasswordHash: hash, CreatedAt: now},
		InitialVaultID:          s.newID(),
		InitialVaultName:        InitialVaultName,
		AllowMemberRegistration: s.cfg.RegistrationEnabled,
		AuditEventID:            s.newID(),
	}
}

func (s *Service) verifyPasswordAndIssue(ctx context.Context, identity UserIdentity, email, password string) (TokenPair, error) {
	if err := s.hasher.Compare(identity.PasswordHash, password); err != nil {
		if auditErr := s.auditLoginAttempt(ctx, identity.UserID, email, "failure"); auditErr != nil {
			return TokenPair{}, fmt.Errorf("auditar falha de senha para %q: %w", email, auditErr)
		}
		return TokenPair{}, ErrInvalidCredentials
	}
	user, err := s.repo.GetUserByID(ctx, identity.UserID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("buscar usuario %q: %w", identity.UserID, err)
	}
	return s.issueAndPersist(ctx, user, "auth.login")
}

func (s *Service) auditLoginAttempt(ctx context.Context, actorID, target, result string) error {
	return s.repo.RecordAuditEvent(ctx, AuditEventRecord{
		ID:         s.newID(),
		ActorID:    actorID,
		Action:     "auth.login",
		Target:     target,
		Result:     result,
		OccurredAt: s.clock(),
	})
}

func (s *Service) buildSessionRecords(user User, pair TokenPair, action string) (RefreshTokenRecord, AuditEventRecord) {
	now := s.clock()
	rec := RefreshTokenRecord{
		TokenHash: HashOpaqueToken(pair.RefreshToken),
		UserID:    user.ID,
		ExpiresAt: now.Add(s.cfg.RefreshExpirationPeriod),
		Revoked:   false,
		CreatedAt: now,
	}
	event := AuditEventRecord{
		ID:         s.newID(),
		ActorID:    user.ID,
		Action:     action,
		Target:     user.Email,
		Result:     "success",
		OccurredAt: now,
	}
	return rec, event
}

func (s *Service) issueAndPersist(ctx context.Context, user User, action string) (TokenPair, error) {
	pair, err := s.tokens.GenerateTokenPair(user.ID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("gerar tokens para %q: %w", user.ID, err)
	}
	rec, event := s.buildSessionRecords(user, pair, action)
	if err := s.repo.IssueSessionTx(ctx, rec, event); err != nil {
		return TokenPair{}, fmt.Errorf("gravar sessao de %q: %w", user.ID, err)
	}
	return pair, nil
}

// RefreshToken valida o refresh token fornecido, revoga-o e emite um novo par de tokens.
func (s *Service) RefreshToken(ctx context.Context, rawRefreshToken string) (TokenPair, error) {
	hash := HashOpaqueToken(rawRefreshToken)
	userID, err := s.repo.ConsumeRefreshToken(ctx, hash)
	if err != nil {
		return s.handleRefreshFailure(ctx, hash, err)
	}
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("buscar usuario %q do refresh token: %w", userID, err)
	}
	return s.issueAndPersist(ctx, user, "auth.refresh")
}

func (s *Service) handleRefreshFailure(ctx context.Context, hash string, origErr error) (TokenPair, error) {
	rec, err := s.repo.GetRefreshToken(ctx, hash)
	if err == nil && rec.Revoked {
		return s.handleRefreshReuse(ctx, rec.UserID)
	}
	return TokenPair{}, origErr
}

func (s *Service) handleRefreshReuse(ctx context.Context, userID string) (TokenPair, error) {
	if err := s.repo.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		return TokenPair{}, fmt.Errorf("revogar todas sessoes do usuario %q: %w", userID, err)
	}
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("buscar usuario %q do token reusado: %w", userID, err)
	}
	if err := s.repo.RecordAuditEvent(ctx, s.refreshReuseEvent(user)); err != nil {
		return TokenPair{}, fmt.Errorf("auditar reuso de token do usuario %q: %w", userID, err)
	}
	return TokenPair{}, ErrRefreshTokenRevoked
}

func (s *Service) refreshReuseEvent(user User) AuditEventRecord {
	return AuditEventRecord{
		ID:         s.newID(),
		ActorID:    user.ID,
		Action:     "auth.refresh_reuse",
		Target:     user.ID,
		Result:     "failure",
		OccurredAt: s.clock(),
	}
}

// RevokeSession revoga a sessão associada ao refresh token.
func (s *Service) RevokeSession(ctx context.Context, rawRefreshToken string) error {
	hash := HashOpaqueToken(rawRefreshToken)
	return s.repo.RevokeRefreshToken(ctx, hash)
}

// GetAccount busca os detalhes da conta do usuário.
func (s *Service) GetAccount(ctx context.Context, userID string) (User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// ResolveTenant devolve o vínculo de userID com o tenant requested. Sem pedido, vale o
// único tenant da pessoa, e quem tem vários precisa escolher.
func (s *Service) ResolveTenant(ctx context.Context, userID, requested string) (Membership, error) {
	if requested != "" {
		return s.repo.GetMembership(ctx, userID, requested)
	}
	memberships, err := s.repo.ListMemberships(ctx, userID)
	if err != nil {
		return Membership{}, err
	}
	switch len(memberships) {
	case 0:
		return Membership{}, fmt.Errorf("%w: usuario %q", ErrNoTenant, userID)
	case 1:
		return memberships[0], nil
	}
	return Membership{}, fmt.Errorf("%w: usuario %q esta em %d tenants", ErrTenantRequired, userID, len(memberships))
}

// ListTenants devolve os tenants de userID, com o papel em cada um.
func (s *Service) ListTenants(ctx context.Context, userID string) ([]Membership, error) {
	return s.repo.ListMemberships(ctx, userID)
}

// maxTenantNameLength limita o nome que aparece no seletor de tenant dos clientes.
const maxTenantNameLength = 100

// CreateTenant cria o tenant name com userID como owner e o cofre inicial dele lá.
func (s *Service) CreateTenant(ctx context.Context, userID, name string) (Membership, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxTenantNameLength {
		return Membership{}, fmt.Errorf("%w: %q, esperado de 1 a %d caracteres", ErrInvalidTenantName, name, maxTenantNameLength)
	}
	p := CreateTenantParams{
		TenantID: s.newID(), Name: name, UserID: userID,
		VaultID: s.newID(), AuditEventID: s.newID(), Now: s.clock(),
	}
	if err := s.repo.CreateTenantTx(ctx, p); err != nil {
		return Membership{}, fmt.Errorf("criar tenant %q para %q: %w", name, userID, err)
	}
	return Membership{TenantID: p.TenantID, TenantName: name, Role: identity.RoleOwner}, nil
}

// ListMyInvites devolve os convites pendentes para o email da conta userID.
func (s *Service) ListMyInvites(ctx context.Context, userID string) ([]PendingInvite, error) {
	return s.repo.ListPendingInvites(ctx, userID, s.clock())
}

// AcceptMyInvite aceita, pela conta userID, o convite inviteID endereçado a ela.
func (s *Service) AcceptMyInvite(ctx context.Context, userID, inviteID string) (Membership, error) {
	return s.repo.AcceptInviteByIDTx(ctx, AcceptInviteByIDParams{
		InviteID: inviteID, UserID: userID, VaultID: s.newID(), AuditEventID: s.newID(), Now: s.clock(),
	})
}

// DeclineMyInvite recusa, pela conta userID, o convite inviteID endereçado a ela.
func (s *Service) DeclineMyInvite(ctx context.Context, userID, inviteID string) error {
	return s.repo.DeclineInviteTx(ctx, DeclineInviteParams{
		InviteID: inviteID, UserID: userID, AuditEventID: s.newID(), Now: s.clock(),
	})
}
