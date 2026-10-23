package cimd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

// redirectURIsPermitidas valem tanto no documento CIMD quanto no registro dinâmico.
var redirectURIsPermitidas = []string{
	"https://agente.example.com/cb",
	"https://agente.example.com:8443/cb?estado=1",
	"http://127.0.0.1/callback",
	"http://127.0.0.1:53682/callback",
	"http://[::1]:8080/callback",
	"http://localhost/callback",
	"http://localhost:8080/cb",
	"com.exemplo.app:/oauth2redirect",
	"com.exemplo.app://callback",
	"meuapp://callback",
	"cursor://anysphere.cursor-retrieval/oauth/callback",
	"vscode://vscode.github-authentication/did-authenticate",
}

// redirectURIsProibidas são recusadas tanto no documento CIMD quanto no registro dinâmico.
var redirectURIsProibidas = map[string]string{
	"javascript":            "javascript:alert(1)",
	"data":                  "data:text/html,oi",
	"vbscript":              "vbscript:msgbox(1)",
	"file":                  "file:///etc/passwd",
	"javascript maiusculo":  "JavaScript:alert(1)",
	"file maiusculo":        "FILE:///etc/passwd",
	"https opaca":           "https:agente.example.com/cb",
	"https sem host":        "https:///cb",
	"http opaca":            "http:127.0.0.1/cb",
	"http fora do loopback": "http://agente.example.com/cb",
	"http rede privada":     "http://192.168.0.10/cb",
	"fragmento https":       "https://agente.example.com/cb#x",
	"fragmento vazio":       "https://agente.example.com/cb#",
	"fragmento privado":     "com.exemplo.app:/cb#x",
	"userinfo":              "https://usuario@agente.example.com/cb",
	"relativa":              "/callback",
	"vazia":                 "",
}

func TestRedirectURIProblem_Permitida_DevolveVazio(t *testing.T) {
	for _, uri := range redirectURIsPermitidas {
		assert.Empty(t, cimd.RedirectURIProblem(uri), uri)
	}
}

func TestRedirectURIProblem_Proibida_DevolveMotivo(t *testing.T) {
	for nome, uri := range redirectURIsProibidas {
		assert.NotEmpty(t, cimd.RedirectURIProblem(uri), nome)
	}
}

func TestFetcher_RedirectURIPermitida_Aceita(t *testing.T) {
	for _, uri := range redirectURIsPermitidas {
		t.Run(uri, func(t *testing.T) {
			srv := novoServidorDeMetadados(t)
			srv.documento.RedirectURIs = new([]string{uri})

			meta, err := srv.buscar()

			require.NoError(t, err)
			assert.Equal(t, []string{uri}, meta.RedirectURIs)
		})
	}
}

func TestFetcher_RedirectURIProibida_RecusaComValor(t *testing.T) {
	for nome, uri := range redirectURIsProibidas {
		t.Run(nome, func(t *testing.T) {
			srv := novoServidorDeMetadados(t)
			srv.documento.RedirectURIs = new([]string{"https://agente.example.com/ok", uri})

			_, err := srv.buscar()

			require.ErrorIs(t, err, cimd.ErrInvalidDocument)
			assert.Contains(t, err.Error(), "esperado")
		})
	}
}
