package oauth_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func TestCanonicalURL_HostMaiusculoEBarraFinal_Normaliza(t *testing.T) {
	parsed, err := url.Parse("HTTPS://MCP.Markupp.dev:8443/Rota/")
	require.NoError(t, err)

	assert.Equal(t, "https://mcp.markupp.dev:8443/Rota", oauth.CanonicalURL(parsed))
}

func TestCanonicalURL_SoBarra_FicaSemCaminho(t *testing.T) {
	parsed, err := url.Parse("https://mcp.markupp.dev/")
	require.NoError(t, err)

	assert.Equal(t, "https://mcp.markupp.dev", oauth.CanonicalURL(parsed))
}

func TestCanonicalURL_PortaPadraoDoEsquema_RemoveAPorta(t *testing.T) {
	for raw, esperado := range map[string]string{
		"https://mcp.markupp.dev:443/mcp": "https://mcp.markupp.dev/mcp",
		"HTTP://Localhost:80/painel":      "http://localhost/painel",
		"http://[::1]:80/mcp":             "http://[::1]/mcp",
	} {
		parsed, err := url.Parse(raw)
		require.NoError(t, err)

		assert.Equal(t, esperado, oauth.CanonicalURL(parsed), raw)
	}
}

func TestCanonicalURL_PortaPadraoDeOutroEsquema_MantemAPorta(t *testing.T) {
	for _, raw := range []string{"https://mcp.markupp.dev:80/mcp", "http://localhost:443/painel"} {
		parsed, err := url.Parse(raw)
		require.NoError(t, err)

		assert.Equal(t, raw, oauth.CanonicalURL(parsed))
	}
}
