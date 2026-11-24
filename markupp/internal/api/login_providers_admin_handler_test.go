package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

// fakeInstanceLoginProviders devolve o que o teste põe e guarda o último pedido.
type fakeInstanceLoginProviders struct {
	list    instance.LoginProvidersView
	view    instance.LoginProviderView
	input   *instance.LoginProviderInput
	deleted string
	removed int
	err     error
	reached bool
}

func (f *fakeInstanceLoginProviders) List(context.Context) (instance.LoginProvidersView, error) {
	f.reached = true
	return f.list, f.err
}

func (f *fakeInstanceLoginProviders) Create(_ context.Context, in instance.LoginProviderInput) (instance.LoginProviderView, error) {
	f.reached, f.input = true, &in
	return f.view, f.err
}

func (f *fakeInstanceLoginProviders) Update(_ context.Context, in instance.LoginProviderInput) (instance.LoginProviderView, error) {
	f.reached, f.input = true, &in
	return f.view, f.err
}

func (f *fakeInstanceLoginProviders) Delete(_ context.Context, id string) (int, error) {
	f.reached, f.deleted = true, id
	return f.removed, f.err
}

func loginProvidersRequest(user auth.User, svc api.InstanceLoginProviderService, method, path, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Auth: &fakeAuthService{accountUser: user}, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: user.ID}},
		InstanceAdmins: instance.Admins{emailDoAdministrador}, InstanceLoginProviders: svc,
	})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestListInstanceLoginProviders_Administrador_DevolveSemOSegredo(t *testing.T) {
	svc := &fakeInstanceLoginProviders{list: instance.LoginProvidersView{Providers: []instance.LoginProviderView{{
		ID: "github", Name: "GitHub", ClientID: "id", SecretSet: true, Enabled: true,
		CallbackURL: "https://painel/auth/github/callback", LinkedAccounts: 2,
	}}}}

	rec := loginProvidersRequest(administradorDaInstancia, svc, http.MethodGet, "/v1/instance/login-providers", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"locked":false,"providers":[{"id":"github","name":"GitHub","issuer":"","client_id":"id",
		"client_secret_set":true,"enabled":true,"locked":false,"callback_url":"https://painel/auth/github/callback","linked_accounts":2}]}`, rec.Body.String())
}

func TestCreateInstanceLoginProvider_Administrador_Retorna201ComOPedido(t *testing.T) {
	svc := &fakeInstanceLoginProviders{view: instance.LoginProviderView{ID: "google"}}

	rec := loginProvidersRequest(administradorDaInstancia, svc, http.MethodPost, "/v1/instance/login-providers",
		`{"id":"google","name":"Google","issuer":"https://accounts.google.com","client_id":"id","client_secret":"s","enabled":true}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.NotNil(t, svc.input)
	assert.Equal(t, "https://accounts.google.com", svc.input.Issuer)
	assert.Equal(t, "s", *svc.input.ClientSecret)
	assert.True(t, svc.input.Enabled)
}

func TestUpdateInstanceLoginProvider_SemSegredo_MandaNilEOIdDaRota(t *testing.T) {
	svc := &fakeInstanceLoginProviders{}

	rec := loginProvidersRequest(administradorDaInstancia, svc, http.MethodPut, "/v1/instance/login-providers/github",
		`{"id":"outro","name":"GitHub","client_id":"id","enabled":false}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "github", svc.input.ID, "o id não muda depois de criado")
	assert.Nil(t, svc.input.ClientSecret)
	assert.False(t, svc.input.Enabled)
}

func TestDeleteInstanceLoginProvider_DizQuantasContasPerderamOVinculo(t *testing.T) {
	svc := &fakeInstanceLoginProviders{removed: 3}

	rec := loginProvidersRequest(administradorDaInstancia, svc, http.MethodDelete, "/v1/instance/login-providers/google", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "google", svc.deleted)
	assert.JSONEq(t, `{"removed_links":3}`, rec.Body.String())
}

func TestInstanceLoginProviders_ContaForaDaLista_Retorna403SemChegarAoServico(t *testing.T) {
	svc := &fakeInstanceLoginProviders{}

	rec := loginProvidersRequest(auth.User{ID: "u2", Email: "bia@exemplo.org", EmailVerified: true}, svc, http.MethodGet, "/v1/instance/login-providers", "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, svc.reached)
}

func TestInstanceLoginProviders_ErrosDoServico_ViramCodigos(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{instance.ErrInvalidLoginProvider, http.StatusUnprocessableEntity, "invalid_login_provider"},
		{instance.ErrLoginProvidersLocked, http.StatusConflict, "host_locked"},
		{instance.ErrLoginProviderIssuerLocked, http.StatusConflict, "login_provider_issuer_locked"},
		{instance.ErrLoginProviderExists, http.StatusConflict, "login_provider_exists"},
		{instance.ErrLoginProviderNotFound, http.StatusNotFound, "login_provider_not_found"},
	}
	for _, c := range casos {
		svc := &fakeInstanceLoginProviders{err: c.err}

		rec := loginProvidersRequest(administradorDaInstancia, svc, http.MethodPut, "/v1/instance/login-providers/github", `{"client_id":"id"}`)

		assert.Equal(t, c.status, rec.Code, c.code)
		assert.Contains(t, rec.Body.String(), c.code)
	}
}
