// Package notify entrega por email, fora do caminho da requisição, o que a API
// deixou numa fila no banco (ADR-0021). O primeiro uso é o convite: o token é gerado
// na entrega, e só o hash dele volta para o banco.
package notify

import (
	"context"
	"fmt"
	"net/url"
	"strings"
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
	body := fmt.Sprintf("%s convidou você para %s no markupp, com o papel %s.\n\n"+
		"%sToken do convite:\n%s\n\n"+
		"Use o token no cliente do markupp para aceitar. O convite vale até %s.\n",
		d.InviterEmail, d.TenantName, d.Role, panelLink(panelURL, "/invite_acceptance/new", token), token, validUntil(d.ExpiresAt))
	return Message{From: from, To: d.Email, Subject: "Convite para " + d.TenantName + " no markupp", Body: body}
}

// VerifyEmailMessage monta o email que leva o token de verificação da conta e,
// com panelURL, o link do painel para confirmar.
func VerifyEmailMessage(d TokenDetails, token, from, panelURL string) Message {
	body := fmt.Sprintf("Confirme que este email é seu para receber convites no markupp.\n\n"+
		"%sToken de verificação:\n%s\n\n"+
		"Use o token no cliente do markupp, logado na conta que você criou. Ele vale até %s.\n\n"+
		"Se não foi você que criou a conta, ignore este email e não use o token.\n",
		panelLink(panelURL, "/email_confirmation/new", token), token, validUntil(d.ExpiresAt))
	return Message{From: from, To: d.Email, Subject: "Confirme seu email no markupp", Body: body}
}

// ResetPasswordMessage monta o email que leva o token de troca de senha e, com
// panelURL, o link do painel para trocar.
func ResetPasswordMessage(d TokenDetails, token, from, panelURL string) Message {
	body := fmt.Sprintf("Alguém pediu a troca da senha da sua conta no markupp.\n\n"+
		"%sToken de troca de senha:\n%s\n\n"+
		"Use o token no cliente do markupp. Ele vale até %s.\n\n"+
		"Se não foi você, ignore este email: a senha só muda com o token.\n",
		panelLink(panelURL, "/password_reset/new", token), token, validUntil(d.ExpiresAt))
	return Message{From: from, To: d.Email, Subject: "Troca de senha no markupp", Body: body}
}

// panelLink devolve o parágrafo com o link da rota do painel que recebe o token, ou
// vazio sem panelURL, e o email fica só com o token.
func panelLink(panelURL, path, token string) string {
	if panelURL == "" {
		return ""
	}
	link := strings.TrimSuffix(panelURL, "/") + path + "?token=" + url.QueryEscape(token)
	return "Abra no painel do markupp:\n" + link + "\n\n"
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
