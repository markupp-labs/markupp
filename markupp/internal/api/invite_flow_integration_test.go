package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

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

	status, _ = sendJSON(t, http.MethodGet, url+"/users/invites", tokenMembro, "")
	assert.Equal(t, http.StatusForbidden, status, "convites seguem so para admin")
	assert.NotEmpty(t, idDaConta(t, url, tokenMembro, "admin@markupp.dev"), "member lista os colegas")
	status, _ = sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"membro@markupp.dev","password":"senhaMembro"}`)
	assert.Equal(t, http.StatusOK, status, "convidado entra depois com a senha escolhida")
}

func meusTenants(t *testing.T, url, token string) []map[string]any {
	t.Helper()
	resp := getWithAuth(t, url+"/me/tenants", token)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var tenants []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tenants))
	require.Len(t, tenants, 1)
	return tenants
}

func TestIntegration_MeusTenants_ContaSoAsNotasDosCofresDaConta(t *testing.T) {
	server, _ := setupAuthIntegrationServer(t)
	url := server.URL
	_, admin := sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"admin@markupp.dev","password":"senhaAdmin"}`)
	tokenAdmin := admin["access_token"].(string)
	_, convite := sendJSON(t, http.MethodPost, url+"/users/invites", tokenAdmin, `{"email":"membro@markupp.dev","role":"member"}`)
	_, membro := sendJSON(t, http.MethodPost, url+"/auth/invites/accept", "", `{"token":"`+convite["token"].(string)+`","password":"senhaMembro"}`)
	tokenMembro := membro["access_token"].(string)
	status, nota := sendJSON(t, http.MethodPost, url+"/notes", tokenAdmin, `{"path":"do-admin.md","content":"x"}`)
	require.Equal(t, http.StatusCreated, status)
	for _, path := range []string{"a.md", "b.md"} {
		status, _ = sendJSON(t, http.MethodPost, url+"/notes", tokenMembro, `{"path":"`+path+`","content":"x"}`)
		require.Equal(t, http.StatusCreated, status)
	}

	doAdmin, doMembro := meusTenants(t, url, tokenAdmin)[0], meusTenants(t, url, tokenMembro)[0]

	assert.Equal(t, float64(1), doAdmin["notes_count"], "o admin nao le o cofre do membro")
	assert.Equal(t, float64(2), doMembro["notes_count"])
	ultimaTexto, _ := doAdmin["last_note_at"].(string)
	ultima, err := time.Parse(time.RFC3339, ultimaTexto)
	require.NoError(t, err)
	criadaTexto, _ := nota["updated_at"].(string)
	criada, err := time.Parse(time.RFC3339, criadaTexto)
	require.NoError(t, err)
	assert.WithinDuration(t, criada, ultima, time.Millisecond)
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

	status, _ = sendAs(t, http.MethodGet, url+"/users/invites", tokenA, tenantB["id"].(string), "")
	assert.Equal(t, http.StatusForbidden, status, "viewer nao administra")
	status, _ = sendAs(t, http.MethodGet, url+"/users", tokenA, tenantB["id"].(string), "")
	assert.Equal(t, http.StatusOK, status, "viewer lista os colegas")
	status, _ = sendAs(t, http.MethodDelete, url+"/users/"+sessaoUserID(t, url, tokenB), tokenB, tenantB["id"].(string), "")
	assert.Equal(t, http.StatusConflict, status, "o ultimo owner nao sai")
}

func sessaoUserID(t *testing.T, url, token string) string {
	t.Helper()
	resp := getWithAuth(t, url+"/auth/me", token)
	defer func() { _ = resp.Body.Close() }()
	return decodeJSON(t, resp.Body)["id"].(string)
}
