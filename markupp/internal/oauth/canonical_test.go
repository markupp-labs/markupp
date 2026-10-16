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
