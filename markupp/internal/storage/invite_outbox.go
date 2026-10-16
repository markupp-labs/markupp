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

// ClaimNextDue reserva a próxima notificação vencida por lease e troca o hash do
// convite por tokenHash, numa transação que termina antes do envio: o lock no convite
// não espera o servidor de email.
func (o *PostgresInviteOutbox) ClaimNextDue(ctx context.Context, tokenHash string, lease time.Duration) (notify.Claim, bool, error) {
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

// reissue troca o hash do convite da notificação pela função do banco, a única porta
// do notificador para a tabela de convites.
func reissue(ctx context.Context, q *gen.Queries, row gen.ClaimDueNotificationRow, tokenHash string) (notify.Claim, error) {
	claim := notify.Claim{NotificationID: row.ID, Attempt: int(row.Attempts)}
	rows, err := q.ReissueInviteToken(ctx, gen.ReissueInviteTokenParams{InviteID: row.InviteID, TokenHash: tokenHash})
	if err != nil {
		return notify.Claim{}, fmt.Errorf("trocar o hash do convite %q: %w", row.InviteID, err)
	}
	if len(rows) == 0 {
		return claim, nil
	}
	r := rows[0]
	claim.Pending = true
	claim.Details = notify.InviteDetails{
		Email: r.InviteEmail, Role: r.InviteRole, TenantName: r.TenantName,
		InviterEmail: r.InviterEmail, ExpiresAt: r.InviteExpiresAt,
	}
	return claim, nil
}

// Record grava o resultado da entrega reservada, que substitui o lease.
func (o *PostgresInviteOutbox) Record(ctx context.Context, notificationID string, outcome notify.Outcome) error {
	return o.inNotifierTx(ctx, func(q *gen.Queries) error {
		return recordOutcome(ctx, q, notificationID, outcome)
	})
}

func (o *PostgresInviteOutbox) inNotifierTx(ctx context.Context, fn func(q *gen.Queries) error) error {
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
