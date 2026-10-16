package cimd_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth/cimd"
)

func TestParseClientID_URLsValidas_Aceita(t *testing.T) {
	validas := []string{
		"https://app.example.com/client.json",
		"https://app.example.com:8443/oauth/client",
		"https://app.example.com/client?versao=2",
	}
	for _, raw := range validas {
		t.Run(raw, func(t *testing.T) {
			u, err := cimd.ParseClientID(raw)

			require.NoError(t, err)
			assert.Equal(t, raw, u.String())
		})
	}
}

func TestParseClientID_URLsInvalidas_RecusaComValorEFormato(t *testing.T) {
	invalidas := map[string]string{
		"http":                   "http://app.example.com/client.json",
		"sem esquema":            "app.example.com/client.json",
		"sem caminho":            "https://app.example.com",
		"so barra":               "https://app.example.com/",
		"fragmento":              "https://app.example.com/client.json#x",
		"fragmento vazio":        "https://app.example.com/client.json#",
		"credenciais":            "https://user:senha@app.example.com/client.json",
		"ponto":                  "https://app.example.com/./client.json",
		"ponto ponto":            "https://app.example.com/a/../client.json",
		"ponto ponto codificado": "https://app.example.com/a/%2e%2e/client.json",
		"opaca":                  "https:app.example.com/client.json",
		"vazia":                  "",
	}
	for nome, raw := range invalidas {
		t.Run(nome, func(t *testing.T) {
			_, err := cimd.ParseClientID(raw)

			require.ErrorIs(t, err, cimd.ErrInvalidClientID)
			assert.Contains(t, err.Error(), "esperado")
		})
	}
}
