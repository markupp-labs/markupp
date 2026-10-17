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
		"scopes_supported": ["notas:leitura", "notas:escrita", "administracao"]
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

func TestOAuthMetadata_OrigemForaDaLista_RecebeCORSAberto(t *testing.T) {
	router := api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, AllowedOrigins: []string{origemPermitida}, OAuthMetadata: &api.OAuthMetadata{Issuer: "https://markupp.exemplo"}})
	req := httptest.NewRequest(http.MethodGet, metadataPath, nil)
	req.Header.Set("Origin", "https://cliente.exemplo")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestOAuthMetadata_OrigemDaLista_RecebeCORSAbertoSemEco(t *testing.T) {
	router := api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, AllowedOrigins: []string{origemPermitida}, OAuthMetadata: &api.OAuthMetadata{Issuer: "https://markupp.exemplo"}})
	req := httptest.NewRequest(http.MethodGet, metadataPath, nil)
	req.Header.Set("Origin", origemPermitida)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

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
