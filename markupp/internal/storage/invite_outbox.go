package storage

import (
	"context"
	"errors"
	"fmt"

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
		pending := notify.Pending{NotificationID: row.ID, InviteID: row.InviteID, Attempts: int(row.Attempts)}
		outcome, err := deliver(ctx, pending, reissuer(q, row.InviteID))
		if err != nil {
			return err
		}
		return recordOutcome(ctx, q, row.ID, outcome)
	})
	return found, err
}

// reissuer troca o hash do convite inviteID pela função do banco, a única porta do
// notificador para a tabela de convites.
func reissuer(q *gen.Queries, inviteID string) notify.ReissueFunc {
	return func(ctx context.Context, tokenHash string) (notify.TokenDetails, bool, error) {
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
