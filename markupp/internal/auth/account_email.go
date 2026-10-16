package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Erros dos pedidos de email da conta. Token de email inexistente, vencido, já usado
// ou de outra conta devolve ErrInvalidToken, o mesmo do JWT.
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

// PasswordResetParams é a troca de senha pelo token recebido por email. HashPassword
// só é chamado depois que o token vale, para token lixo não custar um bcrypt.
type PasswordResetParams struct {
	TokenHash    string
	AuditEventID string
	Now          time.Time
	HashPassword func() (string, error)
}

// AccountEmailRepository é a persistência dos pedidos de email da conta. Cada pedido
// enfileira a notificação na mesma transação, e o token nasce na entrega. Validade e
// intervalo entre pedidos seguem o relógio do banco.
type AccountEmailRepository interface {
	// RequestEmailVerificationTx troca o pedido de verificação aberto da conta por um
	// novo, ou não faz nada dentro de cooldown. Devolve ErrEmailAlreadyVerified e
	// ErrUserNotFound.
	RequestEmailVerificationTx(ctx context.Context, userID string, cooldown time.Duration) error
	// ConfirmEmailTx verifica a conta userID se o token for dela, ou devolve
	// ErrInvalidToken sem gastar o token.
	ConfirmEmailTx(ctx context.Context, userID, tokenHash string) error
	// RequestPasswordResetTx pede a troca para a conta local do email. Email sem conta
	// e pedido dentro de cooldown não fazem nada e não dão erro.
	RequestPasswordResetTx(ctx context.Context, email string, cooldown time.Duration) error
	// ResetPasswordTx troca a senha, verifica o email, derruba as sessões e audita, ou
	// devolve ErrInvalidToken.
	ResetPasswordTx(ctx context.Context, p PasswordResetParams) error
}

// AccountEmailOptions configura o AccountEmailService.
type AccountEmailOptions struct {
	// EmailEnabled diz que há SMTP. Sem ele, os pedidos que mandam email devolvem
	// ErrEmailUnavailable.
	EmailEnabled bool
	// LocalEnabled diz que a credencial local existe. Sem ela, não há senha a trocar.
	LocalEnabled bool
	// Background roda o pedido de troca de senha fora da requisição, para o tempo de
	// resposta não revelar se a conta existe.
	Background func(func())
	Logger     *slog.Logger
}

// AccountEmailService atende a verificação de email e a recuperação de acesso, que só
// existem com SMTP configurado (ADR-0021).
type AccountEmailService struct {
	repo   AccountEmailRepository
	hasher PasswordHasher
	clock  func() time.Time
	newID  func() string
	opts   AccountEmailOptions
}

// NewAccountEmailService monta o serviço sobre o repositório, o hasher e o relógio.
func NewAccountEmailService(repo AccountEmailRepository, hasher PasswordHasher, clock func() time.Time, opts AccountEmailOptions) *AccountEmailService {
	return &AccountEmailService{repo: repo, hasher: hasher, clock: clock, newID: uuid.NewString, opts: opts}
}

// RequestEmailVerification envia de novo o email de verificação da conta userID.
func (s *AccountEmailService) RequestEmailVerification(ctx context.Context, userID string) error {
	if !s.opts.EmailEnabled {
		return ErrEmailUnavailable
	}
	return s.repo.RequestEmailVerificationTx(ctx, userID, emailRequestCooldown)
}

// ConfirmEmail verifica a conta logada userID com o token recebido por email. O token
// de outra conta não serve, então o dono do email não verifica a conta de um impostor
// que se cadastrou com o email dele.
func (s *AccountEmailService) ConfirmEmail(ctx context.Context, userID, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrInvalidToken
	}
	return s.repo.ConfirmEmailTx(ctx, userID, HashOpaqueToken(token))
}

// RequestPasswordReset aceita o pedido de troca de senha e o grava fora da
// requisição. A resposta é a mesma, no conteúdo e no tempo, exista a conta ou não.
func (s *AccountEmailService) RequestPasswordReset(ctx context.Context, email string) error {
	if err := s.checkPasswordFlow(); err != nil {
		return err
	}
	normEmail := normalizeEmail(email)
	if normEmail == "" {
		return ErrEmptyEmail
	}
	detached := context.WithoutCancel(ctx)
	// ponytail: pedido em voo se perde se o processo cair; a pessoa pede de novo.
	s.opts.Background(func() {
		if err := s.repo.RequestPasswordResetTx(detached, normEmail, emailRequestCooldown); err != nil {
			s.opts.Logger.Error("falha ao gravar pedido de troca de senha", "email", normEmail, "error", err)
		}
	})
	return nil
}

// ResetPassword troca a senha da conta dona do token recebido por email.
func (s *AccountEmailService) ResetPassword(ctx context.Context, token, password string) error {
	if err := s.checkPasswordFlow(); err != nil {
		return err
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return ErrInvalidToken
	}
	return s.repo.ResetPasswordTx(ctx, PasswordResetParams{
		TokenHash: HashOpaqueToken(token), AuditEventID: s.newID(), Now: s.clock(),
		HashPassword: func() (string, error) { return s.hasher.Hash(password) },
	})
}

func (s *AccountEmailService) checkPasswordFlow() error {
	if !s.opts.EmailEnabled {
		return ErrEmailUnavailable
	}
	if !s.opts.LocalEnabled {
		return ErrLocalAuthDisabled
	}
	return nil
}
