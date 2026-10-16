package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// ClaimNextDue reserva a próxima notificação vencida por lease e troca o hash do
// pedido dela por tokenHash, numa transação que termina antes do envio: o lock no
// pedido não espera o servidor de email.
func (o *PostgresNotificationOutbox) ClaimNextDue(ctx context.Context, tokenHash string, lease time.Duration) (notify.Claim, bool, error) {
	var claim notify.Claim
	found := false
	err := o.inNotifierTx(ctx, func(q *gen.Queries) error {
		row, err := q.ClaimDueNotification(ctx, int32(lease.Seconds()))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reservar notificacao vencida: %w", err)
		}
		found = true
		claim, err = reissue(ctx, q, row, tokenHash)
		return err
	})
	return claim, found, err
}

// accountTokenTTL é a validade de cada tipo de token da conta, contada da entrega.
var accountTokenTTL = map[notify.Kind]time.Duration{
	notify.KindEmailVerification: auth.EmailVerificationTTL,
	notify.KindPasswordReset:     auth.PasswordResetTTL,
}

// reissue troca o hash do pedido da notificação pela função do banco do tipo dela, a
// única porta do notificador para convites e tokens da conta.
func reissue(ctx context.Context, q *gen.Queries, row gen.ClaimDueNotificationRow, tokenHash string) (notify.Claim, error) {
	claim := notify.Claim{NotificationID: row.ID, Kind: notify.Kind(row.Kind), Attempt: int(row.Attempts)}
	if claim.Kind == notify.KindInvite {
		return reissueInvite(ctx, q, claim, row.TargetID, tokenHash)
	}
	return reissueAccountToken(ctx, q, claim, row.TargetID, tokenHash)
}

func reissueInvite(ctx context.Context, q *gen.Queries, claim notify.Claim, inviteID, tokenHash string) (notify.Claim, error) {
	rows, err := q.ReissueInviteToken(ctx, gen.ReissueInviteTokenParams{InviteID: inviteID, TokenHash: tokenHash})
	if err != nil {
		return notify.Claim{}, fmt.Errorf("trocar o hash do convite %q: %w", inviteID, err)
	}
	if len(rows) == 0 {
		return claim, nil
	}
	r := rows[0]
	claim.Pending = true
	claim.Details = notify.TokenDetails{
		Email: r.InviteEmail, Role: r.InviteRole, TenantName: r.TenantName,
		InviterEmail: r.InviterEmail, ExpiresAt: r.InviteExpiresAt,
	}
	return claim, nil
}

func reissueAccountToken(ctx context.Context, q *gen.Queries, claim notify.Claim, tokenID, tokenHash string) (notify.Claim, error) {
	ttl, ok := accountTokenTTL[claim.Kind]
	if !ok {
		return notify.Claim{}, fmt.Errorf("notificacao %q do tipo %q, esperado %q, %q ou %q",
			claim.NotificationID, claim.Kind, notify.KindInvite, notify.KindEmailVerification, notify.KindPasswordReset)
	}
	rows, err := q.ReissueAccountToken(ctx, gen.ReissueAccountTokenParams{ID: tokenID, Purpose: string(claim.Kind), TokenHash: tokenHash, Ttl: interval(ttl)})
	if err != nil {
		return notify.Claim{}, fmt.Errorf("trocar o hash do token %q: %w", tokenID, err)
	}
	if len(rows) == 0 {
		return claim, nil
	}
	claim.Pending = true
	claim.Details = notify.TokenDetails{Email: rows[0].AccountEmail, ExpiresAt: rows[0].TokenExpiresAt}
	return claim, nil
}

func interval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

// Record grava o resultado da entrega reservada, que substitui o lease.
func (o *PostgresNotificationOutbox) Record(ctx context.Context, notificationID string, outcome notify.Outcome) error {
	return o.inNotifierTx(ctx, func(q *gen.Queries) error {
		return recordOutcome(ctx, q, notificationID, outcome)
	})
}

func (o *PostgresNotificationOutbox) inNotifierTx(ctx context.Context, fn func(q *gen.Queries) error) error {
	return inTx(ctx, o.pool, func(q *gen.Queries) error {
		if err := q.EnterNotifier(ctx); err != nil {
			return fmt.Errorf("entrar no papel do notificador: %w", err)
		}
		return fn(q)
	})
}

func recordOutcome(ctx context.Context, q *gen.Queries, id string, outcome notify.Outcome) error {
	lastError := pgtype.Text{String: storableText(outcome.Error), Valid: outcome.Error != ""}
	switch {
	case outcome.Sent:
		return q.MarkNotificationSent(ctx, id)
	case outcome.GiveUp:
		return q.MarkNotificationFailed(ctx, gen.MarkNotificationFailedParams{ID: id, LastError: lastError})
	}
	return q.MarkNotificationRetry(ctx, gen.MarkNotificationRetryParams{ID: id, NextAttemptAt: outcome.RetryAt, LastError: lastError})
}

// storableText limpa o que o PostgreSQL recusa num TEXT: bytes fora do UTF-8, como a
// resposta de um relay em Latin-1, e o byte nulo.
func storableText(raw string) string {
	return strings.ReplaceAll(strings.ToValidUTF8(raw, "\uFFFD"), "\x00", "")
}
