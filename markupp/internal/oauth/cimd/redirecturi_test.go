package cimd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

func TestFetcher_RedirectURIPermitida_Aceita(t *testing.T) {
	permitidas := []string{
		"https://agente.example.com/cb",
		"https://agente.example.com:8443/cb?estado=1",
		"http://127.0.0.1/callback",
		"http://127.0.0.1:53682/callback",
		"http://[::1]:8080/callback",
		"com.exemplo.app:/oauth2redirect",
		"com.exemplo.app://callback",
	}
	for _, uri := range permitidas {
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
	proibidas := map[string]string{
		"javascript":            "javascript:alert(1)",
		"data":                  "data:text/html,oi",
		"vbscript":              "vbscript:msgbox(1)",
		"file":                  "file:///etc/passwd",
		"esquema sem ponto":     "meuapp://callback",
		"https opaca":           "https:agente.example.com/cb",
		"https sem host":        "https:///cb",
		"http opaca":            "http:127.0.0.1/cb",
		"http fora do loopback": "http://agente.example.com/cb",
		"http localhost":        "http://localhost:8080/cb",
		"http rede privada":     "http://192.168.0.10/cb",
		"fragmento https":       "https://agente.example.com/cb#x",
		"fragmento vazio":       "https://agente.example.com/cb#",
		"fragmento privado":     "com.exemplo.app:/cb#x",
		"userinfo":              "https://usuario@agente.example.com/cb",
		"relativa":              "/callback",
		"vazia":                 "",
	}
	for nome, uri := range proibidas {
		t.Run(nome, func(t *testing.T) {
			srv := novoServidorDeMetadados(t)
			srv.documento.RedirectURIs = new([]string{"https://agente.example.com/ok", uri})

			_, err := srv.buscar()

			require.ErrorIs(t, err, cimd.ErrInvalidDocument)
			assert.Contains(t, err.Error(), "esperado")
		})
	}
}
