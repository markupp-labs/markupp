package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
)

const metadataPath = "/.well-known/oauth-authorization-server"

func metadataRequest(t *testing.T, metadata *api.OAuthMetadata, path string) *httptest.ResponseRecorder {
	t.Helper()
	router := api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, OAuthMetadata: metadata})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestOAuthMetadata_Configurado_Retorna200ComOsCampos(t *testing.T) {
	rec := metadataRequest(t, &api.OAuthMetadata{Issuer: "https://markupp.exemplo"}, metadataPath)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{
		"issuer": "https://markupp.exemplo",
		"authorization_endpoint": "https://markupp.exemplo/oauth/authorize",
		"token_endpoint": "https://markupp.exemplo/oauth/token",
		"registration_endpoint": "https://markupp.exemplo/oauth/register",
		"revocation_endpoint": "https://markupp.exemplo/oauth/revoke",
		"response_types_supported": ["code"],
		"grant_types_supported": ["authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:token-exchange"],
		"code_challenge_methods_supported": ["S256"],
		"token_endpoint_auth_methods_supported": ["none", "client_secret_basic"],
		"revocation_endpoint_auth_methods_supported": ["none"],
		"scopes_supported": ["notas:leitura", "notas:escrita", "administracao"],
		"client_id_metadata_document_supported": true
	}`, rec.Body.String())
}

func TestOAuthMetadata_Configurado_NaoMarcaObsoleta(t *testing.T) {
	rec := metadataRequest(t, &api.OAuthMetadata{Issuer: "https://markupp.exemplo"}, metadataPath)

	assert.Empty(t, rec.Header().Get("Deprecation"))
	assert.Empty(t, rec.Header().Get("Link"))
}

func TestOAuthMetadata_SemConfiguracao_Retorna404(t *testing.T) {
	rec := metadataRequest(t, nil, metadataPath)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOAuthMetadata_SobV1_Retorna404(t *testing.T) {
	rec := metadataRequest(t, &api.OAuthMetadata{Issuer: "https://markupp.exemplo"}, "/v1"+metadataPath)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
