// Package notify entrega por email, fora do caminho da requisição, o que a API
// deixou numa fila no banco (ADR-0021). O primeiro uso é o convite: o token é gerado
// na entrega, e só o hash dele volta para o banco.
package notify

import (
	"context"
	"fmt"
	"time"
)

// Message é um email de texto puro.
type Message struct {
	From    string
	To      string
	Subject string
	Body    string
}

// Mailer entrega uma mensagem a um servidor de email.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// PendingInvite é a notificação de convite reservada para entrega.
type PendingInvite struct {
	NotificationID string
	InviteID       string
	// Attempts conta as entregas que já falharam.
	Attempts int
}

// InviteDetails é o que o email do convite mostra.
type InviteDetails struct {
	Email        string
	Role         string
	TenantName   string
	InviterEmail string
	ExpiresAt    time.Time
}

// ReissueFunc grava tokenHash no convite ainda pendente e devolve os detalhes dele,
// ou found falso quando o convite já foi respondido, revogado ou venceu.
type ReissueFunc func(ctx context.Context, tokenHash string) (details InviteDetails, found bool, err error)

// Outcome é o resultado de uma entrega: enviada, remarcada para RetryAt ou abandonada.
type Outcome struct {
	Sent    bool
	GiveUp  bool
	RetryAt time.Time
	Error   string
}

// DeliverFunc entrega uma notificação reservada. Erro desfaz a transação inteira, e a
// notificação volta para a fila como estava.
type DeliverFunc func(ctx context.Context, p PendingInvite, reissue ReissueFunc) (Outcome, error)

// Outbox reserva a próxima notificação vencida, entrega com deliver e grava o
// resultado na mesma transação. Devolve false quando não há notificação vencida livre.
type Outbox interface {
	DeliverNextDue(ctx context.Context, deliver DeliverFunc) (bool, error)
}

// InviteMessage monta o email que leva o token do convite.
func InviteMessage(d InviteDetails, token, from string) Message {
	body := fmt.Sprintf("%s convidou você para %s no markupp, com o papel %s.\n\n"+
		"Token do convite:\n%s\n\n"+
		"Use o token no cliente do markupp para aceitar. O convite vale até %s.\n",
		d.InviterEmail, d.TenantName, d.Role, token, d.ExpiresAt.UTC().Format("2006-01-02"))
	return Message{From: from, To: d.Email, Subject: "Convite para " + d.TenantName + " no markupp", Body: body}
}

const (
	maxAttempts  = 8
	firstBackoff = time.Minute
	maxBackoff   = time.Hour
)

// NextAttempt devolve quando tentar de novo depois da falha número attempts: um
// minuto, dobrando até uma hora. Na oitava falha, devolve false e a entrega desiste.
func NextAttempt(attempts int, now time.Time) (time.Time, bool) {
	if attempts >= maxAttempts {
		return time.Time{}, false
	}
	backoff := firstBackoff << (attempts - 1)
	return now.Add(min(backoff, maxBackoff)), true
}

// RunLoop entrega as notificações vencidas até não sobrar nenhuma, chama wait e
// recomeça. Termina no primeiro erro do banco ou de wait.
func RunLoop(ctx context.Context, outbox Outbox, deliver DeliverFunc, wait func(context.Context) error) error {
	for {
		found, err := outbox.DeliverNextDue(ctx, deliver)
		if err != nil {
			return fmt.Errorf("entregar proxima notificacao: %w", err)
		}
		if found {
			continue
		}
		if err := wait(ctx); err != nil {
			return err
		}
	}
}
