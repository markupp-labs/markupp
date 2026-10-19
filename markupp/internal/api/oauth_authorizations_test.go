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

func TestMyAuthorizations_Listar_DevolveAsAutorizacoesDaConta(t *testing.T) {
	svc := &fakeOAuthService{authorizations: []oauth.AuthorizationView{{
		ID: "a1", ClientID: "c1", ClientName: "Agente", Scopes: []string{"notas:leitura"},
		CreatedAt: "2026-10-16T12:00:00Z", UpdatedAt: "2026-10-17T12:00:00Z",
	}}}

	rec := consentRequest(svc, http.MethodGet, "/v1/me/authorizations", true)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"id":"a1","client_id":"c1","client_name":"Agente","scopes":["notas:leitura"],
		"created_at":"2026-10-16T12:00:00Z","updated_at":"2026-10-17T12:00:00Z"}]`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "u1", svc.gotUserID)
}

func TestMyAuthorizations_ListarVazio_DevolveListaVazia(t *testing.T) {
	rec := consentRequest(&fakeOAuthService{}, http.MethodGet, "/v1/me/authorizations", true)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestMyAuthorizations_ListarSemEscopos_DevolveEscoposVazios(t *testing.T) {
	svc := &fakeOAuthService{authorizations: []oauth.AuthorizationView{{ID: "a1"}}}

	rec := consentRequest(svc, http.MethodGet, "/v1/me/authorizations", true)

	assert.Contains(t, rec.Body.String(), `"scopes":[]`)
}

func TestMyAuthorizations_Revogar_Retorna204SemCache(t *testing.T) {
	svc := &fakeOAuthService{}

	rec := consentRequest(svc, http.MethodDelete, "/v1/me/authorizations/a1", true)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "u1", svc.gotUserID)
	assert.Equal(t, "a1", svc.gotAuthorizationID)
}

func TestMyAuthorizations_RevogarInexistente_Retorna404(t *testing.T) {
	svc := &fakeOAuthService{err: fmt.Errorf("revogar: %w", oauth.ErrAuthorizationNotFound)}

	rec := consentRequest(svc, http.MethodDelete, "/v1/me/authorizations/a1", true)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"authorization_not_found"`)
}

func TestMyAuthorizations_ErroInterno_Retorna500(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/v1/me/authorizations"},
		{http.MethodDelete, "/v1/me/authorizations/a1"},
	} {
		rec := consentRequest(&fakeOAuthService{err: errors.New("banco fora")}, route.method, route.path, true)

		assert.Equal(t, http.StatusInternalServerError, rec.Code, route.path)
		assert.JSONEq(t, `{"error":"internal","message":"erro interno"}`, rec.Body.String(), route.path)
	}
}

func TestMyAuthorizations_SemToken_Retorna401(t *testing.T) {
	svc := &fakeOAuthService{}

	rec := consentRequest(svc, http.MethodDelete, "/v1/me/authorizations/a1", false)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, svc.gotAuthorizationID)
}

func TestMyAuthorizations_SemPrefixo_Retorna404(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/me/authorizations"},
		{http.MethodDelete, "/me/authorizations/a1"},
	} {
		svc := &fakeOAuthService{}

		rec := consentRequest(svc, route.method, route.path, true)

		assert.Equal(t, http.StatusNotFound, rec.Code, route.path)
		assert.Empty(t, svc.gotUserID, route.path)
	}
}

func TestMyAuthorizations_OrigemDaLista_RecebeCORSDaAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/me/authorizations", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	req.Header.Set("Origin", origemPermitida)
	rec := httptest.NewRecorder()
	oauthRouterComOrigens(&fakeOAuthService{}, []string{origemPermitida}).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, origemPermitida, rec.Header().Get("Access-Control-Allow-Origin"))
}
