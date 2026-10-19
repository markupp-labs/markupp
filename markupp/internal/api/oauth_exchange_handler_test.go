package api_test

import (
	"encoding/base64"
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

const recursoDeTeste = "https://mcp.exemplo"

func exchangeForm() url.Values {
	return url.Values{
		"grant_type": {oauth.GrantTypeTokenExchange}, "subject_token": {"st"}, "subject_token_type": {oauth.TokenTypeAccessToken},
		"requested_token_type": {oauth.TokenTypeAccessToken}, "audience": {issuerDeTeste}, "resource": {issuerDeTeste},
	}
}

// exchangeRequest manda a troca com o Basic dado, já codificado pelo chamador; usuário
// vazio manda sem Basic.
func exchangeRequest(svc *fakeOAuthService, form url.Values, user, password string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if user != "" {
		req.SetBasicAuth(user, password)
	}
	return serveOAuth(svc, req)
}

func assertInvalidClient(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_client"`)
	assert.Equal(t, `Basic realm="markupp"`, rec.Header().Get("WWW-Authenticate"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
}

func TestTokenExchange_LeBasicEFormularioParaOGrant(t *testing.T) {
	svc := &fakeOAuthService{}

	segredoDecodificado := "s+creto x"

	exchangeRequest(svc, exchangeForm(), url.QueryEscape(recursoDeTeste), "s%2Bcreto+x")

	assert.Equal(t, oauth.ExchangeGrant{
		ResourceID: recursoDeTeste, ResourceSecret: segredoDecodificado, SubjectToken: "st", SubjectTokenType: oauth.TokenTypeAccessToken,
		RequestedTokenType: oauth.TokenTypeAccessToken, Audience: issuerDeTeste, Resource: issuerDeTeste,
	}, svc.gotExchange)
}

func TestTokenExchange_Sucesso_Retorna200SemCache(t *testing.T) {
	svc := &fakeOAuthService{exchange: oauth.ExchangeResponse{
		AccessToken: "at", IssuedTokenType: oauth.TokenTypeAccessToken, TokenType: "Bearer", ExpiresIn: 300, Scope: "notas:leitura",
	}}

	rec := exchangeRequest(svc, exchangeForm(), recursoDeTeste, "segredo")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"access_token":"at","issued_token_type":"urn:ietf:params:oauth:token-type:access_token",
		"token_type":"Bearer","expires_in":300,"scope":"notas:leitura"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
}

func TestTokenExchange_SemBasic_Retorna401SemChamarServico(t *testing.T) {
	svc := &fakeOAuthService{}

	rec := exchangeRequest(svc, exchangeForm(), "", "")

	assertInvalidClient(t, rec)
	assert.Empty(t, svc.gotExchange.SubjectToken)
}

func TestTokenExchange_BasicMalCodificado_Retorna401SemEcoarNemChamarServico(t *testing.T) {
	for _, cred := range []struct{ user, password string }{{"https%zz", "senha-correta-1"}, {recursoDeTeste, "seg%zz"}} {
		svc := &fakeOAuthService{}

		rec := exchangeRequest(svc, exchangeForm(), cred.user, cred.password)

		assertInvalidClient(t, rec)
		assert.Empty(t, svc.gotExchange.SubjectToken, cred.user)
		assert.NotContains(t, rec.Body.String(), cred.user)
		assert.NotContains(t, rec.Body.String(), cred.password)
	}
}

func TestTokenExchange_BasicComUsuarioOuSenhaVazios_Retorna401SemChamarServico(t *testing.T) {
	for _, header := range []string{"Basic Og==", "Basic " + basicCredentials("", "segredo"), "Basic " + basicCredentials(url.QueryEscape(recursoDeTeste), "")} {
		svc := &fakeOAuthService{}
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(exchangeForm().Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", header)

		rec := serveOAuth(svc, req)

		assertInvalidClient(t, rec)
		assert.Equal(t, oauth.ExchangeGrant{}, svc.gotExchange, header)
	}
}

func basicCredentials(user, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
}

func TestTokenExchange_ServicoRecusaOResource_Retorna401ComDesafio(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{Code: "invalid_client", Description: "resource nao autenticado"}}

	rec := exchangeRequest(svc, exchangeForm(), recursoDeTeste, "errado")

	assertInvalidClient(t, rec)
	assert.JSONEq(t, `{"error":"invalid_client","error_description":"resource nao autenticado"}`, rec.Body.String())
}

func TestTokenExchange_OutroErroDoProtocolo_Retorna400SemDesafio(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{Code: "invalid_target", Description: "audience x"}}

	rec := exchangeRequest(svc, exchangeForm(), recursoDeTeste, "segredo")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid_target","error_description":"audience x"}`, rec.Body.String())
	assert.Empty(t, rec.Header().Get("WWW-Authenticate"))
}

func TestTokenExchange_ParametroRepetido_Retorna400SemChamarServico(t *testing.T) {
	form := exchangeForm()
	form.Add("subject_token", "outro")
	svc := &fakeOAuthService{}

	rec := exchangeRequest(svc, form, recursoDeTeste, "segredo")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
	assert.Empty(t, svc.gotExchange.SubjectToken)
}

func TestTokenExchange_ErroInterno_Retorna500SemCache(t *testing.T) {
	rec := exchangeRequest(&fakeOAuthService{err: errors.New("banco fora")}, exchangeForm(), recursoDeTeste, "segredo")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}
