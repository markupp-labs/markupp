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
	rec := metadataRequest(t, &api.OAuthMetadata{Issuer: "https://markupp.exemplo", DynamicRegistration: true}, metadataPath)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{
		"issuer": "https://markupp.exemplo",
		"authorization_endpoint": "https://markupp.exemplo/oauth/authorize",
		"token_endpoint": "https://markupp.exemplo/oauth/token",
		"registration_endpoint": "https://markupp.exemplo/oauth/register",
		"revocation_endpoint": "https://markupp.exemplo/oauth/revoke",
		"scopes_supported": ["notas:leitura", "notas:escrita", "administracao"],
		"response_types_supported": ["code"],
		"grant_types_supported": ["authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:token-exchange"],
		"code_challenge_methods_supported": ["S256"],
		"token_endpoint_auth_methods_supported": ["none"],
		"revocation_endpoint_auth_methods_supported": ["none"],
		"client_id_metadata_document_supported": true
	}`, rec.Body.String())
}

func TestOAuthMetadata_SemRegistroDinamico_NaoAnunciaRegistrationEndpoint(t *testing.T) {
	rec := metadataRequest(t, &api.OAuthMetadata{Issuer: "https://markupp.exemplo"}, metadataPath)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "registration_endpoint")
	assert.Contains(t, rec.Body.String(), `"token_endpoint":"https://markupp.exemplo/oauth/token"`)
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

func TestOAuthMetadata_OrigemForaDaLista_RecebeCORSAberto(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, metadataPath, nil)
	req.Header.Set("Origin", "https://cliente.exemplo")
	rec := httptest.NewRecorder()
	oauthRouterComOrigens(&fakeOAuthService{}, []string{origemPermitida}).ServeHTTP(rec, req)

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestOAuthMetadata_OrigemDaLista_RecebeCORSAbertoSemEco(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, metadataPath, nil)
	req.Header.Set("Origin", origemPermitida)
	rec := httptest.NewRecorder()
	oauthRouterComOrigens(&fakeOAuthService{}, []string{origemPermitida}).ServeHTTP(rec, req)

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Vary"))
}

func TestOAuthMetadata_RotaDeNotasComMesmaOrigem_SegueSemCORS(t *testing.T) {
	router := api.NewRouter(api.RouterDeps{Notes: &fakeService{listResult: nil}, Tenants: tenantDeTeste, Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, AllowedOrigins: []string{origemPermitida}, OAuthMetadata: &api.OAuthMetadata{Issuer: "https://markupp.exemplo"}})
	req := httptest.NewRequest(http.MethodGet, "/v1/notes", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	req.Header.Set("Origin", "https://cliente.exemplo")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestNewRouter_IssuerInvalido_EntraEmPanic(t *testing.T) {
	for _, issuer := range []string{"", "markupp.exemplo", "/caminho", "https://", "://sem-esquema"} {
		assert.Panics(t, func() {
			api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, OAuthMetadata: &api.OAuthMetadata{Issuer: issuer}})
		}, "issuer %q", issuer)
	}
}

func TestOAuthMetadata_Preflight_Retorna204ComCORSAbertoEMCPProtocolVersion(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, metadataPath, nil)
	req.Header.Set("Origin", "https://cliente.exemplo")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "mcp-protocol-version")

	rec := serveOAuth(&fakeOAuthService{}, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, MCP-Protocol-Version", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}
