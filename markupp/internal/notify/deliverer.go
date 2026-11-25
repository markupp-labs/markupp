package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// claimLease segura a notificação reservada longe de outra réplica enquanto o envio
// acontece. Cobre com folga a sessão SMTP mais lenta, de até um minuto, e é a espera
// para devolver à fila a notificação de um notificador que caiu no meio do envio.
const claimLease = 5 * time.Minute

// Deliverer entrega as notificações: gera o token, grava só o hash dele no pedido
// (convite ou token da conta) e envia o token por email.
type Deliverer struct {
	outbox   Outbox
	mailer   Mailer
	from     string
	panelURL string
	newToken func() (string, error)
	clock    func() time.Time
	logger   *slog.Logger
}

// NewDeliverer monta a entrega sobre a fila, o servidor de email, o remetente, a URL
// base do painel (vazia deixa o email sem link), o gerador de token, o relógio e o
// log onde cada entrega fica registrada.
func NewDeliverer(outbox Outbox, mailer Mailer, from, panelURL string, newToken func() (string, error), clock func() time.Time, logger *slog.Logger) *Deliverer {
	return &Deliverer{outbox: outbox, mailer: mailer, from: from, panelURL: panelURL, newToken: newToken, clock: clock, logger: logger}
}

// DeliverNext reserva a próxima notificação vencida, envia fora de transação e grava
// o resultado. Devolve false com a fila vazia, e erro só do banco: falha do servidor
// de email vira nova tentativa com espera crescente.
func (d *Deliverer) DeliverNext(ctx context.Context) (bool, error) {
	token, err := d.newToken()
	if err != nil {
		return false, fmt.Errorf("generate notification token: %w", err)
	}
	claim, found, err := d.outbox.ClaimNextDue(ctx, auth.HashOpaqueToken(token), claimLease)
	if err != nil || !found {
		return false, err
	}
	// ponytail: entrega pelo menos uma vez. Se o envio passa e a gravação falha, o
	// lease vence e o reenvio leva outro token, que invalida o do primeiro email.
	outcome := d.attempt(ctx, claim, token)
	d.logOutcome(claim, outcome)
	return true, d.outbox.Record(ctx, claim.NotificationID, outcome)
}

// logOutcome registra a entrega sem o token e sem o endereço de quem recebe, que o
// erro do servidor de email costuma repetir.
func (d *Deliverer) logOutcome(claim Claim, outcome Outcome) {
	logger := d.logger.With("notificacao", claim.NotificationID, "tipo", string(claim.Kind), "tentativa", claim.Attempt)
	if outcome.Sent {
		logger.Info("entrega de notificacao", "resultado", "enviada")
		return
	}
	reason := withoutRecipient(outcome.Error, claim.Details.Email)
	if outcome.GiveUp {
		logger.Warn("entrega de notificacao", "resultado", "desistencia", "err", reason)
		return
	}
	logger.Warn("entrega de notificacao", "resultado", "nova_tentativa", "nova_tentativa_em", outcome.RetryAt, "err", reason)
}

func withoutRecipient(text, email string) string {
	if email == "" {
		return text
	}
	return strings.ReplaceAll(text, email, "<destinatario>")
}

func (d *Deliverer) attempt(ctx context.Context, claim Claim, token string) Outcome {
	compose, err := composerFor(claim.Kind)
	if err != nil {
		return Outcome{GiveUp: true, Error: err.Error()}
	}
	if !claim.Pending {
		return Outcome{GiveUp: true, Error: fmt.Sprintf("%s request %q is no longer open", claim.Kind, claim.NotificationID)}
	}
	if claim.Attempt > maxAttempts {
		return Outcome{GiveUp: true, Error: fmt.Sprintf("attempt %d, expected at most %d", claim.Attempt, maxAttempts)}
	}
	if err := d.mailer.Send(ctx, compose(claim.Details, token, d.from, d.panelURL)); err != nil {
		return d.failure(claim.Attempt, err)
	}
	return Outcome{Sent: true}
}

func (d *Deliverer) failure(attempts int, err error) Outcome {
	retryAt, retry := NextAttempt(attempts, d.clock())
	return Outcome{GiveUp: !retry, RetryAt: retryAt, Error: err.Error()}
}

func composerFor(kind Kind) (func(TokenDetails, string, string, string) Message, error) {
	switch kind {
	case KindInvite:
		return InviteMessage, nil
	case KindEmailVerification:
		return VerifyEmailMessage, nil
	case KindPasswordReset:
		return ResetPasswordMessage, nil
	}
	return nil, fmt.Errorf("notification kind %q, expected %q, %q or %q", kind, KindInvite, KindEmailVerification, KindPasswordReset)
}
