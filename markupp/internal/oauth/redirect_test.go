package oauth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func TestMatchRedirectURI_Compativel_Aceita(t *testing.T) {
	casos := map[string]struct{ registrada, pedida string }{
		"igual":                         {"https://app.exemplo.dev/callback", "https://app.exemplo.dev/callback"},
		"igual com query":               {"https://app.exemplo.dev/cb?x=1", "https://app.exemplo.dev/cb?x=1"},
		"localhost igual":               {"http://localhost:3000/cb", "http://localhost:3000/cb"},
		"loopback ipv4 com outra porta": {"http://127.0.0.1/cb", "http://127.0.0.1:51234/cb"},
		"loopback ipv4 troca de porta":  {"http://127.0.0.1:8080/cb", "http://127.0.0.1:9090/cb"},
		"loopback ipv6 com outra porta": {"http://[::1]/cb", "http://[::1]:51234/cb"},
		"loopback com query igual":      {"http://127.0.0.1/cb?x=1", "http://127.0.0.1:4000/cb?x=1"},
		"localhost com outra porta":     {"http://localhost:3000/cb", "http://localhost:4000/cb"},
		"localhost do claude code":      {"http://localhost/callback", "http://localhost:53682/callback"},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			assert.True(t, oauth.MatchRedirectURI([]string{"https://outro.dev/cb", caso.registrada}, caso.pedida))
		})
	}
}

func TestMatchRedirectURI_Diferente_Recusa(t *testing.T) {
	casos := map[string]struct{ registrada, pedida string }{
		"caminho a mais":                {"https://app.exemplo.dev/cb", "https://app.exemplo.dev/cb/"},
		"query a mais":                  {"https://app.exemplo.dev/cb", "https://app.exemplo.dev/cb?x=1"},
		"maiusculas":                    {"https://app.exemplo.dev/cb", "https://APP.exemplo.dev/cb"},
		"https com outra porta":         {"https://app.exemplo.dev/cb", "https://app.exemplo.dev:8443/cb"},
		"fragmento":                     {"https://app.exemplo.dev/cb", "https://app.exemplo.dev/cb#x"},
		"fragmento vazio":               {"https://app.exemplo.dev/cb", "https://app.exemplo.dev/cb#"},
		"fragmento registrado igual":    {"https://app.exemplo.dev/cb#x", "https://app.exemplo.dev/cb#x"},
		"loopback com outro caminho":    {"http://127.0.0.1/cb", "http://127.0.0.1:4000/outro"},
		"loopback com outra query":      {"http://127.0.0.1/cb?x=1", "http://127.0.0.1:4000/cb?x=2"},
		"loopback com fragmento":        {"http://127.0.0.1/cb", "http://127.0.0.1:4000/cb#x"},
		"loopback ipv4 para ipv6":       {"http://127.0.0.1/cb", "http://[::1]:4000/cb"},
		"loopback para localhost":       {"http://127.0.0.1/cb", "http://localhost:4000/cb"},
		"localhost para loopback":       {"http://localhost/cb", "http://127.0.0.1:4000/cb"},
		"localhost em caixa alta":       {"http://localhost/cb", "http://LOCALHOST:4000/cb"},
		"loopback com https":            {"https://127.0.0.1/cb", "https://127.0.0.1:4000/cb"},
		"loopback de http para https":   {"http://127.0.0.1/cb", "https://127.0.0.1:4000/cb"},
		"loopback com esquema em caixa": {"http://127.0.0.1/cb", "HTTP://127.0.0.1:4000/cb"},
		"loopback com usuario":          {"http://127.0.0.1/cb", "http://intruso@127.0.0.1:4000/cb"},
		"outro ip de loopback":          {"http://127.0.0.2/cb", "http://127.0.0.2:4000/cb"},
		"pedida vazia":                  {"https://app.exemplo.dev/cb", ""},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			assert.False(t, oauth.MatchRedirectURI([]string{caso.registrada}, caso.pedida))
		})
	}
}

func TestMatchRedirectURI_SemRegistradas_Recusa(t *testing.T) {
	assert.False(t, oauth.MatchRedirectURI(nil, "https://app.exemplo.dev/cb"))
}
