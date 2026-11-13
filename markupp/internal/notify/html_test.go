package notify_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notify"
)

var todasAsMensagens = map[string]montaMensagem{
	"convite":     notify.InviteMessage,
	"verificacao": notify.VerifyEmailMessage,
	"senha":       notify.ResetPasswordMessage,
}

func TestMensagens_ComPainel_AssinamComOEnderecoDoPainel(t *testing.T) {
	for nome, monta := range todasAsMensagens {
		msg := monta(detalhesDeTeste(), "tok", "contas@markupp.dev", "https://painel.markupp.dev/")

		assert.Contains(t, msg.Body, "\n-- \nmarkupp · painel.markupp.dev\nVocê recebeu este email porque ", nome)
		assert.Contains(t, msg.HTML, `href="https://painel.markupp.dev/"`, nome)
		assert.Contains(t, msg.HTML, ">painel.markupp.dev</a>", nome)
	}
}

func TestMensagens_SemPainel_AssinamSoComONomeESemLink(t *testing.T) {
	for nome, monta := range todasAsMensagens {
		msg := monta(detalhesDeTeste(), "tok", "contas@markupp.dev", "")

		assert.Contains(t, msg.Body, "\n-- \nmarkupp\nVocê recebeu este email porque ", nome)
		assert.NotContains(t, msg.HTML, "href", nome)
	}
}

func TestMensagens_HTML_DizemPorQueAPessoaRecebeu(t *testing.T) {
	motivos := map[string]string{
		"convite":     "Você recebeu este email porque este endereço foi convidado para um tenant.",
		"verificacao": "Você recebeu este email porque uma conta do markupp foi criada com este endereço.",
		"senha":       "Você recebeu este email porque pediram a troca da senha da conta deste endereço.",
	}
	for nome, monta := range todasAsMensagens {
		msg := monta(detalhesDeTeste(), "tok", "contas@markupp.dev", "")

		assert.Contains(t, msg.Body, motivos[nome], nome)
		assert.Contains(t, msg.HTML, motivos[nome], nome)
	}
}

func TestMensagens_ComPainel_HTMLTemBotaoEOLinkPorExtensoSemOToken(t *testing.T) {
	casos := map[string]struct {
		monta   montaMensagem
		botao   string
		caminho string
	}{
		"convite":     {notify.InviteMessage, "Aceitar convite", "/invite_acceptance/new"},
		"verificacao": {notify.VerifyEmailMessage, "Confirmar email", "/email_confirmation/new"},
		"senha":       {notify.ResetPasswordMessage, "Trocar senha", "/password_reset/new"},
	}
	for nome, c := range casos {
		msg := c.monta(detalhesDeTeste(), "a+b/c=", "contas@markupp.dev", "https://painel.markupp.dev")

		link := "https://painel.markupp.dev" + c.caminho + "?token=a%2Bb%2Fc%3D"
		assert.Contains(t, msg.HTML, `href="`+link+`"`, nome)
		assert.Contains(t, msg.HTML, ">"+c.botao+"</a>", nome)
		assert.Contains(t, msg.HTML, ">"+link+"</a>", "o link por extenso para quando o botao nao funciona: %s", nome)
		assert.NotContains(t, msg.HTML, ">a&#43;b/c=<", "com painel, o token so vai dentro do link: %s", nome)
	}
}

func TestMensagens_SemPainel_HTMLMostraOTokenComORotulo(t *testing.T) {
	msg := notify.InviteMessage(detalhesDeTeste(), "tok-123", "convites@markupp.dev", "")

	assert.Contains(t, msg.HTML, "Token do convite:")
	assert.Contains(t, msg.HTML, ">tok-123<")
	assert.Contains(t, msg.HTML, "2026-10-22 12:00 UTC")
}

func TestInviteMessage_HTMLEscapaOQueQuemConvidaEscolhe(t *testing.T) {
	detalhes := detalhesDeTeste()
	detalhes.TenantName = `<a href="https://phishing.example">Equipe</a>`

	msg := notify.InviteMessage(detalhes, "tok", "convites@markupp.dev", "")

	assert.NotContains(t, msg.HTML, "phishing.example\"", "o nome do tenant e texto, nunca marcacao")
	assert.Contains(t, msg.HTML, "&lt;a href=")
}

func TestMensagens_HTMLTemONomeEOTemaEscuroDoPainel(t *testing.T) {
	for nome, monta := range todasAsMensagens {
		msg := monta(detalhesDeTeste(), "tok", "contas@markupp.dev", "")

		assert.Contains(t, msg.HTML, `<html lang="pt-BR">`, nome)
		assert.Contains(t, msg.HTML, ">markupp</td>", nome)
		assert.Contains(t, msg.HTML, "prefers-color-scheme: dark", nome)
		assert.Contains(t, msg.HTML, "#1f1f28", "o fundo do wave no escuro: %s", nome)
	}
}
