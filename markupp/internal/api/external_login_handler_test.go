package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// fakeExternalLogin faz o papel do serviço de login externo: devolve o erro escolhido e
// guarda o que recebeu.
type fakeExternalLogin struct {
	err        error
	callback   auth.ExternalCallback
	userID     string
	identityID string
	identities []auth.UserIdentity
}

func (f *fakeExternalLogin) Providers() []auth.LoginProviderSummary {
	return []auth.LoginProviderSummary{{ID: "github", Name: "GitHub"}}
}

func (f *fakeExternalLogin) Begin(_ context.Context, providerID string) (loginprovider.Attempt, error) {
	return loginprovider.Attempt{AuthorizationURL: "https://" + providerID + "/authorize", State: "s", CodeVerifier: "v", Nonce: "n"}, f.err
}

func (f *fakeExternalLogin) Login(_ context.Context, cb auth.ExternalCallback) (auth.TokenPair, error) {
	f.callback = cb
	return auth.TokenPair{AccessToken: "acesso", RefreshToken: "refresh", ExpiresIn: 900}, f.err
}

func (f *fakeExternalLogin) Register(_ context.Context, cb auth.ExternalCallback) (auth.TokenPair, error) {
	f.callback = cb
	return auth.TokenPair{AccessToken: "acesso-novo", RefreshToken: "refresh", ExpiresIn: 900}, f.err
}

func (f *fakeExternalLogin) Link(_ context.Context, userID string, cb auth.ExternalCallback) (auth.UserIdentity, error) {
	f.userID, f.callback = userID, cb
	return auth.UserIdentity{ID: "ident-1", Provider: cb.Provider, CreatedAt: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)}, f.err
}

func (f *fakeExternalLogin) ListIdentities(_ context.Context, userID string) ([]auth.UserIdentity, error) {
	f.userID = userID
	return f.identities, f.err
}

func (f *fakeExternalLogin) Unlink(_ context.Context, userID, identityID string) error {
	f.userID, f.identityID = userID, identityID
	return f.err
}

func externalRouter(svc *fakeExternalLogin, claims auth.Claims) http.Handler {
	return api.NewRouter(api.RouterDeps{ExternalLogin: svc, Tokens: &fakeTokenValidator{claims: claims}, Probe: &fakeProbe{}})
}

func doExternal(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

const callbackBody = `{"provider":"github","code":"c","code_verifier":"v","nonce":"n"}`

var loginToken = auth.Claims{Subject: "user-1"}

func TestExternalLogin_ListaProvedores_SemLogin(t *testing.T) {
	rec := doExternal(t, externalRouter(&fakeExternalLogin{}, loginToken), http.MethodGet, "/v1/login-providers", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[{"id":"github","name":"GitHub"}]`, rec.Body.String())
}

func TestExternalLogin_Attempt_DevolveURLEValoresDaTentativa(t *testing.T) {
	rec := doExternal(t, externalRouter(&fakeExternalLogin{}, loginToken), http.MethodPost, "/v1/login-providers/github/attempts", "")

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"authorization_url":"https://github/authorize","state":"s","code_verifier":"v","nonce":"n"}`, rec.Body.String())
}

func TestExternalLogin_Attempt_ProvedorDesconhecido_404(t *testing.T) {
	svc := &fakeExternalLogin{err: auth.ErrUnknownLoginProvider}

	rec := doExternal(t, externalRouter(svc, loginToken), http.MethodPost, "/v1/login-providers/gitlab/attempts", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestExternalLogin_Login_DevolveASessao(t *testing.T) {
	svc := &fakeExternalLogin{}

	rec := doExternal(t, externalRouter(svc, loginToken), http.MethodPost, "/v1/auth/external-login", callbackBody)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, auth.ExternalCallback{Provider: "github", Code: "c", CodeVerifier: "v", Nonce: "n"}, svc.callback)
	var pair map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pair))
	assert.Equal(t, "acesso", pair["access_token"])
}

func TestExternalLogin_Register_DevolveASessaoCom201(t *testing.T) {
	rec := doExternal(t, externalRouter(&fakeExternalLogin{}, loginToken), http.MethodPost, "/v1/accounts/external", callbackBody)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestExternalLogin_ErrosViramRespostas(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{auth.ErrIdentityNotLinked, http.StatusUnauthorized, "identity_not_linked"},
		{loginprovider.ErrLoginRejected, http.StatusUnauthorized, "login_rejected"},
		{loginprovider.ErrProviderUnavailable, http.StatusBadGateway, "provider_unavailable"},
		{auth.ErrUnknownLoginProvider, http.StatusBadRequest, "unknown_login_provider"},
		{auth.ErrRegistrationDisabled, http.StatusForbidden, "registration_disabled"},
		{auth.ErrIdentityAlreadyLinked, http.StatusConflict, "identity_already_linked"},
		{auth.ErrUserAlreadyExists, http.StatusConflict, "already_exists"},
		{auth.ErrProviderEmailMissing, http.StatusUnprocessableEntity, "provider_email_missing"},
		{errors.New("banco fora"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range casos {
		t.Run(c.code, func(t *testing.T) {
			svc := &fakeExternalLogin{err: c.err}

			rec := doExternal(t, externalRouter(svc, loginToken), http.MethodPost, "/v1/accounts/external", callbackBody)

			assert.Equal(t, c.status, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error":"`+c.code+`"`)
		})
	}
}

func TestExternalLogin_CorpoInvalido_400(t *testing.T) {
	rec := doExternal(t, externalRouter(&fakeExternalLogin{}, loginToken), http.MethodPost, "/v1/auth/external-login", "{")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestExternalLogin_ListIdentities_DaContaLogada(t *testing.T) {
	svc := &fakeExternalLogin{identities: []auth.UserIdentity{
		{ID: "ident-1", Provider: "github", CreatedAt: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)},
	}}

	rec := doExternal(t, externalRouter(svc, loginToken), http.MethodGet, "/v1/me/identities", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-1", svc.userID)
	assert.JSONEq(t, `[{"id":"ident-1","provider":"github","created_at":"2026-11-12T10:00:00Z"}]`, rec.Body.String())
}

func TestExternalLogin_ListIdentities_SemNenhuma_ListaVazia(t *testing.T) {
	rec := doExternal(t, externalRouter(&fakeExternalLogin{}, loginToken), http.MethodGet, "/v1/me/identities", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestExternalLogin_Link_VinculaAContaLogada(t *testing.T) {
	svc := &fakeExternalLogin{}

	rec := doExternal(t, externalRouter(svc, loginToken), http.MethodPost, "/v1/me/identities", callbackBody)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "user-1", svc.userID)
	assert.JSONEq(t, `{"id":"ident-1","provider":"github","created_at":"2026-11-12T10:00:00Z"}`, rec.Body.String())
}

func TestExternalLogin_Unlink_204(t *testing.T) {
	svc := &fakeExternalLogin{}

	rec := doExternal(t, externalRouter(svc, loginToken), http.MethodDelete, "/v1/me/identities/ident-1", "")

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "ident-1", svc.identityID)
}

func TestExternalLogin_Unlink_Erros(t *testing.T) {
	casos := map[error]int{auth.ErrIdentityNotFound: http.StatusNotFound, auth.ErrLastIdentity: http.StatusConflict}
	for err, status := range casos {
		rec := doExternal(t, externalRouter(&fakeExternalLogin{err: err}, loginToken), http.MethodDelete, "/v1/me/identities/x", "")

		assert.Equal(t, status, rec.Code, err.Error())
	}
}

func TestExternalLogin_IdentidadesComTokenOAuth_Recusa(t *testing.T) {
	appToken := auth.Claims{Subject: "user-1", Scope: "notas:leitura notas:escrita administracao"}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := doExternal(t, externalRouter(&fakeExternalLogin{}, appToken), method, "/v1/me/identities", callbackBody)

		assert.Equal(t, http.StatusForbidden, rec.Code, "so o token de login mexe nas formas de entrar")
	}
}

func TestExternalLogin_SemAliasSemPrefixo(t *testing.T) {
	rec := doExternal(t, externalRouter(&fakeExternalLogin{}, loginToken), http.MethodGet, "/login-providers", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
