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

// PostgresInviteOutbox é a fila do email de convite, lida no papel markupp_notifier.
type PostgresInviteOutbox struct {
	pool *pgxpool.Pool
}

// NewPostgresInviteOutbox monta a fila sobre um pool já aberto.
func NewPostgresInviteOutbox(pool *pgxpool.Pool) *PostgresInviteOutbox {
	return &PostgresInviteOutbox{pool: pool}
}

// DeliverNextDue reserva a próxima notificação vencida, entrega com deliver e grava
// o resultado na mesma transação. Erro de deliver desfaz tudo, inclusive a troca do
// hash, e a notificação volta para a fila como estava.
func (o *PostgresInviteOutbox) DeliverNextDue(ctx context.Context, deliver notify.DeliverFunc) (bool, error) {
	found := false
	err := inTx(ctx, o.pool, func(q *gen.Queries) error {
		if err := q.EnterNotifier(ctx); err != nil {
			return fmt.Errorf("entrar no papel do notificador: %w", err)
		}
		row, err := q.ReserveDueNotification(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reservar notificacao vencida: %w", err)
		}
		found = true
		pending := notify.Pending{NotificationID: row.ID, Kind: notify.Kind(row.Kind), TargetID: row.TargetID, Attempts: int(row.Attempts)}
		outcome, err := deliver(ctx, pending, reissuer(q, pending))
		if err != nil {
			return err
		}
		return recordOutcome(ctx, q, row.ID, outcome)
	})
	return found, err
}

// reissuer grava o hash do token no pedido da notificação pela função do banco do
// tipo dela, a única porta do notificador para as tabelas de pedidos.
func reissuer(q *gen.Queries, p notify.Pending) notify.ReissueFunc {
	return func(ctx context.Context, tokenHash string) (notify.TokenDetails, bool, error) {
		switch p.Kind {
		case notify.KindEmailVerification:
			rows, err := q.ReissueEmailVerification(ctx, gen.ReissueEmailVerificationParams{ID: p.TargetID, TokenHash: tokenHash, Ttl: interval(auth.EmailVerificationTTL)})
			return firstAccountDetails(rows, err, func(r gen.ReissueEmailVerificationRow) notify.TokenDetails {
				return notify.TokenDetails{Email: r.AccountEmail, ExpiresAt: r.TokenExpiresAt}
			})
		case notify.KindPasswordReset:
			rows, err := q.ReissuePasswordReset(ctx, gen.ReissuePasswordResetParams{ID: p.TargetID, TokenHash: tokenHash, Ttl: interval(auth.PasswordResetTTL)})
			return firstAccountDetails(rows, err, func(r gen.ReissuePasswordResetRow) notify.TokenDetails {
				return notify.TokenDetails{Email: r.AccountEmail, ExpiresAt: r.TokenExpiresAt}
			})
		}
		return reissueInvite(ctx, q, p.TargetID, tokenHash)
	}
}

func reissueInvite(ctx context.Context, q *gen.Queries, inviteID, tokenHash string) (notify.TokenDetails, bool, error) {
	rows, err := q.ReissueInviteToken(ctx, gen.ReissueInviteTokenParams{InviteID: inviteID, TokenHash: tokenHash})
	if err != nil || len(rows) == 0 {
		return notify.TokenDetails{}, false, err
	}
	r := rows[0]
	return notify.TokenDetails{
		Email: r.InviteEmail, Role: r.InviteRole, TenantName: r.TenantName,
		InviterEmail: r.InviterEmail, ExpiresAt: r.InviteExpiresAt,
	}, true, nil
}

// firstAccountDetails lê a única linha que a troca de hash de um pedido da conta
// devolve, ou found falso quando o pedido não está mais aberto.
func firstAccountDetails[T any](rows []T, err error, details func(T) notify.TokenDetails) (notify.TokenDetails, bool, error) {
	if err != nil || len(rows) == 0 {
		return notify.TokenDetails{}, false, err
	}
	return details(rows[0]), true, nil
}

func interval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

func recordOutcome(ctx context.Context, q *gen.Queries, id string, outcome notify.Outcome) error {
	lastError := pgtype.Text{String: outcome.Error, Valid: outcome.Error != ""}
	switch {
	case outcome.Sent:
		return q.MarkNotificationSent(ctx, id)
	case outcome.GiveUp:
		return q.MarkNotificationFailed(ctx, gen.MarkNotificationFailedParams{ID: id, LastError: lastError})
	}
	return q.MarkNotificationRetry(ctx, gen.MarkNotificationRetryParams{ID: id, NextAttemptAt: outcome.RetryAt, LastError: lastError})
}
