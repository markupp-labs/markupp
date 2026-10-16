// Package notify entrega por email, fora do caminho da requisição, o que a API
// deixou numa fila no banco (ADR-0021). O primeiro uso é o convite: o token é gerado
// na entrega, e só o hash dele volta para o banco.
package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/drain"
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

// Kind é o tipo da notificação, que decide a mensagem e o pedido cujo token ela leva.
type Kind string

// Tipos de notificação.
const (
	KindInvite            Kind = "invite"
	KindEmailVerification Kind = "email_verification"
	KindPasswordReset     Kind = "password_reset"
)

// Pending é a notificação reservada para entrega.
type Pending struct {
	NotificationID string
	Kind           Kind
	// TargetID é o convite, a verificação ou a troca de senha da notificação.
	TargetID string
	// Attempts conta as entregas que já falharam.
	Attempts int
}

// TokenDetails é o que o email mostra junto do token. Os campos de convite só vêm
// preenchidos na notificação de convite.
type TokenDetails struct {
	Email        string
	Role         string
	TenantName   string
	InviterEmail string
	ExpiresAt    time.Time
}

// ReissueFunc grava tokenHash no convite ainda pendente e devolve os detalhes dele,
// ou found falso quando o convite já foi respondido, revogado ou venceu.
type ReissueFunc func(ctx context.Context, tokenHash string) (details TokenDetails, found bool, err error)

// Outcome é o resultado de uma entrega: enviada, remarcada para RetryAt ou abandonada.
type Outcome struct {
	Sent    bool
	GiveUp  bool
	RetryAt time.Time
	Error   string
}

// DeliverFunc entrega uma notificação reservada. Erro desfaz a transação inteira, e a
// notificação volta para a fila como estava.
type DeliverFunc func(ctx context.Context, p Pending, reissue ReissueFunc) (Outcome, error)

// Outbox reserva a próxima notificação vencida, entrega com deliver e grava o
// resultado na mesma transação. Devolve false quando não há notificação vencida livre.
type Outbox interface {
	DeliverNextDue(ctx context.Context, deliver DeliverFunc) (bool, error)
}

// InviteMessage monta o email que leva o token do convite.
func InviteMessage(d TokenDetails, token, from string) Message {
	body := fmt.Sprintf("%s convidou você para %s no markupp, com o papel %s.\n\n"+
		"Token do convite:\n%s\n\n"+
		"Use o token no cliente do markupp para aceitar. O convite vale até %s.\n",
		d.InviterEmail, d.TenantName, d.Role, token, d.ExpiresAt.UTC().Format("2006-01-02"))
	return Message{From: from, To: d.Email, Subject: "Convite para " + d.TenantName + " no markupp", Body: body}
}

// VerifyEmailMessage monta o email que leva o token de verificação da conta.
func VerifyEmailMessage(d TokenDetails, token, from string) Message {
	body := fmt.Sprintf("Confirme que este email é seu para receber convites no markupp.\n\n"+
		"Token de verificação:\n%s\n\n"+
		"Use o token no cliente do markupp. Ele vale até %s.\n",
		token, untilMinute(d.ExpiresAt))
	return Message{From: from, To: d.Email, Subject: "Confirme seu email no markupp", Body: body}
}

// ResetPasswordMessage monta o email que leva o token de troca de senha.
func ResetPasswordMessage(d TokenDetails, token, from string) Message {
	body := fmt.Sprintf("Alguém pediu a troca da senha da sua conta no markupp.\n\n"+
		"Token de troca de senha:\n%s\n\n"+
		"Use o token no cliente do markupp. Ele vale até %s.\n\n"+
		"Se não foi você, ignore este email: a senha só muda com o token.\n",
		token, untilMinute(d.ExpiresAt))
	return Message{From: from, To: d.Email, Subject: "Troca de senha no markupp", Body: body}
}

func untilMinute(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04") + " UTC"
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
	return drain.Run(ctx, func(ctx context.Context) (bool, error) {
		found, err := outbox.DeliverNextDue(ctx, deliver)
		if err != nil {
			return false, fmt.Errorf("entregar proxima notificacao: %w", err)
		}
		return found, nil
	}, wait)
}
