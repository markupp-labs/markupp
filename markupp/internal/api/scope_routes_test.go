package api_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

type scopedRoute struct{ method, path string }

var (
	notesReadRoutes = []scopedRoute{
		{http.MethodGet, "/v1/notes"}, {http.MethodGet, "/v1/notes/search?query=x"}, {http.MethodGet, "/v1/notes/n1"},
	}
	notesWriteRoutes = []scopedRoute{
		{http.MethodPost, "/v1/notes"}, {http.MethodPut, "/v1/notes/n1"}, {http.MethodDelete, "/v1/notes/n1"},
	}
	adminScopeRoutes = []scopedRoute{
		{http.MethodGet, "/v1/users"}, {http.MethodGet, "/v1/users/invites"}, {http.MethodPost, "/v1/users/invites"},
		{http.MethodPatch, "/v1/users/u2"}, {http.MethodDelete, "/v1/users/u2"}, {http.MethodDelete, "/v1/users/invites/c1"},
		{http.MethodPost, "/v1/tenants"}, {http.MethodGet, "/v1/me/invites"},
		{http.MethodPost, "/v1/me/invites/c1/accept"}, {http.MethodPost, "/v1/me/invites/c1/decline"},
	}
	anyScopeRoutes  = []scopedRoute{{http.MethodGet, "/v1/auth/me"}, {http.MethodGet, "/v1/me/tenants"}}
	loginOnlyRoutes = []scopedRoute{
		{http.MethodGet, "/v1/oauth/requests/p1"}, {http.MethodPost, "/v1/oauth/requests/p1/approve"},
		{http.MethodPost, "/v1/oauth/requests/p1/deny"}, {http.MethodGet, "/v1/me/authorizations"},
		{http.MethodDelete, "/v1/me/authorizations/a1"}, {http.MethodPost, "/v1/auth/email/verification"},
		{http.MethodPost, "/v1/auth/email/verify"},
	}
)

// serveScoped manda a rota a um roteador com todos os serviços, autenticada por um
// token com a claim scope dada (vazia é token de login) e com o papel owner no tenant.
func serveScoped(route scopedRoute, scope string) *httptest.ResponseRecorder {
	return serveScopedWith(route, scope, &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: "owner"}})
}

func serveScopedWith(route scopedRoute, scope string, tenants api.TenantResolver) *httptest.ResponseRecorder {
	router := routerWithAllServices(&fakeTokenValidator{claims: auth.Claims{Subject: "u1", Scope: scope}}, tenants)
	// Corpo JSON vazio para os handlers que leem um; os de GET e DELETE o ignoram.
	req := httptest.NewRequest(route.method, route.path, strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// routerWithAllServices monta o roteador com todos os serviços falsos ligados.
func routerWithAllServices(tokens api.TokenValidator, tenants api.TenantResolver) chi.Router {
	return api.NewRouter(api.RouterDeps{
		Notes: &fakeService{}, Graph: &fakeNoteGraph{}, Auth: &fakeAuthService{}, Admin: &fakeAdminService{},
		AccountEmail: &fakeAccountEmailService{}, OAuth: &fakeOAuthService{},
		OAuthMetadata: &api.OAuthMetadata{Issuer: issuerDeTeste}, Probe: &fakeProbe{},
		Tokens: tokens, Tenants: tenants,
	})
}

func assertRoutesPass(t *testing.T, routes []scopedRoute, scope string) {
	t.Helper()
	for _, route := range routes {
		rec := serveScoped(route, scope)

		assert.True(t, rec.Code >= 200 && rec.Code < 300, "%s %s com scope %q respondeu %d: %s", route.method, route.path, scope, rec.Code, rec.Body.String())
	}
}

func assertRoutesNeedScope(t *testing.T, routes []scopedRoute, scope, required string) {
	t.Helper()
	for _, route := range routes {
		rec := serveScoped(route, scope)

		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s com scope %q", route.method, route.path, scope)
		assert.JSONEq(t, `{"error":"insufficient_scope","message":"o token nao tem o escopo `+required+`"}`, rec.Body.String(), route.path)
		assert.Equal(t, `Bearer error="insufficient_scope", scope="`+required+`"`, rec.Header().Get("WWW-Authenticate"), route.path)
	}
}

func TestScopeRoutes_TokenDeLogin_AlcancaTodosOsGrupos(t *testing.T) {
	for _, routes := range [][]scopedRoute{notesReadRoutes, notesWriteRoutes, adminScopeRoutes, anyScopeRoutes, loginOnlyRoutes} {
		assertRoutesPass(t, routes, "")
	}
}

func TestScopeRoutes_NotasLeitura_SoLeAsNotas(t *testing.T) {
	assertRoutesPass(t, notesReadRoutes, "notas:leitura")
	assertRoutesNeedScope(t, notesWriteRoutes, "notas:leitura", "notas:escrita")
	assertRoutesNeedScope(t, adminScopeRoutes, "notas:leitura", "administracao")
}

func TestScopeRoutes_NotasEscrita_LeEEscreveAsNotas(t *testing.T) {
	assertRoutesPass(t, notesReadRoutes, "notas:escrita")
	assertRoutesPass(t, notesWriteRoutes, "notas:escrita")
	assertRoutesNeedScope(t, adminScopeRoutes, "notas:escrita", "administracao")
}

func TestScopeRoutes_Administracao_SoAdministra(t *testing.T) {
	assertRoutesPass(t, adminScopeRoutes, "administracao")
	assertRoutesNeedScope(t, notesReadRoutes, "administracao", "notas:leitura")
	assertRoutesNeedScope(t, notesWriteRoutes, "administracao", "notas:escrita")
}

func TestScopeRoutes_ContaETenants_ValemComQualquerEscopo(t *testing.T) {
	for _, scope := range []string{"notas:leitura", "notas:escrita", "administracao"} {
		assertRoutesPass(t, anyScopeRoutes, scope)
	}
}

func TestScopeRoutes_RotasSoDeLogin_RecusamTokenOAuth(t *testing.T) {
	for _, scope := range []string{"notas:leitura", "notas:leitura notas:escrita administracao"} {
		assertFirstPartyOnly(t, loginOnlyRoutes, scope)
	}
}

func assertFirstPartyOnly(t *testing.T, routes []scopedRoute, scope string) {
	t.Helper()
	for _, route := range routes {
		rec := serveScoped(route, scope)

		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s com scope %q", route.method, route.path, scope)
		assert.JSONEq(t, `{"error":"first_party_only","message":"rota so aceita o token de login"}`, rec.Body.String(), route.path)
		assert.Empty(t, rec.Header().Get("WWW-Authenticate"), route.path)
	}
}

func TestScopeRoutes_EscopoDasNotas_FalhaAntesDoTenant(t *testing.T) {
	semTenant := &fakeTenantResolver{err: auth.ErrTenantRequired}
	for _, route := range slices.Concat(notesReadRoutes, notesWriteRoutes) {
		rec := serveScopedWith(route, "administracao", semTenant)

		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s", route.method, route.path)
		assert.Contains(t, rec.Body.String(), `"error":"insufficient_scope"`, route.path)
	}
}

func TestScopeRoutes_ScopeSoDeEspacos_RecusadoEmTodaRota(t *testing.T) {
	for _, route := range slices.Concat(notesReadRoutes, notesWriteRoutes, adminScopeRoutes, anyScopeRoutes, loginOnlyRoutes) {
		rec := serveScoped(route, "  \t ")

		assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s", route.method, route.path)
		assert.JSONEq(t, `{"error":"unauthorized","message":"token ausente ou invalido"}`, rec.Body.String(), route.path)
	}
}
