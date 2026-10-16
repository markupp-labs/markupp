package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// Deliverer entrega o convite: gera o token, grava só o hash dele no convite e
// envia o token por email.
type Deliverer struct {
	mailer   Mailer
	from     string
	newToken func() (string, error)
	clock    func() time.Time
}

// NewDeliverer monta a entrega sobre o servidor de email, o remetente, o gerador de
// token e o relógio dados.
func NewDeliverer(mailer Mailer, from string, newToken func() (string, error), clock func() time.Time) *Deliverer {
	return &Deliverer{mailer: mailer, from: from, newToken: newToken, clock: clock}
}

// Deliver é a DeliverFunc das notificações. Falha do servidor de email vira nova
// tentativa com espera crescente, e pedido que não está mais pendente é abandonado
// sem envio.
func (d *Deliverer) Deliver(ctx context.Context, p Pending, reissue ReissueFunc) (Outcome, error) {
	compose, err := composerFor(p.Kind)
	if err != nil {
		return Outcome{}, err
	}
	token, err := d.newToken()
	if err != nil {
		return Outcome{}, err
	}
	details, found, err := reissue(ctx, auth.HashOpaqueToken(token))
	if err != nil {
		return Outcome{}, err
	}
	if !found {
		return Outcome{GiveUp: true, Error: "pedido respondido, revogado ou vencido"}, nil
	}
	if err := d.mailer.Send(ctx, compose(details, token, d.from)); err != nil {
		return d.failure(p.Attempts+1, err), nil
	}
	return Outcome{Sent: true}, nil
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
