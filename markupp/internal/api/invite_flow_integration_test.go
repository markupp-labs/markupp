package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
)

func sendJSON(t *testing.T, method, url, token, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func countNotes(t *testing.T, baseURL, token string) int {
	t.Helper()
	resp := getWithAuth(t, baseURL+"/notes", token)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var lista []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&lista))
	return len(lista)
}

func TestIntegration_ConviteAteNotaIsolada(t *testing.T) {
	server, _ := setupAuthIntegrationServer(t)
	url := server.URL

	status, admin := sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"admin@markupp.dev","password":"senhaAdmin"}`)
	require.Equal(t, http.StatusOK, status)
	tokenAdmin := admin["access_token"].(string)

	status, convite := sendJSON(t, http.MethodPost, url+"/users/invites", tokenAdmin, `{"email":"membro@markupp.dev","role":"member"}`)
	require.Equal(t, http.StatusCreated, status)

	status, membro := sendJSON(t, http.MethodPost, url+"/auth/invites/accept", "", `{"token":"`+convite["token"].(string)+`","password":"senhaMembro"}`)
	require.Equal(t, http.StatusOK, status)
	tokenMembro := membro["access_token"].(string)

	status, _ = sendJSON(t, http.MethodPost, url+"/auth/invites/accept", "", `{"token":"`+convite["token"].(string)+`","password":"outra"}`)
	assert.Equal(t, http.StatusBadRequest, status, "convite e de uso unico")

	status, _ = sendJSON(t, http.MethodPost, url+"/notes", tokenMembro, `{"path":"privada.md","content":"so minha"}`)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, 1, countNotes(t, url, tokenMembro))
	assert.Equal(t, 0, countNotes(t, url, tokenAdmin), "admin nao e membro do cofre do convidado")

	status, _ = sendJSON(t, http.MethodGet, url+"/users", tokenMembro, "")
	assert.Equal(t, http.StatusForbidden, status)
	status, _ = sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"membro@markupp.dev","password":"senhaMembro"}`)
	assert.Equal(t, http.StatusOK, status, "convidado entra depois com a senha escolhida")
}

// sendAs é sendJSON com o tenant escolhido no cabeçalho.
func sendAs(t *testing.T, method, url, token, tenantID, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(api.TenantHeader, tenantID)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func firstPendingInvite(t *testing.T, url, token string) string {
	t.Helper()
	resp := getWithAuth(t, url+"/me/invites", token)
	defer func() { _ = resp.Body.Close() }()
	var pendentes []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pendentes))
	require.Len(t, pendentes, 1)
	return pendentes[0]["id"].(string)
}

func TestIntegration_ContaGlobalEmDoisTenantsComPapeis(t *testing.T) {
	server, _ := setupAuthIntegrationServer(t)
	url := server.URL
	_, sessaoA := sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"ana@x.dev","password":"senha-ana"}`)
	tokenA := sessaoA["access_token"].(string)
	_, convite := sendJSON(t, http.MethodPost, url+"/users/invites", tokenA, `{"email":"bia@x.dev","role":"member"}`)
	_, sessaoB := sendJSON(t, http.MethodPost, url+"/auth/invites/accept", "", `{"token":"`+convite["token"].(string)+`","password":"senha-bia"}`)
	tokenB := sessaoB["access_token"].(string)
	status, tenantB := sendJSON(t, http.MethodPost, url+"/tenants", tokenB, `{"name":"Estudio da Bia"}`)
	require.Equal(t, http.StatusCreated, status)

	status, _ = sendJSON(t, http.MethodGet, url+"/notes", tokenB, "")
	assert.Equal(t, http.StatusBadRequest, status, "em dois tenants, bia precisa escolher")
	status, _ = sendAs(t, http.MethodPost, url+"/notes", tokenB, tenantB["id"].(string), `{"path":"meu.md","content":"x"}`)
	assert.Equal(t, http.StatusCreated, status, "owner do proprio tenant escreve")

	_, convite2 := sendAs(t, http.MethodPost, url+"/users/invites", tokenB, tenantB["id"].(string), `{"email":"ana@x.dev","role":"viewer"}`)
	require.NotEmpty(t, convite2["id"])
	status, _ = sendJSON(t, http.MethodPost, url+"/me/invites/"+firstPendingInvite(t, url, tokenA)+"/accept", tokenA, "")
	require.Equal(t, http.StatusOK, status)
	status, _ = sendAs(t, http.MethodPost, url+"/notes", tokenA, tenantB["id"].(string), `{"path":"da-ana.md","content":"x"}`)
	assert.Equal(t, http.StatusForbidden, status, "viewer nao escreve")

	status, _ = sendAs(t, http.MethodGet, url+"/users", tokenA, tenantB["id"].(string), "")
	assert.Equal(t, http.StatusForbidden, status, "viewer nao administra")
	status, _ = sendAs(t, http.MethodDelete, url+"/users/"+sessaoUserID(t, url, tokenB), tokenB, tenantB["id"].(string), "")
	assert.Equal(t, http.StatusConflict, status, "o ultimo owner nao sai")
}

func sessaoUserID(t *testing.T, url, token string) string {
	t.Helper()
	resp := getWithAuth(t, url+"/auth/me", token)
	defer func() { _ = resp.Body.Close() }()
	return decodeJSON(t, resp.Body)["id"].(string)
}
