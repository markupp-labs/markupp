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

// PostgresNotificationOutbox é a fila de email (convite, verificação e troca de senha),
// lida no papel markupp_notifier.
type PostgresNotificationOutbox struct {
	pool *pgxpool.Pool
}

// NewPostgresNotificationOutbox monta a fila sobre um pool já aberto.
func NewPostgresNotificationOutbox(pool *pgxpool.Pool) *PostgresNotificationOutbox {
	return &PostgresNotificationOutbox{pool: pool}
}

// DeliverNextDue reserva a próxima notificação vencida, entrega com deliver e grava
// o resultado na mesma transação. Erro de deliver desfaz tudo, inclusive a troca do
// hash, e a notificação volta para a fila como estava.
func (o *PostgresNotificationOutbox) DeliverNextDue(ctx context.Context, deliver notify.DeliverFunc) (bool, error) {
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

// accountTokenTTL é a validade de cada tipo de token da conta, contada da entrega.
var accountTokenTTL = map[notify.Kind]time.Duration{
	notify.KindEmailVerification: auth.EmailVerificationTTL,
	notify.KindPasswordReset:     auth.PasswordResetTTL,
}

// reissuer grava o hash do token no alvo da notificação pela função do banco do tipo
// dela, a única porta do notificador para convites e tokens da conta.
func reissuer(q *gen.Queries, p notify.Pending) notify.ReissueFunc {
	if p.Kind == notify.KindInvite {
		return func(ctx context.Context, tokenHash string) (notify.TokenDetails, bool, error) {
			return reissueInvite(ctx, q, p.TargetID, tokenHash)
		}
	}
	return func(ctx context.Context, tokenHash string) (notify.TokenDetails, bool, error) {
		return reissueAccountToken(ctx, q, p, tokenHash)
	}
}

func reissueInvite(ctx context.Context, q *gen.Queries, inviteID, tokenHash string) (notify.TokenDetails, bool, error) {
	rows, err := q.ReissueInviteToken(ctx, gen.ReissueInviteTokenParams{InviteID: inviteID, TokenHash: tokenHash})
	if err != nil {
		return notify.TokenDetails{}, false, fmt.Errorf("trocar hash do convite %q: %w", inviteID, err)
	}
	if len(rows) == 0 {
		return notify.TokenDetails{}, false, nil
	}
	r := rows[0]
	return notify.TokenDetails{
		Email: r.InviteEmail, Role: r.InviteRole, TenantName: r.TenantName,
		InviterEmail: r.InviterEmail, ExpiresAt: r.InviteExpiresAt,
	}, true, nil
}

func reissueAccountToken(ctx context.Context, q *gen.Queries, p notify.Pending, tokenHash string) (notify.TokenDetails, bool, error) {
	ttl, ok := accountTokenTTL[p.Kind]
	if !ok {
		return notify.TokenDetails{}, false, fmt.Errorf("notificacao %q do tipo %q, esperado %q ou %q", p.NotificationID, p.Kind, notify.KindEmailVerification, notify.KindPasswordReset)
	}
	rows, err := q.ReissueAccountToken(ctx, gen.ReissueAccountTokenParams{ID: p.TargetID, Purpose: string(p.Kind), TokenHash: tokenHash, Ttl: interval(ttl)})
	if err != nil {
		return notify.TokenDetails{}, false, fmt.Errorf("trocar hash do token %q: %w", p.TargetID, err)
	}
	if len(rows) == 0 {
		return notify.TokenDetails{}, false, nil
	}
	return notify.TokenDetails{Email: rows[0].AccountEmail, ExpiresAt: rows[0].TokenExpiresAt}, true, nil
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
