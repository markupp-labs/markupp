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

// InviteDetails é o que o email do convite mostra.
type InviteDetails struct {
	Email        string
	Role         string
	TenantName   string
	InviterEmail string
	ExpiresAt    time.Time
}

// Claim é a notificação reservada para entrega, com o hash do token novo já gravado
// no convite.
type Claim struct {
	NotificationID string
	// Attempt é o número desta tentativa, contada já na reserva.
	Attempt int
	Details InviteDetails
	// Pending é falso quando o convite já foi respondido, revogado, venceu ou perdeu
	// quem o convidou com o papel dele: nada a enviar, e o hash fica como estava.
	Pending bool
}

// Outcome é o resultado de uma entrega: enviada, remarcada para RetryAt ou abandonada.
type Outcome struct {
	Sent    bool
	GiveUp  bool
	RetryAt time.Time
	Error   string
}

// Outbox é a fila do email de convite no banco. Cada chamada é uma transação curta,
// e o envio acontece entre as duas, sem lock nenhum.
type Outbox interface {
	// ClaimNextDue reserva a próxima notificação vencida por lease, conta a tentativa
	// e troca o hash do convite por tokenHash. Devolve false com a fila vazia.
	ClaimNextDue(ctx context.Context, tokenHash string, lease time.Duration) (Claim, bool, error)
	// Record grava o resultado da entrega reservada.
	Record(ctx context.Context, notificationID string, outcome Outcome) error
}

// InviteMessage monta o email que leva o token do convite.
func InviteMessage(d InviteDetails, token, from string) Message {
	body := fmt.Sprintf("%s convidou você para %s no markupp, com o papel %s.\n\n"+
		"Token do convite:\n%s\n\n"+
		"Use o token no cliente do markupp para aceitar. O convite vale até %s.\n",
		d.InviterEmail, d.TenantName, d.Role, token, d.ExpiresAt.UTC().Format("2006-01-02 15:04 MST"))
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
