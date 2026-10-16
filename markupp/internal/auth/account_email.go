package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Erros dos pedidos de email da conta. Token de email inexistente, vencido ou já
// usado devolve ErrInvalidToken, o mesmo do JWT.
var (
	ErrEmailUnavailable     = errors.New("envio de email indisponivel, sem SMTP configurado")
	ErrEmailAlreadyVerified = errors.New("email da conta ja verificado")
)

const (
	// EmailVerificationTTL é a validade do token de verificação, contada da entrega.
	EmailVerificationTTL = 24 * time.Hour
	// PasswordResetTTL é a validade do token de troca de senha, contada da entrega.
	PasswordResetTTL = time.Hour
	// emailRequestCooldown limita os pedidos de email por conta, contra quem usa o
	// markupp para mandar email em massa a um endereço alheio.
	emailRequestCooldown = 5 * time.Minute
)

// PasswordResetParams é a troca de senha pelo token recebido por email.
type PasswordResetParams struct {
	TokenHash    string
	PasswordHash string
	AuditEventID string
	Now          time.Time
}

// AccountEmailRepository é a persistência dos pedidos de email da conta. Cada pedido
// enfileira a notificação na mesma transação, e o token nasce na entrega.
type AccountEmailRepository interface {
	// RequestEmailVerificationTx troca o pedido de verificação pendente da conta por um
	// novo, ou não faz nada dentro de cooldown. Devolve ErrEmailAlreadyVerified.
	RequestEmailVerificationTx(ctx context.Context, userID string, now time.Time, cooldown time.Duration) error
	// ConfirmEmailTx verifica a conta dona do token, ou devolve ErrInvalidToken.
	ConfirmEmailTx(ctx context.Context, tokenHash string, now time.Time) error
	// RequestPasswordResetTx pede a troca para a conta local do email. Email sem conta
	// e pedido dentro de cooldown não fazem nada e não dão erro.
	RequestPasswordResetTx(ctx context.Context, email string, now time.Time, cooldown time.Duration) error
	// ResetPasswordTx troca a senha, derruba as sessões e audita, ou devolve
	// ErrInvalidToken.
	ResetPasswordTx(ctx context.Context, p PasswordResetParams) error
}

// AccountEmailService atende a verificação de email e a recuperação de acesso, que só
// existem com SMTP configurado (ADR-0021).
type AccountEmailService struct {
	repo         AccountEmailRepository
	hasher       PasswordHasher
	clock        func() time.Time
	newID        func() string
	emailEnabled bool
}

// NewAccountEmailService monta o serviço. Sem emailEnabled, todo pedido que manda
// email devolve ErrEmailUnavailable.
func NewAccountEmailService(repo AccountEmailRepository, hasher PasswordHasher, clock func() time.Time, emailEnabled bool) *AccountEmailService {
	return &AccountEmailService{repo: repo, hasher: hasher, clock: clock, newID: uuid.NewString, emailEnabled: emailEnabled}
}

// RequestEmailVerification envia de novo o email de verificação da conta userID.
func (s *AccountEmailService) RequestEmailVerification(ctx context.Context, userID string) error {
	if !s.emailEnabled {
		return ErrEmailUnavailable
	}
	return s.repo.RequestEmailVerificationTx(ctx, userID, s.clock(), emailRequestCooldown)
}

// ConfirmEmail verifica a conta dona do token recebido por email.
func (s *AccountEmailService) ConfirmEmail(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrInvalidToken
	}
	return s.repo.ConfirmEmailTx(ctx, HashOpaqueToken(token), s.clock())
}

// RequestPasswordReset pede o email de troca de senha. A resposta é a mesma exista a
// conta ou não, para não revelar quem tem conta.
func (s *AccountEmailService) RequestPasswordReset(ctx context.Context, email string) error {
	if !s.emailEnabled {
		return ErrEmailUnavailable
	}
	normEmail := strings.ToLower(strings.TrimSpace(email))
	if normEmail == "" {
		return ErrEmptyEmail
	}
	return s.repo.RequestPasswordResetTx(ctx, normEmail, s.clock(), emailRequestCooldown)
}

// ResetPassword troca a senha da conta dona do token recebido por email.
func (s *AccountEmailService) ResetPassword(ctx context.Context, token, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return ErrInvalidToken
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	return s.repo.ResetPasswordTx(ctx, PasswordResetParams{
		TokenHash: HashOpaqueToken(token), PasswordHash: hash, AuditEventID: s.newID(), Now: s.clock(),
	})
}
