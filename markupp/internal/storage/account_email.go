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
func (r *PostgresAccountEmailRepository) RequestEmailVerificationTx(ctx context.Context, userID string, now time.Time, cooldown time.Duration) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		verified, err := q.LockUserEmailVerified(ctx, userID)
		if err != nil {
			return fmt.Errorf("ler a verificacao da conta %q: %w", userID, err)
		}
		if verified {
			return auth.ErrEmailAlreadyVerified
		}
		recent, err := q.RecentEmailVerificationExists(ctx, gen.RecentEmailVerificationExistsParams{UserID: userID, Since: now.Add(-cooldown)})
		if err != nil || recent {
			return err
		}
		if err := q.DeleteOpenEmailVerifications(ctx, userID); err != nil {
			return fmt.Errorf("invalidar verificacoes abertas da conta %q: %w", userID, err)
		}
		return enqueueEmailVerification(ctx, q, userID, now)
	})
}

// enqueueEmailVerification cria o pedido de verificação e a notificação dele.
func enqueueEmailVerification(ctx context.Context, q *gen.Queries, userID string, now time.Time) error {
	id, err := q.CreateEmailVerification(ctx, gen.CreateEmailVerificationParams{UserID: userID, CreatedAt: now})
	if err != nil {
		return fmt.Errorf("criar verificacao da conta %q: %w", userID, err)
	}
	return q.EnqueueEmailVerificationNotification(ctx, pgtype.Text{String: id, Valid: true})
}

// ConfirmEmailTx marca a conta dona do token como verificada, uma vez só.
func (r *PostgresAccountEmailRepository) ConfirmEmailTx(ctx context.Context, tokenHash string, now time.Time) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		userID, err := q.UseEmailVerification(ctx, gen.UseEmailVerificationParams{TokenHash: pgtype.Text{String: tokenHash, Valid: true}, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.ErrInvalidToken
		}
		if err != nil {
			return fmt.Errorf("usar token de verificacao: %w", err)
		}
		return q.MarkEmailVerified(ctx, userID)
	})
}

// RequestPasswordResetTx pede a troca para a conta local do email. Email sem conta e
// pedido dentro de cooldown terminam sem erro, para a resposta não revelar nada.
func (r *PostgresAccountEmailRepository) RequestPasswordResetTx(ctx context.Context, email string, now time.Time, cooldown time.Duration) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		userID, err := q.LocalUserIDByEmail(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("buscar conta local de %q: %w", email, err)
		}
		recent, err := q.RecentPasswordResetExists(ctx, gen.RecentPasswordResetExistsParams{UserID: userID, Since: now.Add(-cooldown)})
		if err != nil || recent {
			return err
		}
		return replacePasswordReset(ctx, q, userID, now)
	})
}

func replacePasswordReset(ctx context.Context, q *gen.Queries, userID string, now time.Time) error {
	if err := q.DeleteOpenPasswordResets(ctx, userID); err != nil {
		return fmt.Errorf("invalidar trocas abertas da conta %q: %w", userID, err)
	}
	id, err := q.CreatePasswordReset(ctx, gen.CreatePasswordResetParams{UserID: userID, CreatedAt: now})
	if err != nil {
		return fmt.Errorf("criar troca de senha da conta %q: %w", userID, err)
	}
	return q.EnqueuePasswordResetNotification(ctx, pgtype.Text{String: id, Valid: true})
}

// ResetPasswordTx troca a senha local da conta dona do token, derruba as sessões dela e
// audita, tudo na mesma transação.
func (r *PostgresAccountEmailRepository) ResetPasswordTx(ctx context.Context, p auth.PasswordResetParams) error {
	return inTx(ctx, r.pool, func(q *gen.Queries) error {
		userID, err := q.UsePasswordReset(ctx, gen.UsePasswordResetParams{TokenHash: pgtype.Text{String: p.TokenHash, Valid: true}, Now: p.Now})
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.ErrInvalidToken
		}
		if err != nil {
			return fmt.Errorf("usar token de troca de senha: %w", err)
		}
		if err := q.UpdateLocalPassword(ctx, gen.UpdateLocalPasswordParams{UserID: userID, PasswordHash: pgtype.Text{String: p.PasswordHash, Valid: true}}); err != nil {
			return fmt.Errorf("gravar senha nova da conta %q: %w", userID, err)
		}
		if err := q.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
			return fmt.Errorf("derrubar sessoes da conta %q: %w", userID, err)
		}
		return q.CreateAuditEvent(ctx, toCreateAuditEventParams(auth.AuditEventRecord{
			ID: p.AuditEventID, ActorID: userID, Action: "auth.password_reset", Target: userID, Result: "success", OccurredAt: p.Now,
		}))
	})
}
