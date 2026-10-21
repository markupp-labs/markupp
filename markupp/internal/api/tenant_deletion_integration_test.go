package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contaConvidada convida email para tenantID com role, pela sessão de quem convida, e
// devolve a sessão da conta criada pelo token do convite.
func contaConvidada(t *testing.T, url, tokenDeQuemConvida, tenantID, email, role string) string {
	t.Helper()
	status, convite := sendAs(t, http.MethodPost, url+"/users/invites", tokenDeQuemConvida, tenantID, `{"email":"`+email+`","role":"`+role+`"}`)
	require.Equal(t, http.StatusCreated, status, email)
	status, sessao := sendJSON(t, http.MethodPost, url+"/auth/invites/accept", "", `{"token":"`+convite["token"].(string)+`","password":"senha-`+role+`"}`)
	require.Equal(t, http.StatusOK, status, email)
	return sessao["access_token"].(string)
}

func tenantsDaConta(t *testing.T, url, token string) []string {
	t.Helper()
	resp := getWithAuth(t, url+"/me/tenants", token)
	defer func() { _ = resp.Body.Close() }()
	var lista []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&lista))
	ids := make([]string, 0, len(lista))
	for _, tenant := range lista {
		ids = append(ids, tenant["id"].(string))
	}
	return ids
}

func TestIntegration_ApagarTenant_SoOOwnerApagaETudoSome(t *testing.T) {
	server, _ := setupAuthIntegrationServer(t)
	url := server.URL
	_, sessaoAna := sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"ana@x.dev","password":"senha-ana"}`)
	tokenAna := sessaoAna["access_token"].(string)
	tenantPadrao := tenantsDaConta(t, url, tokenAna)[0]
	status, criado := sendJSON(t, http.MethodPost, url+"/tenants", tokenAna, `{"name":"Apagavel"}`)
	require.Equal(t, http.StatusCreated, status)
	apagavel := criado["id"].(string)
	status, _ = sendAs(t, http.MethodPost, url+"/notes", tokenAna, apagavel, `{"path":"some.md","content":"x"}`)
	require.Equal(t, http.StatusCreated, status)
	status, _ = sendAs(t, http.MethodPost, url+"/notes", tokenAna, tenantPadrao, `{"path":"fica.md","content":"x"}`)
	require.Equal(t, http.StatusCreated, status)
	tokenAdmin := contaConvidada(t, url, tokenAna, apagavel, "admin@x.dev", "admin")
	tokenMembro := contaConvidada(t, url, tokenAna, apagavel, "membro@x.dev", "member")
	tokenLeitor := contaConvidada(t, url, tokenAna, apagavel, "leitor@x.dev", "viewer")
	tokenDeFora := contaConvidada(t, url, tokenAna, tenantPadrao, "fora@x.dev", "member")

	for papel, token := range map[string]string{"admin": tokenAdmin, "member": tokenMembro, "viewer": tokenLeitor} {
		status, corpo := sendJSON(t, http.MethodDelete, url+"/tenants/"+apagavel, token, "")
		assert.Equal(t, http.StatusForbidden, status, papel)
		assert.Equal(t, "forbidden", corpo["error"], papel)
	}
	status, corpo := sendJSON(t, http.MethodDelete, url+"/tenants/"+apagavel, tokenDeFora, "")
	assert.Equal(t, http.StatusNotFound, status, "quem nao e membro nao descobre que o tenant existe")
	assert.Equal(t, "tenant_not_found", corpo["error"])

	status, _ = sendJSON(t, http.MethodDelete, url+"/tenants/"+apagavel, tokenAna, "")
	require.Equal(t, http.StatusNoContent, status)

	assert.Equal(t, []string{tenantPadrao}, tenantsDaConta(t, url, tokenAna))
	assert.Empty(t, tenantsDaConta(t, url, tokenAdmin), "a conta continua, sem o tenant")
	assert.Equal(t, 1, countNotes(t, url, tokenAna), "o outro tenant fica intacto")
	status, _ = sendJSON(t, http.MethodDelete, url+"/tenants/"+apagavel, tokenAna, "")
	assert.Equal(t, http.StatusNotFound, status, "tenant apagado nao existe mais")
}
