package api_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func consentRequest(svc *fakeOAuthService, method, path string, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authenticated {
		req.Header.Set("Authorization", "Bearer token-valido")
	}
	return serveOAuth(svc, req)
}

func TestOAuthRequests_Descrever_DevolveOPedidoDaConta(t *testing.T) {
	svc := &fakeOAuthService{view: oauth.RequestView{
		ID: "p1", ClientID: "c1", ClientName: "Agente", RedirectHost: "cli.exemplo",
		Scopes: []string{"notas:leitura"}, AlreadyAuthorized: true, ExpiresAt: "2026-10-16T12:10:00Z",
	}}

	rec := consentRequest(svc, http.MethodGet, "/v1/oauth/requests/p1", true)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"p1","client_id":"c1","client_name":"Agente","redirect_host":"cli.exemplo",
		"scopes":["notas:leitura"],"already_authorized":true,"expires_at":"2026-10-16T12:10:00Z"}`, rec.Body.String())
	assert.Equal(t, "p1", svc.gotRequestID)
	assert.Equal(t, "u1", svc.gotUserID)
}

func TestOAuthRequests_DescreverSemEscopos_DevolveListaVazia(t *testing.T) {
	rec := consentRequest(&fakeOAuthService{view: oauth.RequestView{ID: "p1"}}, http.MethodGet, "/v1/oauth/requests/p1", true)

	assert.Contains(t, rec.Body.String(), `"scopes":[]`)
}

func TestOAuthRequests_Decidir_DevolveORedirect(t *testing.T) {
	for _, decision := range []string{"approve", "deny"} {
		svc := &fakeOAuthService{redirectTo: "https://cli.exemplo/cb?code=x"}

		rec := consentRequest(svc, http.MethodPost, "/v1/oauth/requests/p1/"+decision, true)

		require.Equal(t, http.StatusOK, rec.Code, decision)
		assert.JSONEq(t, `{"redirect_to":"https://cli.exemplo/cb?code=x"}`, rec.Body.String(), decision)
		assert.Equal(t, decision, svc.decision)
		assert.Equal(t, "p1", svc.gotRequestID, decision)
		assert.Equal(t, "u1", svc.gotUserID, decision)
	}
}

func TestOAuthRequests_PedidoInexistente_Retorna404(t *testing.T) {
	wrapped := fmt.Errorf("decidir: %w", oauth.ErrRequestNotFound)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/v1/oauth/requests/p1"},
		{http.MethodPost, "/v1/oauth/requests/p1/approve"},
		{http.MethodPost, "/v1/oauth/requests/p1/deny"},
	} {
		rec := consentRequest(&fakeOAuthService{err: wrapped}, route.method, route.path, true)

		assert.Equal(t, http.StatusNotFound, rec.Code, route.path)
		assert.Contains(t, rec.Body.String(), `"error":"request_not_found"`, route.path)
	}
}

func TestOAuthRequests_ErroInterno_Retorna500(t *testing.T) {
	rec := consentRequest(&fakeOAuthService{err: errors.New("banco fora")}, http.MethodPost, "/v1/oauth/requests/p1/approve", true)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOAuthRequests_SemToken_Retorna401(t *testing.T) {
	svc := &fakeOAuthService{}

	rec := consentRequest(svc, http.MethodPost, "/v1/oauth/requests/p1/approve", false)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, svc.decision)
}

func TestOAuthRequests_SemTenant_Atende(t *testing.T) {
	rec := consentRequest(&fakeOAuthService{view: oauth.RequestView{ID: "p1"}}, http.MethodGet, "/v1/oauth/requests/p1", true)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOAuthRequests_SemPrefixo_Retorna404SemObsolescencia(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/oauth/requests/p1"},
		{http.MethodPost, "/oauth/requests/p1/approve"},
		{http.MethodPost, "/oauth/requests/p1/deny"},
	} {
		svc := &fakeOAuthService{view: oauth.RequestView{ID: "p1"}}

		rec := consentRequest(svc, route.method, route.path, true)

		assert.Equal(t, http.StatusNotFound, rec.Code, route.path)
		assert.Empty(t, rec.Header().Get("Deprecation"), route.path)
		assert.Empty(t, svc.gotRequestID, route.path)
	}
}
