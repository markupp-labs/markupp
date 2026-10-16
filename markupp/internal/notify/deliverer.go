package notify

import (
	"context"
	"fmt"
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
	newToken func() (string, error)
	clock    func() time.Time
}

// NewDeliverer monta a entrega sobre a fila, o servidor de email, o remetente, o
// gerador de token e o relógio dados.
func NewDeliverer(outbox Outbox, mailer Mailer, from string, newToken func() (string, error), clock func() time.Time) *Deliverer {
	return &Deliverer{outbox: outbox, mailer: mailer, from: from, newToken: newToken, clock: clock}
}

// DeliverNext reserva a próxima notificação vencida, envia fora de transação e grava
// o resultado. Devolve false com a fila vazia, e erro só do banco: falha do servidor
// de email vira nova tentativa com espera crescente.
func (d *Deliverer) DeliverNext(ctx context.Context) (bool, error) {
	token, err := d.newToken()
	if err != nil {
		return false, fmt.Errorf("gerar token da notificacao: %w", err)
	}
	claim, found, err := d.outbox.ClaimNextDue(ctx, auth.HashOpaqueToken(token), claimLease)
	if err != nil || !found {
		return false, err
	}
	// ponytail: entrega pelo menos uma vez. Se o envio passa e a gravação falha, o
	// lease vence e o reenvio leva outro token, que invalida o do primeiro email.
	return true, d.outbox.Record(ctx, claim.NotificationID, d.attempt(ctx, claim, token))
}

func (d *Deliverer) attempt(ctx context.Context, claim Claim, token string) Outcome {
	compose, err := composerFor(claim.Kind)
	if err != nil {
		return Outcome{GiveUp: true, Error: err.Error()}
	}
	if !claim.Pending {
		return Outcome{GiveUp: true, Error: fmt.Sprintf("pedido %s %q nao esta mais aberto", claim.Kind, claim.NotificationID)}
	}
	if claim.Attempt > maxAttempts {
		return Outcome{GiveUp: true, Error: fmt.Sprintf("tentativa %d, esperado no maximo %d", claim.Attempt, maxAttempts)}
	}
	if err := d.mailer.Send(ctx, compose(claim.Details, token, d.from)); err != nil {
		return d.failure(claim.Attempt, err)
	}
	return Outcome{Sent: true}
}

func (d *Deliverer) failure(attempts int, err error) Outcome {
	retryAt, retry := NextAttempt(attempts, d.clock())
	return Outcome{GiveUp: !retry, RetryAt: retryAt, Error: err.Error()}
}

func composerFor(kind Kind) (func(TokenDetails, string, string) Message, error) {
	switch kind {
	case KindInvite:
		return InviteMessage, nil
	case KindEmailVerification:
		return VerifyEmailMessage, nil
	case KindPasswordReset:
		return ResetPasswordMessage, nil
	}
	return nil, fmt.Errorf("notificacao do tipo %q, esperado %q, %q ou %q", kind, KindInvite, KindEmailVerification, KindPasswordReset)
}
