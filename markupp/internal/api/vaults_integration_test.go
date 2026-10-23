package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idDaConta acha, na lista de membros do tenant que o admin lê, o id da conta email.
func idDaConta(t *testing.T, url, tokenAdmin, email string) string {
	t.Helper()
	resp := getWithAuth(t, url+"/v1/users", tokenAdmin)
	defer func() { _ = resp.Body.Close() }()
	var membros []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&membros))
	for _, m := range membros {
		if m["email"] == email {
			return m["id"].(string)
		}
	}
	t.Fatalf("conta %q fora da lista de membros %v", email, membros)
	return ""
}

func TestIntegration_CofreCompartilhado_ColegaLeANota(t *testing.T) {
	server, _ := setupAuthIntegrationServer(t)
	url := server.URL
	_, admin := sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"admin@markupp.dev","password":"senhaAdmin"}`)
	tokenAdmin := admin["access_token"].(string)
	_, convite := sendJSON(t, http.MethodPost, url+"/users/invites", tokenAdmin, `{"email":"membro@markupp.dev","role":"member"}`)
	_, membro := sendJSON(t, http.MethodPost, url+"/auth/invites/accept", "", `{"token":"`+convite["token"].(string)+`","password":"senhaMembro"}`)
	tokenMembro := membro["access_token"].(string)

	status, cofre := sendJSON(t, http.MethodPost, url+"/v1/vaults", tokenAdmin, `{"name":" Time "}`)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "Time", cofre["name"])
	vaultURL := url + "/v1/vaults/" + cofre["id"].(string)
	status, nota := sendJSON(t, http.MethodPost, vaultURL+"/notes", tokenAdmin, `{"path":"plano.md","content":"do time"}`)
	require.Equal(t, http.StatusCreated, status)
	notaURL := url + "/v1/notes/" + nota["id"].(string)

	status, _ = sendJSON(t, http.MethodGet, notaURL, tokenMembro, "")
	require.Equal(t, http.StatusNotFound, status, "antes de entrar no cofre o colega nao ve a nota")

	idMembro := idDaConta(t, url, tokenAdmin, "membro@markupp.dev")
	status, adicionado := sendJSON(t, http.MethodPost, vaultURL+"/members", tokenAdmin, `{"user_id":"`+idMembro+`"}`)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "membro@markupp.dev", adicionado["email"])

	status, lida := sendJSON(t, http.MethodGet, notaURL, tokenMembro, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "do time", lida["content"])
}
