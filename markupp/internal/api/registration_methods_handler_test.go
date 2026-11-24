package api_test

import (
	"bytes"
	"context"
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

// fakeRegistration responde com os métodos dados e guarda o último pedido de troca.
type fakeRegistration struct {
	views   []instance.RegistrationMethodView
	enabled bool
	err     error
	setID   string
	setOpen *bool
	reached bool
}

func (f *fakeRegistration) Methods(context.Context) ([]instance.RegistrationMethodView, error) {
	f.reached = true
	return f.views, f.err
}

func (f *fakeRegistration) SetOpen(_ context.Context, id string, open bool) (instance.RegistrationMethodView, error) {
	f.reached, f.setID, f.setOpen = true, id, &open
	return instance.RegistrationMethodView{ID: id, Name: id, Open: open}, f.err
}

func (f *fakeRegistration) RegistrationOpen(_ context.Context, method string) (bool, error) {
	for _, v := range f.views {
		if v.ID == method {
			return v.Open, f.err
		}
	}
	return false, f.err
}

func (f *fakeRegistration) RegistrationEnabled() bool { return f.enabled }

func registrationRequest(user auth.User, svc *fakeRegistration, method, path, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Auth: &fakeAuthService{accountUser: user}, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: user.ID}},
		InstanceAdmins: instance.Admins{emailDoAdministrador}, Registration: svc,
		ExternalLogin: &fakeExternalLogin{}, Probe: &fakeProbe{},
	})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func metodosAbertos() *fakeRegistration {
	return &fakeRegistration{enabled: true, views: []instance.RegistrationMethodView{
		{ID: "local", Name: "Email and password", Open: false},
		{ID: "github", Name: "GitHub", Open: true, Locked: true},
	}}
}

func TestListRegistrationMethods_Administrador_DevolveCadaMetodo(t *testing.T) {
	rec := registrationRequest(administradorDaInstancia, metodosAbertos(), http.MethodGet, "/v1/instance/registration-methods", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"id":"local","name":"Email and password","open":false,"locked":false},
		{"id":"github","name":"GitHub","open":true,"locked":true}]`, rec.Body.String())
}

func TestRegistrationMethods_ContaForaDaLista_Retorna403SemChegarAoServico(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		svc := metodosAbertos()
		path := "/v1/instance/registration-methods"
		if method == http.MethodPatch {
			path += "/local"
		}

		rec := registrationRequest(auth.User{ID: "u2", Email: "bia@exemplo.org", EmailVerified: true}, svc, method, path, `{"open":true}`)

		assert.Equal(t, http.StatusForbidden, rec.Code, method)
		assert.Contains(t, rec.Body.String(), "instance_admin_required")
		assert.False(t, svc.reached, method)
	}
}

func TestPatchRegistrationMethod_Administrador_GravaEDevolve(t *testing.T) {
	svc := metodosAbertos()

	rec := registrationRequest(administradorDaInstancia, svc, http.MethodPatch, "/v1/instance/registration-methods/local", `{"open":true}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "local", svc.setID)
	require.NotNil(t, svc.setOpen)
	assert.True(t, *svc.setOpen)
	assert.JSONEq(t, `{"id":"local","name":"local","open":true,"locked":false}`, rec.Body.String())
}

func TestPatchRegistrationMethod_SemOpen_Retorna400SemChegarAoServico(t *testing.T) {
	svc := metodosAbertos()

	rec := registrationRequest(administradorDaInstancia, svc, http.MethodPatch, "/v1/instance/registration-methods/local", `{}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
	assert.False(t, svc.reached)
}

func TestPatchRegistrationMethod_ErrosDoServico_ViramStatus(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: %q", instance.ErrRegistrationLocked, "github"), http.StatusConflict, "host_locked"},
		{fmt.Errorf("%w: %q", instance.ErrUnknownRegistrationMethod, "gitlab"), http.StatusNotFound, "not_found"},
	}
	for _, caso := range casos {
		svc := metodosAbertos()
		svc.err = caso.err

		rec := registrationRequest(administradorDaInstancia, svc, http.MethodPatch, "/v1/instance/registration-methods/x", `{"open":true}`)

		assert.Equal(t, caso.status, rec.Code, caso.code)
		assert.Contains(t, rec.Body.String(), caso.code)
	}
}

func TestAccountPolicy_SemConta_DizSeOCadastroPorSenhaEstaAberto(t *testing.T) {
	svc := metodosAbertos()
	svc.views[0].Open = true
	router := api.NewRouter(api.RouterDeps{Auth: &fakeAuthService{}, Tokens: &fakeTokenValidator{}, Registration: svc, Probe: &fakeProbe{}})
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/policy", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"registration_enabled":true,"password_registration_open":true}`, rec.Body.String())
}

func TestAccountPolicy_CadastroGeralDesligado_DizQueNadaCadastra(t *testing.T) {
	rec := registrationRequest(auth.User{}, &fakeRegistration{}, http.MethodGet, "/v1/accounts/policy", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"registration_enabled":false,"password_registration_open":false}`, rec.Body.String())
}

func TestLoginProviders_ComCadastroPorMetodo_DizSeCadaProvedorCadastra(t *testing.T) {
	rec := registrationRequest(auth.User{}, metodosAbertos(), http.MethodGet, "/v1/login-providers", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"id":"github","name":"GitHub","registration_open":true}]`, rec.Body.String())
}
