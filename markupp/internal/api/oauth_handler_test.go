package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

const issuerDeTeste = "https://markupp.exemplo"

// fakeOAuthService registra o que o roteador repassou e devolve as respostas programadas.
type fakeOAuthService struct {
	consentURL string
	view       oauth.RequestView
	redirectTo string
	tokens     oauth.TokenResponse
	err        error

	authorizations []oauth.AuthorizationView

	gotParams          oauth.AuthorizeParams
	gotGrant           oauth.CodeGrant
	gotRefresh         oauth.RefreshGrant
	gotRevoke          oauth.RevokeRequest
	gotRequestID       string
	gotUserID          string
	gotAuthorizationID string
	decision           string
}

func (f *fakeOAuthService) StartAuthorization(_ context.Context, p oauth.AuthorizeParams) (string, error) {
	f.gotParams = p
	return f.consentURL, f.err
}

func (f *fakeOAuthService) DescribeRequest(_ context.Context, requestID, userID string) (oauth.RequestView, error) {
	f.gotRequestID, f.gotUserID = requestID, userID
	return f.view, f.err
}

func (f *fakeOAuthService) Approve(_ context.Context, requestID, userID string) (string, error) {
	f.gotRequestID, f.gotUserID, f.decision = requestID, userID, "approve"
	return f.redirectTo, f.err
}

func (f *fakeOAuthService) Deny(_ context.Context, requestID, userID string) (string, error) {
	f.gotRequestID, f.gotUserID, f.decision = requestID, userID, "deny"
	return f.redirectTo, f.err
}

func (f *fakeOAuthService) ExchangeCode(_ context.Context, g oauth.CodeGrant) (oauth.TokenResponse, error) {
	f.gotGrant = g
	return f.tokens, f.err
}

func (f *fakeOAuthService) RefreshToken(_ context.Context, g oauth.RefreshGrant) (oauth.TokenResponse, error) {
	f.gotRefresh = g
	return f.tokens, f.err
}

func (f *fakeOAuthService) RevokeToken(_ context.Context, r oauth.RevokeRequest) error {
	f.gotRevoke = r
	return f.err
}

func (f *fakeOAuthService) ListAuthorizations(_ context.Context, userID string) ([]oauth.AuthorizationView, error) {
	f.gotUserID = userID
	return f.authorizations, f.err
}

func (f *fakeOAuthService) RevokeAuthorization(_ context.Context, userID, authorizationID string) error {
	f.gotUserID, f.gotAuthorizationID = userID, authorizationID
	return f.err
}

func oauthRouter(svc *fakeOAuthService) http.Handler {
	return oauthRouterComOrigens(svc, nil)
}

// oauthRouterComOrigens monta o roteador OAuth com a lista de origens do CORS da API.
func oauthRouterComOrigens(svc *fakeOAuthService, origins []string) http.Handler {
	return api.NewRouter(api.RouterDeps{
		Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}}, Probe: &fakeProbe{}, AllowedOrigins: origins,
		OAuth: svc, OAuthMetadata: &api.OAuthMetadata{Issuer: issuerDeTeste},
	})
}

func serveOAuth(svc *fakeOAuthService, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	oauthRouter(svc).ServeHTTP(rec, req)
	return rec
}

func authorizeRequest(svc *fakeOAuthService, query string) *httptest.ResponseRecorder {
	return serveOAuth(svc, httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query, nil))
}

func tokenRequest(svc *fakeOAuthService, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return serveOAuth(svc, req)
}

func TestNewRouter_OAuthSemMetadata_EntraEmPanic(t *testing.T) {
	assert.Panics(t, func() {
		api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}, OAuth: &fakeOAuthService{}})
	})
}

func TestOAuthRotas_SemServico_Retorna404(t *testing.T) {
	router := api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}, Probe: &fakeProbe{}, OAuthMetadata: &api.OAuthMetadata{Issuer: issuerDeTeste}})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/oauth/authorize", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAuthorize_LeOsParametrosDaQuery(t *testing.T) {
	svc := &fakeOAuthService{consentURL: "https://painel.exemplo/consent?id=p1"}

	authorizeRequest(svc, "response_type=code&client_id=https%3A%2F%2Fcli.exemplo%2Fmeta&redirect_uri=https%3A%2F%2Fcli.exemplo%2Fcb"+
		"&scope=notas%3Aleitura&state=xyz&code_challenge=abc&code_challenge_method=S256&resource=https%3A%2F%2Fmcp.exemplo")

	assert.Equal(t, oauth.AuthorizeParams{
		ResponseType: "code", ClientID: "https://cli.exemplo/meta", RedirectURI: "https://cli.exemplo/cb",
		Scope: "notas:leitura", State: "xyz", CodeChallenge: "abc", CodeChallengeMethod: "S256", Resource: "https://mcp.exemplo",
	}, svc.gotParams)
}

func TestAuthorize_Sucesso_RedirecionaAoConsentimento(t *testing.T) {
	svc := &fakeOAuthService{consentURL: "https://painel.exemplo/consent?id=p1"}

	rec := authorizeRequest(svc, "client_id=c1")

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "https://painel.exemplo/consent?id=p1", rec.Header().Get("Location"))
	assert.Empty(t, rec.Header().Get("Deprecation"))
}

func TestAuthorize_ErroComRedirect_VoltaAoClienteComIssuer(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{Code: "invalid_scope", Description: "escopo x", RedirectURI: "https://cli.exemplo/cb", State: "s1"}}

	rec := authorizeRequest(svc, "client_id=c1")

	require.Equal(t, http.StatusFound, rec.Code)
	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "cli.exemplo", location.Host)
	assert.Equal(t, "invalid_scope", location.Query().Get("error"))
	assert.Equal(t, "s1", location.Query().Get("state"))
	assert.Equal(t, issuerDeTeste, location.Query().Get("iss"))
}

func TestAuthorize_ErroEmbrulhadoComRedirect_VoltaAoCliente(t *testing.T) {
	protoErr := &oauth.ProtocolError{Code: "access_denied", Description: "x", RedirectURI: "https://cli.exemplo/cb"}
	svc := &fakeOAuthService{err: fmt.Errorf("iniciar: %w", protoErr)}

	rec := authorizeRequest(svc, "client_id=c1")

	require.Equal(t, http.StatusFound, rec.Code)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Location"), "https://cli.exemplo/cb?"), rec.Header().Get("Location"))
}

func TestAuthorize_ErroSemRedirect_Retorna400ComErroOAuth(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{Code: "invalid_request", Description: "redirect_uri não registrada"}}

	rec := authorizeRequest(svc, "client_id=c1")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid_request","error_description":"redirect_uri não registrada"}`, rec.Body.String())
}

func TestAuthorize_ErroEmbrulhadoSemRedirect_Retorna400(t *testing.T) {
	svc := &fakeOAuthService{err: errors.Join(&oauth.ProtocolError{Code: "invalid_request", Description: "x"})}

	rec := authorizeRequest(svc, "client_id=c1")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthorize_ParametroRepetido_Retorna400SemChamarServico(t *testing.T) {
	svc := &fakeOAuthService{}

	rec := authorizeRequest(svc, "client_id=c1&state=a&state=b")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
	assert.Empty(t, svc.gotParams.ClientID)
}

func TestAuthorize_VariosParametrosRepetidos_CitaSempreOPrimeiroEmOrdem(t *testing.T) {
	for range 20 {
		rec := authorizeRequest(&fakeOAuthService{}, "state=a&state=b&scope=x&scope=y&client_id=1&client_id=2&resource=r&resource=s")

		assert.Contains(t, rec.Body.String(), `'client_id'`)
	}
}

func TestAuthorize_QueryMalformada_Retorna400SemChamarServico(t *testing.T) {
	svc := &fakeOAuthService{}

	rec := authorizeRequest(svc, "client_id=c1&state=%zz")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
	assert.Empty(t, svc.gotParams.ClientID)
}

func TestAuthorize_ErroInterno_Retorna500(t *testing.T) {
	rec := authorizeRequest(&fakeOAuthService{err: errors.New("banco fora")}, "client_id=c1")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal","message":"erro interno"}`, rec.Body.String())
}

func codeForm() url.Values {
	return url.Values{
		"grant_type": {"authorization_code"}, "code": {"cod"}, "code_verifier": {"ver"},
		"client_id": {"c1"}, "redirect_uri": {"https://cli.exemplo/cb"}, "resource": {"https://mcp.exemplo"},
	}
}

func TestToken_LeOFormularioParaOGrant(t *testing.T) {
	svc := &fakeOAuthService{}

	tokenRequest(svc, codeForm())

	assert.Equal(t, oauth.CodeGrant{Code: "cod", CodeVerifier: "ver", ClientID: "c1", RedirectURI: "https://cli.exemplo/cb", Resource: "https://mcp.exemplo"}, svc.gotGrant)
}

func TestToken_Sucesso_Retorna200SemCache(t *testing.T) {
	svc := &fakeOAuthService{tokens: oauth.TokenResponse{AccessToken: "at", TokenType: "Bearer", ExpiresIn: 3600, RefreshToken: "rt", Scope: "notas:leitura"}}

	rec := tokenRequest(svc, codeForm())

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"access_token":"at","token_type":"Bearer","expires_in":3600,"refresh_token":"rt","scope":"notas:leitura"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestToken_ErroDoProtocolo_Retorna400SemCache(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{Code: "invalid_grant", Description: "código usado"}}

	rec := tokenRequest(svc, codeForm())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid_grant","error_description":"código usado"}`, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestToken_ErroDoProtocoloComRedirect_Retorna400SemRedirecionar(t *testing.T) {
	svc := &fakeOAuthService{err: &oauth.ProtocolError{Code: "invalid_grant", Description: "código usado", RedirectURI: "https://cli.exemplo/cb", State: "s1"}}

	rec := tokenRequest(svc, codeForm())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, rec.Header().Get("Location"))
	assert.JSONEq(t, `{"error":"invalid_grant","error_description":"código usado"}`, rec.Body.String())
}

func TestToken_GrantTypeAusenteOuOutro_Retorna400UnsupportedGrantType(t *testing.T) {
	for _, grant := range []string{"", "password", "client_credentials"} {
		form := codeForm()
		form.Set("grant_type", grant)
		svc := &fakeOAuthService{}

		rec := tokenRequest(svc, form)

		assert.Equal(t, http.StatusBadRequest, rec.Code, grant)
		assert.Contains(t, rec.Body.String(), `"error":"unsupported_grant_type"`, grant)
		assert.Empty(t, svc.gotGrant.Code, grant)
	}
}

func TestToken_ParametroRepetido_Retorna400InvalidRequest(t *testing.T) {
	form := codeForm()
	form.Add("code", "outro")
	svc := &fakeOAuthService{}

	rec := tokenRequest(svc, form)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
	assert.Empty(t, svc.gotGrant.Code)
}

func TestToken_CorpoNaoFormulario_Retorna400InvalidRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(`{"grant_type":"authorization_code"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := serveOAuth(&fakeOAuthService{}, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
}

func TestToken_ParametrosNaQuery_SaoIgnorados(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token?grant_type=authorization_code&code=cod", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	svc := &fakeOAuthService{}

	rec := serveOAuth(svc, req)

	assert.Contains(t, rec.Body.String(), `"error":"unsupported_grant_type"`)
	assert.Empty(t, svc.gotGrant.Code)
}

func TestToken_ErroInterno_Retorna500SemCache(t *testing.T) {
	rec := tokenRequest(&fakeOAuthService{err: errors.New("banco fora")}, codeForm())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestToken_Preflight_Retorna204ComCORSAberto(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/oauth/token", nil)
	req.Header.Set("Origin", "https://cliente.exemplo")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)

	rec := serveOAuth(&fakeOAuthService{}, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), http.MethodPost)
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestToken_PreflightDeOrigemDaLista_UsaOCORSAbertoDoToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/oauth/token", nil)
	req.Header.Set("Origin", origemPermitida)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	oauthRouterComOrigens(&fakeOAuthService{}, []string{origemPermitida}).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Empty(t, rec.Header().Get("Vary"))
}

func TestOAuthProtocolo_MetodoNaoSuportado_Retorna405SemObsolescencia(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/oauth/token"},
		{http.MethodPost, "/oauth/authorize"},
		{http.MethodHead, "/oauth/authorize"},
	} {
		rec := serveOAuth(&fakeOAuthService{}, httptest.NewRequest(route.method, route.path, nil))

		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, route.method+" "+route.path)
		assert.Empty(t, rec.Header().Get("Deprecation"), route.method+" "+route.path)
		assert.Empty(t, rec.Header().Get("Link"), route.method+" "+route.path)
	}
}

func oauthRequestDeOutraOrigem(method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Origin", "https://cliente.exemplo")
	return serveOAuth(&fakeOAuthService{}, req)
}

func TestOAuthPublico_ErroDeRoteamento_RecebeCORSAberto(t *testing.T) {
	for _, route := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/oauth/token", http.StatusMethodNotAllowed},
		{http.MethodGet, "/oauth/inexistente", http.StatusNotFound},
		{http.MethodPost, "/.well-known/oauth-authorization-server", http.StatusMethodNotAllowed},
		{http.MethodGet, "/.well-known/oauth-authorization-server/inexistente", http.StatusNotFound},
	} {
		rec := oauthRequestDeOutraOrigem(route.method, route.path)

		assert.Equal(t, route.status, rec.Code, route.method+" "+route.path)
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"), route.method+" "+route.path)
	}
}

func TestAuthorize_QualquerResposta_NaoRecebeCORS(t *testing.T) {
	for _, route := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/oauth/authorize?client_id=c1", http.StatusFound},
		{http.MethodGet, "/oauth/authorize?client_id=c1&client_id=c2", http.StatusBadRequest},
		{http.MethodPost, "/oauth/authorize", http.StatusMethodNotAllowed},
		{http.MethodOptions, "/oauth/authorize", http.StatusMethodNotAllowed},
	} {
		rec := oauthRequestDeOutraOrigem(route.method, route.path)

		assert.Equal(t, route.status, rec.Code, route.method+" "+route.path)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"), route.method+" "+route.path)
	}
}
