package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TokenIssuer define o contrato de geração e validação de tokens para o serviço.
type TokenIssuer interface {
	GenerateTokenPair(userID, tenantID, role string) (TokenPair, error)
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
	normEmail, err := s.validateCredentials(email, password)
	if err != nil {
		return TokenPair{}, err
	}
	userCount, err := s.repo.CountUsers(ctx)
	if err != nil {
		return TokenPair{}, fmt.Errorf("contar usuarios: %w", err)
	}
	if userCount == 0 {
		return s.bootstrapAdmin(ctx, normEmail, password)
	}
	return s.authenticateExistingOrReject(ctx, normEmail, password)
}

func (s *Service) validateCredentials(email, password string) (string, error) {
	if strings.TrimSpace(email) == "" {
		return "", ErrEmptyEmail
	}
	if strings.TrimSpace(password) == "" {
		return "", ErrEmptyPassword
	}
	return strings.ToLower(strings.TrimSpace(email)), nil
}

func (s *Service) bootstrapAdmin(ctx context.Context, email, password string) (TokenPair, error) {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return TokenPair{}, fmt.Errorf("hashear senha: %w", err)
	}
	userID := s.newID()
	p := s.buildAccountParams(userID, email, "admin", hash, "auth.register_admin")
	if err := s.repo.CreateAccountTx(ctx, p); err != nil {
		return TokenPair{}, fmt.Errorf("criar conta admin: %w", err)
	}
	return s.issueAndPersistTokens(ctx, userID, s.cfg.DefaultTenantID, "admin")
}

func (s *Service) authenticateExistingOrReject(ctx context.Context, email, password string) (TokenPair, error) {
	identity, err := s.repo.GetUserIdentity(ctx, "local", email)
	if err != nil {
		if errors.Is(err, ErrIdentityNotFound) {
			return s.handleUnknownUser(ctx, email, password)
		}
		return TokenPair{}, fmt.Errorf("buscar identidade: %w", err)
	}
	return s.verifyPasswordAndIssue(ctx, identity, email, password)
}

func (s *Service) handleUnknownUser(ctx context.Context, email, password string) (TokenPair, error) {
	_ = s.auditLoginAttempt(ctx, "", email, "failure")
	if !s.cfg.RegistrationEnabled {
		return TokenPair{}, ErrRegistrationDisabled
	}
	return s.registerMember(ctx, email, password)
}

func (s *Service) registerMember(ctx context.Context, email, password string) (TokenPair, error) {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return TokenPair{}, fmt.Errorf("hashear senha: %w", err)
	}
	userID := s.newID()
	p := s.buildAccountParams(userID, email, "member", hash, "auth.register")
	if err := s.repo.CreateAccountTx(ctx, p); err != nil {
		return TokenPair{}, fmt.Errorf("criar membro: %w", err)
	}
	return s.issueAndPersistTokens(ctx, userID, s.cfg.DefaultTenantID, "member")
}

func (s *Service) buildAccountParams(userID, email, role, hash, action string) CreateAccountParams {
	now := s.clock()
	return CreateAccountParams{
		User:             User{ID: userID, TenantID: s.cfg.DefaultTenantID, Email: email, Role: role, CreatedAt: now, UpdatedAt: now},
		Identity:         UserIdentity{ID: s.newID(), UserID: userID, Provider: "local", ProviderUserID: email, PasswordHash: hash, CreatedAt: now},
		InitialVaultID:   s.newID(),
		InitialVaultName: "Principal",
		AuditEvent:       AuditEventRecord{ID: s.newID(), TenantID: s.cfg.DefaultTenantID, ActorID: userID, Action: action, Target: email, Result: "success", OccurredAt: now},
	}
}

func (s *Service) verifyPasswordAndIssue(ctx context.Context, identity UserIdentity, email, password string) (TokenPair, error) {
	if err := s.hasher.Compare(identity.PasswordHash, password); err != nil {
		_ = s.auditLoginAttempt(ctx, identity.UserID, email, "failure")
		return TokenPair{}, ErrInvalidCredentials
	}
	user, err := s.repo.GetUserByID(ctx, identity.UserID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("buscar usuario: %w", err)
	}
	_ = s.auditLoginAttempt(ctx, user.ID, email, "success")
	return s.issueAndPersistTokens(ctx, user.ID, user.TenantID, user.Role)
}

func (s *Service) auditLoginAttempt(ctx context.Context, actorID, target, result string) error {
	return s.repo.RecordAuditEvent(ctx, AuditEventRecord{
		ID:         s.newID(),
		TenantID:   s.cfg.DefaultTenantID,
		ActorID:    actorID,
		Action:     "auth.login",
		Target:     target,
		Result:     result,
		OccurredAt: s.clock(),
	})
}

func (s *Service) issueAndPersistTokens(ctx context.Context, userID, tenantID, role string) (TokenPair, error) {
	pair, err := s.tokens.GenerateTokenPair(userID, tenantID, role)
	if err != nil {
		return TokenPair{}, fmt.Errorf("gerar tokens: %w", err)
	}
	now := s.clock()
	rec := RefreshTokenRecord{
		TokenHash: HashRefreshToken(pair.RefreshToken),
		UserID:    userID,
		ExpiresAt: now.Add(s.cfg.RefreshExpirationPeriod),
		Revoked:   false,
		CreatedAt: now,
	}
	if err := s.repo.SaveRefreshToken(ctx, rec); err != nil {
		return TokenPair{}, fmt.Errorf("salvar refresh token: %w", err)
	}
	return pair, nil
}

// RefreshToken valida o refresh token fornecido, revoga-o e emite um novo par de tokens.
func (s *Service) RefreshToken(ctx context.Context, rawRefreshToken string) (TokenPair, error) {
	hash := HashRefreshToken(rawRefreshToken)
	rec, err := s.repo.GetRefreshToken(ctx, hash)
	if err != nil {
		return TokenPair{}, err
	}
	if rec.Revoked {
		return TokenPair{}, ErrRefreshTokenRevoked
	}
	if rec.ExpiresAt.Before(s.clock()) {
		return TokenPair{}, ErrRefreshTokenExpired
	}
	_ = s.repo.RevokeRefreshToken(ctx, hash)
	user, err := s.repo.GetUserByID(ctx, rec.UserID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("buscar usuario do token: %w", err)
	}
	return s.issueAndPersistTokens(ctx, user.ID, user.TenantID, user.Role)
}

// RevokeSession revoga a sessão associada ao refresh token.
func (s *Service) RevokeSession(ctx context.Context, rawRefreshToken string) error {
	hash := HashRefreshToken(rawRefreshToken)
	return s.repo.RevokeRefreshToken(ctx, hash)
}

// GetAccount busca os detalhes da conta do usuário.
func (s *Service) GetAccount(ctx context.Context, userID string) (User, error) {
	return s.repo.GetUserByID(ctx, userID)
}
