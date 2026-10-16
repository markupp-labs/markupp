package notify

import (
	"context"
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

// Deliver é a DeliverFunc do convite. Falha do servidor de email vira nova tentativa
// com espera crescente, e convite que não está mais pendente é abandonado sem envio.
func (d *Deliverer) Deliver(ctx context.Context, p Pending, reissue ReissueFunc) (Outcome, error) {
	token, err := d.newToken()
	if err != nil {
		return Outcome{}, err
	}
	details, found, err := reissue(ctx, auth.HashOpaqueToken(token))
	if err != nil {
		return Outcome{}, err
	}
	if !found {
		return Outcome{GiveUp: true, Error: "convite respondido, revogado ou vencido"}, nil
	}
	if err := d.mailer.Send(ctx, InviteMessage(details, token, d.from)); err != nil {
		return d.failure(p.Attempts+1, err), nil
	}
	return Outcome{Sent: true}, nil
}

func (d *Deliverer) failure(attempts int, err error) Outcome {
	retryAt, retry := NextAttempt(attempts, d.clock())
	return Outcome{GiveUp: !retry, RetryAt: retryAt, Error: err.Error()}
}
