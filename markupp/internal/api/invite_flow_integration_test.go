package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
