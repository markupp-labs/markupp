package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

const (
	issuerDeIntegracao   = "https://notas.markupp.dev"
	painelDeIntegracao   = "https://painel.markupp.dev"
	resourceDeIntegracao = "https://mcp.markupp.dev"
	retornoDoAgente      = "http://127.0.0.1:51234/cb"
	verifierDaRFC        = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challengeDaRFC       = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
)

// semSeguirRedirect devolve o 302 em vez de segui-lo, para o teste ler o Location.
var semSeguirRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}}

func setupOAuthIntegrationServer(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	pool := storagetest.EmptyPool(t)
	require.NoError(t, storage.Migrate(context.Background(), pool))
	_, err := pool.Exec(context.Background(), `INSERT INTO oauth_clients (id, source, name, redirect_uris)
		VALUES ('agente', 'dcr', 'Agente de teste', ARRAY['http://127.0.0.1/cb'])`)
	require.NoError(t, err)

	codec := auth.NewTokenCodec("segredo-para-teste-de-integracao-de-oauth", 15*time.Minute, time.Now)
	authCfg := auth.ServiceConfig{DefaultTenantID: "default", LocalEnabled: true, RefreshExpirationPeriod: 24 * time.Hour}
	authSvc := auth.NewService(storage.NewPostgresAuthRepository(pool), codec, auth.NewBcryptHasher(), authCfg, time.Now)
	oauthSvc := oauth.NewService(storage.NewPostgresOAuthRepository(pool), oauth.ServiceConfig{
		Issuer: issuerDeIntegracao, PanelURL: painelDeIntegracao, Resources: []string{resourceDeIntegracao},
		AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RequestTTL: 10 * time.Minute, CodeTTL: time.Minute,
	}, codec, time.Now, uuid.NewString, func() string { return uuid.NewString() + uuid.NewString() })
	router := api.NewRouter(api.RouterDeps{
		Auth: authSvc, Tokens: codec, Tenants: authSvc, Probe: pool,
		OAuthMetadata: &api.OAuthMetadata{Issuer: issuerDeIntegracao}, OAuth: oauthSvc,
	})

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, pool
}

func loginDeIntegracao(t *testing.T, server *httptest.Server) string {
	t.Helper()
	resp := postJSON(t, server.URL+"/v1/auth/login", `{"email":"admin@markupp.dev","password":"senhaForte123"}`)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	token, _ := decodeJSON(t, resp.Body)["access_token"].(string)
	require.NotEmpty(t, token)
	return token
}

func iniciarAutorizacao(t *testing.T, server *httptest.Server) string {
	t.Helper()
	query := url.Values{
		"response_type": {"code"}, "client_id": {"agente"}, "redirect_uri": {retornoDoAgente},
		"scope": {"notas:leitura notas:escrita"}, "state": {"estado-1"}, "resource": {resourceDeIntegracao},
		"code_challenge": {challengeDaRFC}, "code_challenge_method": {"S256"},
	}
	resp, err := semSeguirRedirect.Get(server.URL + "/oauth/authorize?" + query.Encode())
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	consent, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, painelDeIntegracao+"/oauth/consentimento", consent.Scheme+"://"+consent.Host+consent.Path)
	return consent.Query().Get("pedido")
}

func aprovarPedido(t *testing.T, server *httptest.Server, accountToken, requestID string) url.Values {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/oauth/requests/"+requestID+"/approve", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accountToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	redirectTo, _ := decodeJSON(t, resp.Body)["redirect_to"].(string)
	parsed, err := url.Parse(redirectTo)
	require.NoError(t, err)
	return parsed.Query()
}

func trocarCodigo(t *testing.T, server *httptest.Server, code string) (int, map[string]any) {
	t.Helper()
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {verifierDaRFC},
		"client_id": {"agente"}, "redirect_uri": {retornoDoAgente}, "resource": {resourceDeIntegracao},
	}
	resp := postarFormOAuth(t, server, "/oauth/token", form)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, decodeJSON(t, resp.Body)
}

func TestIntegration_OAuth_AuthorizeAprovacaoETroca(t *testing.T) {
	server, _ := setupOAuthIntegrationServer(t)
	accountToken := loginDeIntegracao(t, server)

	requestID := iniciarAutorizacao(t, server)
	require.NotEmpty(t, requestID)
	retorno := aprovarPedido(t, server, accountToken, requestID)

	assert.Equal(t, "estado-1", retorno.Get("state"))
	assert.Equal(t, issuerDeIntegracao, retorno.Get("iss"))
	status, tokens := trocarCodigo(t, server, retorno.Get("code"))
	require.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, tokens["access_token"])
	assert.NotEmpty(t, tokens["refresh_token"])
	assert.Equal(t, "Bearer", tokens["token_type"])
	assert.Equal(t, "notas:leitura notas:escrita", tokens["scope"])
}

func TestIntegration_OAuth_CodigoReusado_RecusaERevogaOsTokens(t *testing.T) {
	server, pool := setupOAuthIntegrationServer(t)
	retorno := aprovarPedido(t, server, loginDeIntegracao(t, server), iniciarAutorizacao(t, server))
	status, _ := trocarCodigo(t, server, retorno.Get("code"))
	require.Equal(t, http.StatusOK, status)

	status, body := trocarCodigo(t, server, retorno.Get("code"))

	requireInvalidGrant(t, status, body)
	assert.Zero(t, tokensAtivos(t, pool), "reuso do código revoga os tokens da Autorização")
}

// autorizarAgente percorre o fluxo inteiro e devolve o token da conta e o par emitido.
func autorizarAgente(t *testing.T, server *httptest.Server) (string, map[string]any) {
	t.Helper()
	accountToken := loginDeIntegracao(t, server)
	retorno := aprovarPedido(t, server, accountToken, iniciarAutorizacao(t, server))
	status, tokens := trocarCodigo(t, server, retorno.Get("code"))
	require.Equal(t, http.StatusOK, status)
	return accountToken, tokens
}

func postarFormOAuth(t *testing.T, server *httptest.Server, path string, form url.Values) *http.Response {
	t.Helper()
	resp, err := http.Post(server.URL+path, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	return resp
}

func renovar(t *testing.T, server *httptest.Server, refresh any) (int, map[string]any) {
	t.Helper()
	return renovarCom(t, server, refresh, url.Values{})
}

// renovarCom renova o refresh com os parâmetros extras do formulário.
func renovarCom(t *testing.T, server *httptest.Server, refresh any, extras url.Values) (int, map[string]any) {
	t.Helper()
	refreshToken, _ := refresh.(string)
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {"agente"}}
	for name, values := range extras {
		form[name] = values
	}
	resp := postarFormOAuth(t, server, "/oauth/token", form)
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, decodeJSON(t, resp.Body)
}

func tokensAtivos(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var ativos int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM oauth_tokens WHERE NOT revoked`).Scan(&ativos))
	return ativos
}

func requireInvalidGrant(t *testing.T, status int, body map[string]any) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_grant", body["error"])
}

func TestIntegration_OAuth_RefreshRotaciona_ReusoDerrubaOsTokens(t *testing.T) {
	server, pool := setupOAuthIntegrationServer(t)
	_, primeiro := autorizarAgente(t, server)

	status, segundo := renovar(t, server, primeiro["refresh_token"])
	require.Equal(t, http.StatusOK, status)
	assert.NotEqual(t, primeiro["refresh_token"], segundo["refresh_token"])
	status, terceiro := renovar(t, server, segundo["refresh_token"])
	require.Equal(t, http.StatusOK, status, "o par novo renova")
	assert.Equal(t, "notas:leitura notas:escrita", terceiro["scope"])

	status, body := renovar(t, server, primeiro["refresh_token"])
	requireInvalidGrant(t, status, body)
	assert.Zero(t, tokensAtivos(t, pool), "o reuso derruba os tokens da Autorização")
	status, body = renovar(t, server, terceiro["refresh_token"])
	requireInvalidGrant(t, status, body)
}

func TestIntegration_OAuth_RefreshRecusadoPorScopeOuResource_ContinuaValido(t *testing.T) {
	server, _ := setupOAuthIntegrationServer(t)
	_, tokens := autorizarAgente(t, server)

	status, body := renovarCom(t, server, tokens["refresh_token"], url.Values{"scope": {"administracao"}})
	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_scope", body["error"])
	status, body = renovarCom(t, server, tokens["refresh_token"], url.Values{"resource": {"https://outro.dev/mcp"}})
	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_target", body["error"])

	status, _ = renovar(t, server, tokens["refresh_token"])
	assert.Equal(t, http.StatusOK, status, "o refresh recusado por scope ou resource não foi consumido")
}

func TestIntegration_OAuth_Revoke_DerrubaOsTokensEORefreshSeguinte(t *testing.T) {
	server, pool := setupOAuthIntegrationServer(t)
	_, tokens := autorizarAgente(t, server)
	accessToken, _ := tokens["access_token"].(string)

	resp := postarFormOAuth(t, server, "/oauth/revoke", url.Values{"token": {accessToken}, "client_id": {"agente"}})
	_ = resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Zero(t, tokensAtivos(t, pool))
	status, body := renovar(t, server, tokens["refresh_token"])
	requireInvalidGrant(t, status, body)
}

func listarAutorizacoes(t *testing.T, server *httptest.Server, accountToken string) []map[string]any {
	t.Helper()
	resp := getWithAuth(t, server.URL+"/v1/me/authorizations", accountToken)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var autorizacoes []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&autorizacoes))
	return autorizacoes
}

func apagarAutorizacao(t *testing.T, server *httptest.Server, accountToken string, id any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/v1/me/authorizations/%v", server.URL, id), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accountToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestIntegration_OAuth_AutorizacoesDaConta_ListaERevoga(t *testing.T) {
	server, _ := setupOAuthIntegrationServer(t)
	accountToken, tokens := autorizarAgente(t, server)

	autorizacoes := listarAutorizacoes(t, server, accountToken)
	require.Len(t, autorizacoes, 1)
	assert.Equal(t, "agente", autorizacoes[0]["client_id"])
	assert.Equal(t, "Agente de teste", autorizacoes[0]["client_name"])
	assert.Equal(t, []any{"notas:leitura", "notas:escrita"}, autorizacoes[0]["scopes"])

	require.Equal(t, http.StatusNoContent, apagarAutorizacao(t, server, accountToken, autorizacoes[0]["id"]))
	assert.Empty(t, listarAutorizacoes(t, server, accountToken))
	status, body := renovar(t, server, tokens["refresh_token"])
	requireInvalidGrant(t, status, body)
}
