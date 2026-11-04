package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minhasNotificacoes lê GET /v1/me/notifications da conta do token.
func minhasNotificacoes(t *testing.T, url, token string) map[string]any {
	t.Helper()
	status, feed := sendJSON(t, http.MethodGet, url+"/v1/me/notifications", token, "")
	require.Equal(t, http.StatusOK, status)
	return feed
}

// conviteDeOutroTenant monta ana, no tenant padrão, e bia, owner do próprio tenant, que convida ana como viewer;
// devolve os tokens das duas.
func conviteDeOutroTenant(t *testing.T, url string) (tokenAna, tokenBia string) {
	t.Helper()
	_, sessaoA := sendJSON(t, http.MethodPost, url+"/auth/login", "", `{"email":"ana@x.dev","password":"senha-ana"}`)
	tokenAna = sessaoA["access_token"].(string)
	_, convite := sendJSON(t, http.MethodPost, url+"/users/invites", tokenAna, `{"email":"bia@x.dev","role":"member"}`)
	_, sessaoB := sendJSON(t, http.MethodPost, url+"/auth/invites/accept", "", `{"token":"`+convite["token"].(string)+`","password":"senha-bia"}`)
	tokenBia = sessaoB["access_token"].(string)
	_, tenantB := sendJSON(t, http.MethodPost, url+"/tenants", tokenBia, `{"name":"Estudio da Bia"}`)
	status, _ := sendAs(t, http.MethodPost, url+"/users/invites", tokenBia, tenantB["id"].(string), `{"email":"ana@x.dev","role":"viewer"}`)
	require.Equal(t, http.StatusCreated, status)
	return tokenAna, tokenBia
}

func TestIntegration_Notificacoes_ConviteDeOutroTenantApareceAteSerAceito(t *testing.T) {
	server, _ := setupAuthIntegrationServer(t)
	tokenAna, tokenBia := conviteDeOutroTenant(t, server.URL)

	feed := minhasNotificacoes(t, server.URL, tokenAna)
	require.Equal(t, float64(1), feed["unread"])
	item := feed["items"].([]any)[0].(map[string]any)
	assert.Equal(t, "invite", item["kind"])
	assert.Equal(t, "Estudio da Bia", item["tenant_name"])
	assert.Equal(t, "viewer", item["role"])
	assert.Equal(t, float64(0), minhasNotificacoes(t, server.URL, tokenBia)["unread"], "quem convida nao e notificado")

	status, _ := sendJSON(t, http.MethodPost, server.URL+"/me/invites/"+item["id"].(string)+"/accept", tokenAna, "")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, map[string]any{"items": []any{}, "unread": float64(0)}, minhasNotificacoes(t, server.URL, tokenAna))
}

func TestIntegration_Notificacoes_ConviteVencidoSome(t *testing.T) {
	server, pool := setupAuthIntegrationServer(t)
	tokenAna, _ := conviteDeOutroTenant(t, server.URL)
	_, err := pool.Exec(context.Background(), "UPDATE user_invites SET expires_at = now() - interval '1 minute' WHERE email = 'ana@x.dev'")
	require.NoError(t, err)

	assert.Equal(t, float64(0), minhasNotificacoes(t, server.URL, tokenAna)["unread"])
}
