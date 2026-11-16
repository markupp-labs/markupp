package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

// fakeInstanceEmbedding devolve view em toda chamada e guarda o último pedido.
type fakeInstanceEmbedding struct {
	view    instance.EmbeddingView
	update  *instance.EmbeddingUpdate
	err     error
	reached bool
}

func (f *fakeInstanceEmbedding) Settings(context.Context) (instance.EmbeddingView, error) {
	f.reached = true
	return f.view, f.err
}

func (f *fakeInstanceEmbedding) Update(_ context.Context, u instance.EmbeddingUpdate) (instance.EmbeddingView, error) {
	f.reached, f.update = true, &u
	return f.view, f.err
}

const emailDoAdministrador = "ana@exemplo.org"

var administradorDaInstancia = auth.User{ID: "u1", Email: "Ana@Exemplo.org", EmailVerified: true}

func instanceRequest(user auth.User, svc api.InstanceEmbeddingService, method, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Auth: &fakeAuthService{accountUser: user}, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: user.ID}},
		InstanceAdmins: instance.Admins{emailDoAdministrador}, InstanceEmbedding: svc,
	})
	req := httptest.NewRequest(method, "/v1/instance/embedding", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestGetInstanceEmbedding_Administrador_DevolveAEscolhaSemAChave(t *testing.T) {
	svc := &fakeInstanceEmbedding{view: instance.EmbeddingView{Provider: "openai-compat", URL: "https://x/v1", Model: "m", APIKeySet: true}}

	rec := instanceRequest(administradorDaInstancia, svc, http.MethodGet, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"provider":"openai-compat","url":"https://x/v1","model":"m","api_key_set":true,
		"locked":false,"host_provider":"","model_status":null}`, rec.Body.String())
}

func TestInstanceEmbedding_ContaForaDaLista_Retorna403SemChegarAoServico(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		svc := &fakeInstanceEmbedding{}

		rec := instanceRequest(auth.User{ID: "u2", Email: "bia@exemplo.org", EmailVerified: true}, svc, method, `{"provider":"local"}`)

		assert.Equal(t, http.StatusForbidden, rec.Code, method)
		assert.Contains(t, rec.Body.String(), "instance_admin_required")
		assert.Contains(t, rec.Body.String(), "bia@exemplo.org")
		assert.False(t, svc.reached, method)
	}
}

func TestInstanceEmbedding_EmailNaoVerificado_Retorna403(t *testing.T) {
	rec := instanceRequest(auth.User{ID: "u1", Email: emailDoAdministrador}, &fakeInstanceEmbedding{}, http.MethodGet, "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPutInstanceEmbedding_ComChave_RepassaOPedido(t *testing.T) {
	svc := &fakeInstanceEmbedding{view: instance.EmbeddingView{Provider: "local"}}

	rec := instanceRequest(administradorDaInstancia, svc, http.MethodPut, `{"provider":"openai-compat","url":"https://x/v1","model":"m","api_key":"sk"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, svc.update.APIKey)
	assert.Equal(t, "sk", *svc.update.APIKey)
	assert.Equal(t, "openai-compat", svc.update.Provider)
}

func TestPutInstanceEmbedding_SemChave_MantemAChave(t *testing.T) {
	svc := &fakeInstanceEmbedding{}

	rec := instanceRequest(administradorDaInstancia, svc, http.MethodPut, `{"provider":"local"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Nil(t, svc.update.APIKey)
}

func TestPutInstanceEmbedding_ErrosDoServico_ViramStatusDoContrato(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: provider %q", instance.ErrInvalidEmbedding, "x"), http.StatusBadRequest, "invalid_request"},
		{instance.ErrHostLocked, http.StatusConflict, "host_locked"},
	}
	for _, caso := range casos {
		rec := instanceRequest(administradorDaInstancia, &fakeInstanceEmbedding{err: caso.err}, http.MethodPut, `{"provider":"x"}`)

		assert.Equal(t, caso.status, rec.Code, caso.code)
		assert.Contains(t, rec.Body.String(), caso.code)
	}
}

func TestPutInstanceEmbedding_CorpoInvalido_Retorna400(t *testing.T) {
	rec := instanceRequest(administradorDaInstancia, &fakeInstanceEmbedding{}, http.MethodPut, `nao e json`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAuthHandler_Me_DizSeAContaAdministraAInstancia(t *testing.T) {
	casos := map[string]struct {
		user     auth.User
		esperado bool
	}{
		"na lista e verificado":  {administradorDaInstancia, true},
		"na lista sem verificar": {auth.User{ID: "u1", Email: emailDoAdministrador}, false},
		"fora da lista":          {auth.User{ID: "u2", Email: "bia@exemplo.org", EmailVerified: true}, false},
	}
	for nome, caso := range casos {
		rec := instanceMeRequest(caso.user)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), nome)
		assert.Equal(t, caso.esperado, resp["instance_admin"], nome)
	}
}

func instanceMeRequest(user auth.User) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Auth: &fakeAuthService{accountUser: user}, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: user.ID}},
		InstanceAdmins: instance.Admins{emailDoAdministrador},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
