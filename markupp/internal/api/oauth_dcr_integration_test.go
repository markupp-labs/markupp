package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registrarPorDCR faz o POST /oauth/register com o corpo JSON dado.
func registrarPorDCR(t *testing.T, server *httptest.Server, body string) (int, map[string]any) {
	t.Helper()
	resp := postJSON(t, server.URL+"/oauth/register", body)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, decodeJSON(t, resp.Body)
}

func TestIntegration_OAuthDCR_RegistraAutorizaETrocaOCodigo(t *testing.T) {
	server, _ := setupOAuthIntegrationServerWith(t, opcoesDeIntegracaoOAuth{dcr: true})

	status, cliente := registrarPorDCR(t, server, `{"client_name":"Agente DCR","redirect_uris":["http://127.0.0.1/cb"]}`)
	require.Equal(t, http.StatusCreated, status, cliente)
	clientID, _ := cliente["client_id"].(string)
	require.NotEmpty(t, clientID)
	retorno := aprovarPedido(t, server, loginDeIntegracao(t, server), iniciarAutorizacaoDe(t, server, clientID, escoposDoAgente))
	status, tokens := trocarCodigoDe(t, server, clientID, retorno.Get("code"))

	require.Equal(t, http.StatusOK, status, tokens)
	assert.NotEmpty(t, tokens["access_token"])
	assert.NotEmpty(t, tokens["refresh_token"])
	assert.Equal(t, escoposDoAgente, tokens["scope"])
}

func TestIntegration_OAuthDCR_RedirectJavascript_Retorna400SemGravar(t *testing.T) {
	server, pool := setupOAuthIntegrationServerWith(t, opcoesDeIntegracaoOAuth{dcr: true})

	status, body := registrarPorDCR(t, server, `{"client_name":"Agente","redirect_uris":["javascript:alert(1)"]}`)

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_redirect_uri", body["error"])
	var clientes int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM oauth_clients").Scan(&clientes))
	assert.Equal(t, 1, clientes, "so o cliente semeado")
}

func TestIntegration_OAuthDCR_Desligado_Retorna404ESemRegistrationEndpoint(t *testing.T) {
	server, _ := setupOAuthIntegrationServer(t)

	status, _ := registrarPorDCR(t, server, `{"client_name":"Agente","redirect_uris":["http://127.0.0.1/cb"]}`)

	assert.Equal(t, http.StatusNotFound, status)
	resp, err := http.Get(server.URL + "/.well-known/oauth-authorization-server")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, decodeJSON(t, resp.Body), "registration_endpoint")
}
