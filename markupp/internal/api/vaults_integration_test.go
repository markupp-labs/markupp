package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idDaConta acha, na lista de colegas do tenant que todo membro lê, o id da conta email.
func idDaConta(t *testing.T, url, token, email string) string {
	t.Helper()
	resp := getWithAuth(t, url+"/v1/users", token)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
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

// adminEMembroConvidado sobe o servidor e devolve a URL, o token do admin e o de um member
// que entrou pelo convite dele.
func adminEMembroConvidado(t *testing.T) (url, tokenAdmin, tokenMembro string) {
	t.Helper()
	server, _ := setupAuthIntegrationServer(t)
	url = server.URL
	_, admin := sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"admin@markupp.dev","password":"senhaAdmin"}`)
	tokenAdmin = admin["access_token"].(string)
	_, convite := sendJSON(t, http.MethodPost, url+"/users/invites", tokenAdmin, `{"email":"membro@markupp.dev","role":"member"}`)
	_, membro := sendJSON(t, http.MethodPost, url+"/auth/invites/accept", "", `{"token":"`+convite["token"].(string)+`","password":"senhaMembro"}`)
	return url, tokenAdmin, membro["access_token"].(string)
}

// cofreComNota cria, com token, o cofre de nome dado e nele uma nota com conteudo, e
// devolve as URLs dos dois.
func cofreComNota(t *testing.T, url, token, nome, conteudo string) (vaultURL, notaURL string) {
	t.Helper()
	status, cofre := sendJSON(t, http.MethodPost, url+"/v1/vaults", token, `{"name":"`+nome+`"}`)
	require.Equal(t, http.StatusCreated, status)
	vaultURL = url + "/v1/vaults/" + cofre["id"].(string)
	status, nota := sendJSON(t, http.MethodPost, vaultURL+"/notes", token, `{"path":"nota.md","content":"`+conteudo+`"}`)
	require.Equal(t, http.StatusCreated, status)
	return vaultURL, url + "/v1/notes/" + nota["id"].(string)
}

// assertColegaPassaALer confere que quem tem tokenLeitor não lê a nota antes de entrar no
// cofre e lê depois que tokenDono o adiciona, achando o id pela lista de colegas.
func assertColegaPassaALer(t *testing.T, url, vaultURL, notaURL, tokenDono, tokenLeitor, emailLeitor, conteudo string) {
	t.Helper()
	status, _ := sendJSON(t, http.MethodGet, notaURL, tokenLeitor, "")
	require.Equal(t, http.StatusNotFound, status, "antes de entrar no cofre o colega nao ve a nota")
	id := idDaConta(t, url, tokenDono, emailLeitor)
	status, adicionado := sendJSON(t, http.MethodPost, vaultURL+"/members", tokenDono, `{"user_id":"`+id+`"}`)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, emailLeitor, adicionado["email"])
	status, lida := sendJSON(t, http.MethodGet, notaURL, tokenLeitor, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, conteudo, lida["content"])
}

func TestIntegration_CofreCompartilhado_ColegaLeANota(t *testing.T) {
	url, tokenAdmin, tokenMembro := adminEMembroConvidado(t)

	vaultURL, notaURL := cofreComNota(t, url, tokenAdmin, "Time", "do time")

	assertColegaPassaALer(t, url, vaultURL, notaURL, tokenAdmin, tokenMembro, "membro@markupp.dev", "do time")
}

func TestIntegration_CofreCompartilhado_NomeSaiAparado(t *testing.T) {
	url, tokenAdmin, _ := adminEMembroConvidado(t)

	status, cofre := sendJSON(t, http.MethodPost, url+"/v1/vaults", tokenAdmin, `{"name":" Time "}`)

	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "Time", cofre["name"])
}

func TestIntegration_MemberSemAdministracao_AdicionaColegaAoProprioCofre(t *testing.T) {
	url, tokenAdmin, tokenMembro := adminEMembroConvidado(t)

	vaultURL, notaURL := cofreComNota(t, url, tokenMembro, "Do membro", "do membro")

	assertColegaPassaALer(t, url, vaultURL, notaURL, tokenMembro, tokenAdmin, "admin@markupp.dev", "do membro")
}

func TestIntegration_ContaQueEntraNoTenant_GanhaOCofreDefault(t *testing.T) {
	url, _, tokenMembro := adminEMembroConvidado(t)

	resp := getWithAuth(t, url+"/v1/vaults", tokenMembro)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var cofres []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&cofres))
	require.Len(t, cofres, 1)
	assert.Equal(t, "Default", cofres[0]["name"])
}
