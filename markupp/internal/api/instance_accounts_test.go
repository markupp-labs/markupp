package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

type fakeInstanceAccounts struct {
	accounts []auth.AccountSummary
	detail   auth.AccountDetail
	err      error

	query           auth.AccountQuery
	gotActor, gotID string
	gotBlocked      *bool
	deleted         bool
}

func (f *fakeInstanceAccounts) List(_ context.Context, q auth.AccountQuery) ([]auth.AccountSummary, error) {
	f.query = q
	return f.accounts, f.err
}

func (f *fakeInstanceAccounts) Get(_ context.Context, id string) (auth.AccountDetail, error) {
	f.gotID = id
	return f.detail, f.err
}

func (f *fakeInstanceAccounts) SetBlocked(_ context.Context, actorID, id string, blocked bool) error {
	f.gotActor, f.gotID, f.gotBlocked = actorID, id, &blocked
	return f.err
}

func (f *fakeInstanceAccounts) Delete(_ context.Context, actorID, id string) error {
	f.gotActor, f.gotID, f.deleted = actorID, id, true
	return f.err
}

type fakeAccountAuthorizations struct {
	views                         []oauth.AuthorizationView
	err                           error
	gotActor, gotUser, gotRevoked string
}

func (f *fakeAccountAuthorizations) ListAuthorizations(_ context.Context, userID string) ([]oauth.AuthorizationView, error) {
	f.gotUser = userID
	return f.views, f.err
}

func (f *fakeAccountAuthorizations) RevokeAuthorizationAs(_ context.Context, actorID, userID, authorizationID string) error {
	f.gotActor, f.gotUser, f.gotRevoked = actorID, userID, authorizationID
	return f.err
}

type instanceAccountsFixture struct {
	accounts *fakeInstanceAccounts
	sessions *fakeSessionService
	apps     *fakeAccountAuthorizations
	user     auth.User
}

func newInstanceAccountsFixture() *instanceAccountsFixture {
	return &instanceAccountsFixture{accounts: &fakeInstanceAccounts{}, sessions: &fakeSessionService{},
		apps: &fakeAccountAuthorizations{}, user: administradorDaInstancia}
}

func (f *instanceAccountsFixture) do(method, path, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Auth: &fakeAuthService{accountUser: f.user}, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: f.user.ID}},
		InstanceAdmins: instance.Admins{emailDoAdministrador}, InstanceAccounts: f.accounts,
		Sessions: f.sessions, AccountAuthorizations: f.apps,
	})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

var criadaEm = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func TestInstanceAccounts_Listar_PassaOFiltroEDevolveAsContas(t *testing.T) {
	f := newInstanceAccountsFixture()
	f.accounts.accounts = []auth.AccountSummary{{ID: "u2", Email: "bia@exemplo.org", EmailVerified: true,
		LoginMethods: []string{"github"}, CreatedAt: criadaEm}}

	rec := f.do(http.MethodGet, "/v1/instance/accounts?q=bia&status=blocked&offset=10&limit=20", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"id":"u2","email":"bia@exemplo.org","email_verified":true,"login_methods":["github"],
		"created_at":"2026-11-01T12:00:00Z","last_access_at":null,"blocked":false}]`, rec.Body.String())
	assert.Equal(t, auth.AccountQuery{Email: "bia", Status: "blocked", Offset: 10, Limit: 20}, f.accounts.query)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestInstanceAccounts_Listar_VaziaEArray(t *testing.T) {
	rec := newInstanceAccountsFixture().do(http.MethodGet, "/v1/instance/accounts", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestInstanceAccounts_Listar_LimiteForaDaFaixa_400(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=201", "offset=-1", "limit=x"} {
		rec := newInstanceAccountsFixture().do(http.MethodGet, "/v1/instance/accounts?"+query, "")

		assert.Equal(t, http.StatusBadRequest, rec.Code, query)
		assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`, query)
	}
}

func TestInstanceAccounts_Listar_StatusInvalido_400(t *testing.T) {
	f := newInstanceAccountsFixture()
	f.accounts.err = fmt.Errorf("%w: status %q", auth.ErrInvalidAccountQuery, "x")

	rec := f.do(http.MethodGet, "/v1/instance/accounts?status=x", "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
}

func TestInstanceAccounts_Detalhe_TrazOQueSomeComAConta(t *testing.T) {
	f := newInstanceAccountsFixture()
	f.accounts.detail = auth.AccountDetail{AccountSummary: auth.AccountSummary{ID: "u2", Email: "bia@exemplo.org",
		LoginMethods: []string{}, CreatedAt: criadaEm}, SoleOwnerTenants: []auth.OwnedTenant{{ID: "t1", Name: "Casa", Notes: 3}}}

	rec := f.do(http.MethodGet, "/v1/instance/accounts/u2", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"u2","email":"bia@exemplo.org","email_verified":false,"login_methods":[],
		"created_at":"2026-11-01T12:00:00Z","last_access_at":null,"blocked":false,
		"sole_owner_tenants":[{"id":"t1","name":"Casa","notes":3}]}`, rec.Body.String())
	assert.Equal(t, "u2", f.accounts.gotID)
}

func TestInstanceAccounts_Inexistente_404(t *testing.T) {
	f := newInstanceAccountsFixture()
	f.accounts.err = fmt.Errorf("get: %w", auth.ErrUserNotFound)

	for _, c := range []struct{ method, body string }{{http.MethodGet, ""}, {http.MethodPatch, `{"blocked":true}`}, {http.MethodDelete, ""}} {
		rec := f.do(c.method, "/v1/instance/accounts/u9", c.body)

		assert.Equal(t, http.StatusNotFound, rec.Code, c.method)
		assert.Contains(t, rec.Body.String(), `"error":"account_not_found"`, c.method)
	}
}

func TestInstanceAccounts_Bloquear_DevolveAContaAtualizada(t *testing.T) {
	f := newInstanceAccountsFixture()
	f.accounts.detail = auth.AccountDetail{AccountSummary: auth.AccountSummary{ID: "u2", Blocked: true, LoginMethods: []string{}},
		SoleOwnerTenants: []auth.OwnedTenant{}}

	rec := f.do(http.MethodPatch, "/v1/instance/accounts/u2", `{"blocked":true}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, f.accounts.gotBlocked)
	assert.True(t, *f.accounts.gotBlocked)
	assert.Equal(t, "u1", f.accounts.gotActor)
	assert.Contains(t, rec.Body.String(), `"blocked":true`)
}

func TestInstanceAccounts_PatchSemBlocked_400(t *testing.T) {
	f := newInstanceAccountsFixture()

	rec := f.do(http.MethodPatch, "/v1/instance/accounts/u2", `{}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Nil(t, f.accounts.gotBlocked)
}

func TestInstanceAccounts_APropria_409(t *testing.T) {
	f := newInstanceAccountsFixture()
	f.accounts.err = auth.ErrCannotTargetSelf

	rec := f.do(http.MethodDelete, "/v1/instance/accounts/u1", "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"cannot_target_self"`)
}

func TestInstanceAccounts_Apagar_204(t *testing.T) {
	f := newInstanceAccountsFixture()

	rec := f.do(http.MethodDelete, "/v1/instance/accounts/u2", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, f.accounts.deleted)
	assert.Equal(t, "u1", f.accounts.gotActor)
}

func TestInstanceAccounts_SessoesDaConta_ListaSemMarcarAtual(t *testing.T) {
	f := newInstanceAccountsFixture()

	rec := f.do(http.MethodGet, "/v1/instance/accounts/u2/sessions", "")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "u2", f.sessions.gotOwner)
	assert.Empty(t, f.sessions.gotCurrent, "a sessão do administrador não é da conta listada")
}

func TestInstanceAccounts_EncerrarSessaoDaConta_204ComOAdministradorComoAutor(t *testing.T) {
	f := newInstanceAccountsFixture()

	rec := f.do(http.MethodDelete, "/v1/instance/accounts/u2/sessions/s9", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "u1", f.sessions.gotActor)
	assert.Equal(t, "u2", f.sessions.gotOwner)
	assert.Equal(t, "s9", f.sessions.gotSessionID)
}

func TestInstanceAccounts_AppsDaConta_ListaERevoga(t *testing.T) {
	f := newInstanceAccountsFixture()

	list := f.do(http.MethodGet, "/v1/instance/accounts/u2/authorizations", "")
	revoke := f.do(http.MethodDelete, "/v1/instance/accounts/u2/authorizations/a9", "")

	require.Equal(t, http.StatusOK, list.Code)
	assert.JSONEq(t, `[]`, list.Body.String())
	assert.Equal(t, http.StatusNoContent, revoke.Code)
	assert.Equal(t, "u1", f.apps.gotActor)
	assert.Equal(t, "u2", f.apps.gotUser)
	assert.Equal(t, "a9", f.apps.gotRevoked)
}

func TestInstanceAccounts_RevogarAppInexistente_404(t *testing.T) {
	f := newInstanceAccountsFixture()
	f.apps.err = oauth.ErrAuthorizationNotFound

	rec := f.do(http.MethodDelete, "/v1/instance/accounts/u2/authorizations/a9", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"authorization_not_found"`)
}

func TestInstanceAccounts_ContaForaDaListaDeAdministradores_403(t *testing.T) {
	f := newInstanceAccountsFixture()
	f.user = auth.User{ID: "u2", Email: "bia@exemplo.org", EmailVerified: true}

	rec := f.do(http.MethodGet, "/v1/instance/accounts", "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"instance_admin_required"`)
}
