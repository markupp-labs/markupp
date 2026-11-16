package notify

import (
	"net/url"
	"strings"
)

// letter é o conteúdo de um email do markupp, que sai em texto puro e em HTML.
type letter struct {
	subject string
	lead    string
	route   tokenLink
	// usage diz como usar o token e até quando, para quando não há painel.
	usage string
	// expiry diz até quando o link vale, para quando há painel.
	expiry string
	// caution é o aviso para quem não fez o pedido, opcional.
	caution string
	// reason completa "Você recebeu este email porque" na assinatura.
	reason string
}

// tokenLink é a rota do painel que recebe o token, o que o link faz, o texto do
// botão no HTML e o rótulo do token quando não há painel. Com painel, o token só
// vai dentro do link, como nos emails de confirmação em geral.
type tokenLink struct{ path, action, button, label string }

var (
	inviteLink = tokenLink{"/invite_acceptance/new", "Abra o link para aceitar o convite:", "Aceitar convite", "Token do convite:"}
	verifyLink = tokenLink{"/email_confirmation/new", "Abra o link para confirmar seu email:", "Confirmar email", "Token de verificação:"}
	resetLink  = tokenLink{"/password_reset/new", "Abra o link para trocar sua senha:", "Trocar senha", "Token de troca de senha:"}
)

func (l letter) message(from, to, token, panelURL string) Message {
	return Message{From: from, To: to, Subject: l.subject, Body: l.text(token, panelURL), HTML: l.html(token, panelURL)}
}

func (l letter) text(token, panelURL string) string {
	body := l.lead + "\n\n" + tokenBlock(panelURL, l.route, token) + l.closing(panelURL) + "\n"
	if l.caution != "" {
		body += "\n" + l.caution + "\n"
	}
	// "-- " com o espaço é o separador de assinatura que os clientes reconhecem (RFC 3676).
	return body + "\n-- \n" + signature(panelURL) + "\n" + l.reasonLine() + "\n"
}

// closing é a frase depois do link ou do token.
func (l letter) closing(panelURL string) string {
	if panelURL == "" {
		return l.usage
	}
	return l.expiry
}

func (l letter) reasonLine() string {
	return "Você recebeu este email porque " + l.reason
}

// tokenBlock devolve o parágrafo do token: com panelURL, o link da rota do painel que
// leva o token; sem, o token com o rótulo.
func tokenBlock(panelURL string, l tokenLink, token string) string {
	if panelURL == "" {
		return l.label + "\n" + token + "\n\n"
	}
	return l.action + "\n" + tokenURL(panelURL, l, token) + "\n\n"
}

func tokenURL(panelURL string, l tokenLink, token string) string {
	return strings.TrimSuffix(panelURL, "/") + l.path + "?token=" + url.QueryEscape(token)
}

// signature identifica a instância pelo endereço do painel, ou só pelo nome sem ele.
func signature(panelURL string) string {
	if host := panelHost(panelURL); host != "" {
		return "markupp · " + host
	}
	return "markupp"
}

func panelHost(panelURL string) string {
	parsed, err := url.Parse(panelURL)
	if err != nil {
		return ""
	}
	return parsed.Host
}
