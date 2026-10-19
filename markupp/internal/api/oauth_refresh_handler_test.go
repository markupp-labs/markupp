package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

func refreshForm() url.Values {
	return url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {"rt"}, "client_id": {"c1"},
		"scope": {"notas:leitura"}, "resource": {"https://mcp.exemplo"},
	}
}

func TestTokenRefresh_LeOFormularioParaOGrant(t *testing.T) {
	svc := &fakeOAuthService{}

	tokenRequest(svc, refreshForm())

	assert.Equal(t, oauth.RefreshGrant{RefreshToken: "rt", ClientID: "c1", Scope: "notas:leitura", Resource: "https://mcp.exemplo"}, svc.gotRefresh)
	assert.Empty(t, svc.gotGrant.Code)
}

func TestTokenRefresh_Sucesso_Retorna200SemCache(t *testing.T) {
	svc := &fakeOAuthService{tokens: oauth.TokenResponse{AccessToken: "at2", TokenType: "Bearer", ExpiresIn: 3600, RefreshToken: "rt2", Scope: "notas:leitura"}}

	rec := tokenRequest(svc, refreshForm())

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"access_token":"at2","token_type":"Bearer","expires_in":3600,"refresh_token":"rt2","scope":"notas:leitura"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestTokenRefresh_ErroDoProtocolo_Retorna400(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{Code: "invalid_grant", Description: "refresh revogado"}}

	rec := tokenRequest(svc, refreshForm())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid_grant","error_description":"refresh revogado"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestTokenRefresh_ParametroRepetido_Retorna400SemChamarServico(t *testing.T) {
	form := refreshForm()
	form.Add("refresh_token", "outro")
	svc := &fakeOAuthService{}

	rec := tokenRequest(svc, form)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
	assert.Empty(t, svc.gotRefresh.RefreshToken)
}

func TestTokenRefresh_ErroInterno_Retorna500SemCache(t *testing.T) {
	rec := tokenRequest(&fakeOAuthService{err: errors.New("banco fora")}, refreshForm())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func revokeRequest(svc *fakeOAuthService, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return serveOAuth(svc, req)
}

func revokeForm() url.Values {
	return url.Values{"token": {"rt"}, "client_id": {"c1"}, "token_type_hint": {"refresh_token"}}
}

func TestRevoke_LeTokenEClienteDoFormulario(t *testing.T) {
	svc := &fakeOAuthService{}

	revokeRequest(svc, revokeForm())

	assert.Equal(t, oauth.RevokeRequest{Token: "rt", ClientID: "c1"}, svc.gotRevoke)
}

func TestRevoke_Sucesso_Retorna200SemCorpoESemCache(t *testing.T) {
	rec := revokeRequest(&fakeOAuthService{}, revokeForm())

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestRevoke_SemToken_Retorna400InvalidRequestSemChamarServico(t *testing.T) {
	svc := &fakeOAuthService{}

	rec := revokeRequest(svc, url.Values{"client_id": {"c1"}})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
	assert.Empty(t, svc.gotRevoke.ClientID)
}

func TestRevoke_ParametroRepetido_Retorna400InvalidRequest(t *testing.T) {
	form := revokeForm()
	form.Add("token", "outro")
	svc := &fakeOAuthService{}

	rec := revokeRequest(svc, form)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
	assert.Empty(t, svc.gotRevoke.Token)
}

func TestRevoke_CorpoNaoFormulario_Retorna400InvalidRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(`{"token":"rt"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := serveOAuth(&fakeOAuthService{}, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
}

func TestRevoke_ErroInterno_Retorna500SemCache(t *testing.T) {
	rec := revokeRequest(&fakeOAuthService{err: errors.New("banco fora")}, revokeForm())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestRevoke_Preflight_Retorna204ComCORSAberto(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/oauth/revoke", nil)
	req.Header.Set("Origin", "https://cliente.exemplo")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	rec := serveOAuth(&fakeOAuthService{}, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
}

func TestRevoke_MetodoNaoSuportado_Retorna405ComCORSAberto(t *testing.T) {
	rec := oauthRequestDeOutraOrigem(http.MethodGet, "/oauth/revoke")

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}
