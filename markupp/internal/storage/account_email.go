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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// Propósitos de account_tokens, iguais ao tipo da notificação que entrega o token.
const (
	purposeEmailVerification = string(notify.KindEmailVerification)
	purposePasswordReset     = string(notify.KindPasswordReset)
)

// PostgresAccountEmailRepository persiste os pedidos de verificação de email e de
// troca de senha. Roda no papel da conexão, como a autenticação: a conta é da
// instância, fora do recorte por tenant.
type PostgresAccountEmailRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAccountEmailRepository monta o repositório sobre um pool já aberto.
func NewPostgresAccountEmailRepository(pool *pgxpool.Pool) *PostgresAccountEmailRepository {
	return &PostgresAccountEmailRepository{pool: pool}
}

// RequestEmailVerificationTx troca o pedido aberto da conta por um novo, que invalida o
// token anterior, ou não faz nada se houve pedido dentro de cooldown.
func (r *PostgresAccountEmailRepository) RequestEmailVerificationTx(ctx context.Context, userID string, cooldown time.Duration) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		verified, err := lockUser(ctx, q, userID)
		if err != nil {
			return err
		}
		if verified {
			return auth.ErrEmailAlreadyVerified
		}
		return renewAccountToken(ctx, q, userID, purposeEmailVerification, cooldown)
	})
}

// lockUser trava a linha da conta, o que serializa os pedidos dela, e devolve se o
// email já está verificado.
func lockUser(ctx context.Context, q *gen.Queries, userID string) (bool, error) {
	verified, err := q.LockUserEmailVerified(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("%w: %q", auth.ErrUserNotFound, userID)
	}
	if err != nil {
		return false, fmt.Errorf("travar a conta %q: %w", userID, err)
	}
	return verified, nil
}

// renewAccountToken troca o token aberto de purpose da conta por um novo, a não ser
// que já exista um pedido dentro de cooldown. Precisa da conta travada.
func renewAccountToken(ctx context.Context, q *gen.Queries, userID, purpose string, cooldown time.Duration) error {
	recent, err := q.RecentAccountTokenExists(ctx, gen.RecentAccountTokenExistsParams{UserID: userID, Purpose: purpose, Cooldown: interval(cooldown)})
	if err != nil {
		return fmt.Errorf("conferir pedido recente de %s da conta %q: %w", purpose, userID, err)
	}
	if recent {
		return nil
	}
	if err := q.DeleteOpenAccountTokens(ctx, gen.DeleteOpenAccountTokensParams{UserID: userID, Purpose: purpose}); err != nil {
		return fmt.Errorf("invalidar %s aberto da conta %q: %w", purpose, userID, err)
	}
	return enqueueAccountToken(ctx, q, userID, purpose)
}

// enqueueAccountToken cria o token de purpose, ainda sem hash, e a notificação dele.
func enqueueAccountToken(ctx context.Context, q *gen.Queries, userID, purpose string) error {
	id, err := q.CreateAccountToken(ctx, gen.CreateAccountTokenParams{UserID: userID, Purpose: purpose})
	if err != nil {
		return fmt.Errorf("criar %s da conta %q: %w", purpose, userID, err)
	}
	return q.EnqueueAccountTokenNotification(ctx, gen.EnqueueAccountTokenNotificationParams{
		Kind: purpose, AccountTokenID: pgtype.Text{String: id, Valid: true},
	})
}

// ConfirmEmailTx marca a conta userID como verificada se o token for dela, uma vez só.
// Token de outra conta não é gasto.
func (r *PostgresAccountEmailRepository) ConfirmEmailTx(ctx context.Context, userID, tokenHash string) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		owner, err := useAccountToken(ctx, q, tokenHash, purposeEmailVerification, pgtype.Text{String: userID, Valid: true})
		if err != nil {
			return err
		}
		return q.MarkEmailVerified(ctx, owner)
	})
}

// useAccountToken gasta o token aberto e dentro da validade, e devolve a conta dona
// dele. Com user válido, só o token dessa conta serve.
func useAccountToken(ctx context.Context, q *gen.Queries, tokenHash, purpose string, user pgtype.Text) (string, error) {
	owner, err := q.UseAccountToken(ctx, gen.UseAccountTokenParams{
		TokenHash: pgtype.Text{String: tokenHash, Valid: true}, Purpose: purpose, UserID: user,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", auth.ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("usar token de %s: %w", purpose, err)
	}
	return owner, nil
}

// RequestPasswordResetTx pede a troca para a conta local do email. Email sem conta e
// pedido dentro de cooldown terminam sem erro, para a resposta não revelar nada.
func (r *PostgresAccountEmailRepository) RequestPasswordResetTx(ctx context.Context, email string, cooldown time.Duration) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		userID, err := q.LocalUserIDByEmail(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("buscar conta local de %q: %w", email, err)
		}
		if _, err := lockUser(ctx, q, userID); err != nil {
			return err
		}
		return renewAccountToken(ctx, q, userID, purposePasswordReset, cooldown)
	})
}

// ResetPasswordTx gasta o token, só então calcula o hash da senha nova, e troca a senha
// local, verifica o email, derruba as sessões e audita, tudo na mesma transação.
func (r *PostgresAccountEmailRepository) ResetPasswordTx(ctx context.Context, p auth.PasswordResetParams) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		userID, err := useAccountToken(ctx, q, p.TokenHash, purposePasswordReset, pgtype.Text{})
		if err != nil {
			return err
		}
		hash, err := p.HashPassword()
		if err != nil {
			return fmt.Errorf("calcular hash da senha nova da conta %q: %w", userID, err)
		}
		return applyPasswordReset(ctx, q, userID, hash, p)
	})
}

// applyPasswordReset grava a troca. Quem usou o token do email provou que controla o
// email, então a conta fica verificada.
func applyPasswordReset(ctx context.Context, q *gen.Queries, userID, hash string, p auth.PasswordResetParams) error {
	if err := q.UpdateLocalPassword(ctx, gen.UpdateLocalPasswordParams{UserID: userID, PasswordHash: pgtype.Text{String: hash, Valid: true}}); err != nil {
		return fmt.Errorf("gravar senha nova da conta %q: %w", userID, err)
	}
	if err := q.MarkEmailVerified(ctx, userID); err != nil {
		return fmt.Errorf("verificar email da conta %q: %w", userID, err)
	}
	if err := q.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("derrubar sessoes da conta %q: %w", userID, err)
	}
	return q.CreateAuditEvent(ctx, toCreateAuditEventParams(auth.AuditEventRecord{
		ID: p.AuditEventID, ActorID: userID, Action: "auth.password_reset", Target: userID, Result: "success", OccurredAt: p.Now,
	}))
}
