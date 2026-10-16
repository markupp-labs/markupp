package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
)

const obsoletaDesde = "@1792108800"

func versionedRequest(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	router := api.NewRouter(api.RouterDeps{Notes: &fakeService{}, Tenants: tenantDeTeste, Auth: &fakeAuthService{}, Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}})
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestV1_Notas_RespondeSemObsolescencia(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/v1/notes")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Deprecation"))
	assert.Empty(t, rec.Header().Get("Link"))
}

func TestV1_Healthz_Responde200(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/v1/healthz")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Deprecation"))
}

func TestV1_AuthMe_AlcancaOHandler(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/v1/auth/me")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSemPrefixo_Notas_MarcaObsoletaEApontaSucessora(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/notes")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, obsoletaDesde, rec.Header().Get("Deprecation"))
	assert.Equal(t, `</v1/notes>; rel="successor-version"`, rec.Header().Get("Link"))
}

func TestSemPrefixo_Healthz_MarcaObsoletaEApontaSucessora(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/healthz")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, obsoletaDesde, rec.Header().Get("Deprecation"))
	assert.Equal(t, `</v1/healthz>; rel="successor-version"`, rec.Header().Get("Link"))
}

func TestSemPrefixo_RotaComParametro_SucessoraMantemOCaminho(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/notes/nota-1")

	assert.Equal(t, `</v1/notes/nota-1>; rel="successor-version"`, rec.Header().Get("Link"))
}
