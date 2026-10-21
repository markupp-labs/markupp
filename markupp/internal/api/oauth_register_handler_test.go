package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const corpoDeRegistro = `{"client_name":"Agente","redirect_uris":["http://127.0.0.1/cb"],` +
	`"token_endpoint_auth_method":"none","grant_types":["authorization_code"],"response_types":["code"],"logo_uri":"ignorado"}`

func registerRequest(svc *fakeOAuthService, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", "https://cliente.exemplo")
	return serveOAuth(svc, req)
}

func registroEmitido() oauth.RegistrationResponse {
	return oauth.RegistrationResponse{
		ClientID: "markupp-dcr-x", ClientIDIssuedAt: 1792000000, ClientName: "Agente",
		RedirectURIs: []string{"http://127.0.0.1/cb"}, TokenEndpointAuthMethod: "none",
		GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"},
	}
}

func TestRegister_Valido_Retorna201SemCacheComCORSAberto(t *testing.T) {
	svc := &fakeOAuthService{registration: registroEmitido()}

	rec := registerRequest(svc, "application/json; charset=utf-8", corpoDeRegistro)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.JSONEq(t, `{"client_id":"markupp-dcr-x","client_id_issued_at":1792000000,"client_name":"Agente",
		"redirect_uris":["http://127.0.0.1/cb"],"token_endpoint_auth_method":"none",
		"grant_types":["authorization_code","refresh_token"],"response_types":["code"]}`, rec.Body.String())
}

func TestRegister_LeOsMetadadosDoCorpo(t *testing.T) {
	svc := &fakeOAuthService{registration: registroEmitido()}

	registerRequest(svc, "application/json", corpoDeRegistro)

	require.NotNil(t, svc.gotRegistration)
	assert.Equal(t, oauth.RegistrationRequest{
		ClientName: "Agente", RedirectURIs: []string{"http://127.0.0.1/cb"}, TokenEndpointAuthMethod: new("none"),
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"},
	}, *svc.gotRegistration)
}

func TestRegister_CamposAusentes_ChegamNil(t *testing.T) {
	svc := &fakeOAuthService{registration: registroEmitido()}

	registerRequest(svc, "application/json", `{"client_name":"Agente"}`)

	require.NotNil(t, svc.gotRegistration)
	assert.Nil(t, svc.gotRegistration.TokenEndpointAuthMethod)
	assert.Nil(t, svc.gotRegistration.GrantTypes)
	assert.Nil(t, svc.gotRegistration.ResponseTypes)
	assert.Nil(t, svc.gotRegistration.RedirectURIs)
}

func TestRegister_CorpoRecusado_Retorna400SemChamarOServico(t *testing.T) {
	casos := map[string]struct{ contentType, body string }{
		"content-type de formulario": {"application/x-www-form-urlencoded", "client_name=Agente"},
		"sem content-type":           {"", corpoDeRegistro},
		"json malformado":            {"application/json", `{"client_name":`},
		"json que nao e objeto":      {"application/json", `["Agente"]`},
		"tipo errado":                {"application/json", `{"client_name":42}`},
		"dados depois do objeto":     {"application/json", corpoDeRegistro + `{}`},
		"corpo vazio":                {"application/json", ""},
		"acima do limite":            {"application/json", `{"client_name":"` + strings.Repeat("a", 64<<10) + `"}`},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			svc := &fakeOAuthService{}

			rec := registerRequest(svc, caso.contentType, caso.body)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error":"invalid_client_metadata"`)
			assert.Contains(t, rec.Body.String(), "esperad")
			assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
			assert.Nil(t, svc.gotRegistration)
		})
	}
}

func TestRegister_RecusaDoServico_Retorna400ComOErroOAuth(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{Code: "invalid_redirect_uri", Description: "redirect_uri 'javascript:x' proibida; esperado https"}}

	rec := registerRequest(svc, "application/json", corpoDeRegistro)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid_redirect_uri","error_description":"redirect_uri 'javascript:x' proibida; esperado https"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestRegister_TetoDePendentes_Retorna429ComRetryAfterEmSegundos(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{
		Code: "temporarily_unavailable", Description: "lotado; esperada nova tentativa", RetryAfter: 90*time.Second + time.Millisecond,
	}}

	rec := registerRequest(svc, "application/json", corpoDeRegistro)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "91", rec.Header().Get("Retry-After"))
	assert.JSONEq(t, `{"error":"temporarily_unavailable","error_description":"lotado; esperada nova tentativa"}`, rec.Body.String())
}

func TestRegister_ErroInterno_Retorna500(t *testing.T) {
	rec := registerRequest(&fakeOAuthService{err: errors.New("banco fora")}, "application/json", corpoDeRegistro)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "banco fora")
}

func TestRegister_Preflight_Retorna204ComCORSAberto(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/oauth/register", nil)
	req.Header.Set("Origin", "https://cliente.exemplo")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	rec := serveOAuth(&fakeOAuthService{}, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
}

func TestRegister_GET_Retorna405(t *testing.T) {
	rec := serveOAuth(&fakeOAuthService{}, httptest.NewRequest(http.MethodGet, "/oauth/register", nil))

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestRegister_RegistroDinamicoDesligado_Retorna404(t *testing.T) {
	svc := &fakeOAuthService{registration: registroEmitido()}
	router := api.NewRouter(api.RouterDeps{
		Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, OAuth: svc, OAuthMetadata: &api.OAuthMetadata{Issuer: issuerDeTeste},
	})
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(corpoDeRegistro))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Nil(t, svc.gotRegistration)
}
