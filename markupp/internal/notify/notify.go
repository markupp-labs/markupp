// Package notify entrega por email, fora do caminho da requisição, o que a API
// deixou numa fila no banco (ADR-0021). O primeiro uso é o convite: o token é gerado
// na entrega, e só o hash dele volta para o banco.
package notify

import (
	"context"
	"fmt"
	"time"
)

// Message é um email com a mesma mensagem em texto puro e em HTML.
type Message struct {
	From    string
	To      string
	Subject string
	Body    string
	HTML    string
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

// TokenDetails é o que o email mostra junto do token. Os campos de convite só vêm
// preenchidos na notificação de convite.
type TokenDetails struct {
	Email        string
	Role         string
	TenantName   string
	InviterEmail string
	ExpiresAt    time.Time
}

// Claim é a notificação reservada para entrega, com o hash do token novo já gravado
// no pedido dela: o convite ou o token da conta.
type Claim struct {
	NotificationID string
	Kind           Kind
	// Attempt é o número desta tentativa, contada já na reserva.
	Attempt int
	Details TokenDetails
	// Pending é falso quando o pedido não está mais aberto: o convite respondido,
	// revogado, vencido ou sem quem convidou no papel, ou o token da conta usado,
	// trocado por um reenvio ou de email já verificado. Nada a enviar, e o hash fica.
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

// InviteMessage monta o email que leva o token do convite e, com panelURL, o link
// do painel para aceitar.
func InviteMessage(d TokenDetails, token, from, panelURL string) Message {
	// O nome do tenant é escolhido por quem convida: no assunto, ele seria a isca de um
	// phishing que sai do remetente da instância (#50). No corpo, vem com quem convidou.
	return letter{
		subject: "Convite para um tenant no markupp",
		lead:    fmt.Sprintf("%s convidou você para %s no markupp, com o papel %s.", d.InviterEmail, d.TenantName, d.Role),
		route:   inviteLink,
		usage:   "Use o token no cliente do markupp para aceitar. O convite vale até " + validUntil(d.ExpiresAt) + ".",
		expiry:  "O convite vale até " + validUntil(d.ExpiresAt) + ".",
		reason:  "este endereço foi convidado para um tenant.",
	}.message(from, d.Email, token, panelURL)
}

// VerifyEmailMessage monta o email que leva o token de verificação da conta e,
// com panelURL, o link do painel para confirmar.
func VerifyEmailMessage(d TokenDetails, token, from, panelURL string) Message {
	return letter{
		subject: "Confirme seu email no markupp",
		lead:    "Confirme que este email é seu para receber convites no markupp.",
		route:   verifyLink,
		usage:   "Use o token no cliente do markupp, logado na conta que você criou. Ele vale até " + validUntil(d.ExpiresAt) + ".",
		expiry:  "Abra o link logado na conta que você criou. Ele vale até " + validUntil(d.ExpiresAt) + ".",
		caution: "Se não foi você que criou a conta, ignore este email e não use o token.",
		reason:  "uma conta do markupp foi criada com este endereço.",
	}.message(from, d.Email, token, panelURL)
}

// ResetPasswordMessage monta o email que leva o token de troca de senha e, com
// panelURL, o link do painel para trocar.
func ResetPasswordMessage(d TokenDetails, token, from, panelURL string) Message {
	return letter{
		subject: "Troca de senha no markupp",
		lead:    "Alguém pediu a troca da senha da sua conta no markupp.",
		route:   resetLink,
		usage:   "Use o token no cliente do markupp. Ele vale até " + validUntil(d.ExpiresAt) + ".",
		expiry:  "O link vale até " + validUntil(d.ExpiresAt) + ".",
		reason:  "pediram a troca da senha da conta deste endereço.",
	}.message(from, d.Email, token, panelURL)
}

// validUntil mostra a validade com hora e fuso, em UTC.
func validUntil(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 MST")
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
