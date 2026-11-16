package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
)

func versionedRequest(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	return versionedRequestWithBody(t, method, path, "")
}

func versionedRequestWithBody(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := api.NewRouter(api.RouterDeps{Notes: &fakeService{}, Tenants: tenantDeTeste, Auth: &fakeAuthService{}, Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}})
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestV1_Notas_RespondeSemObsolescencia(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/v1/notes")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestV1_Healthz_Responde200(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/v1/healthz")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestV1_AuthMe_AlcancaOHandler(t *testing.T) {
	rec := versionedRequest(t, http.MethodGet, "/v1/auth/me")

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestV1_PostLogin_AlcancaOHandler(t *testing.T) {
	rec := versionedRequestWithBody(t, http.MethodPost, "/v1/auth/login", `{"email":"a@b.c","password":"senha"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestV1_PostNotas_CriaSemObsolescencia(t *testing.T) {
	rec := versionedRequestWithBody(t, http.MethodPost, "/v1/notes", `{"path":"x.md","content":"y"}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestSemPrefixo_Responde404(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/notes"}, {http.MethodGet, "/healthz"}, {http.MethodGet, "/auth/me"}, {http.MethodPost, "/auth/login"},
	} {
		rec := versionedRequestWithBody(t, route.method, route.path, `{"email":"a@b.c","password":"senha"}`)

		assert.Equal(t, http.StatusNotFound, rec.Code, route.path)
	}
}

func TestV1_MetodoSemRota_Responde405(t *testing.T) {
	rec := versionedRequest(t, http.MethodDelete, "/v1/healthz")

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
