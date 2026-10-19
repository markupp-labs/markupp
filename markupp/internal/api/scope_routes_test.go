package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

type scopedRoute struct{ method, path string }

var (
	notesReadRoutes = []scopedRoute{
		{http.MethodGet, "/v1/notes"}, {http.MethodGet, "/v1/notes/search?q=x"}, {http.MethodGet, "/v1/notes/n1"},
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
	router := api.NewRouter(api.RouterDeps{
		Notes: &fakeService{}, Auth: &fakeAuthService{}, Admin: &fakeAdminService{},
		AccountEmail: &fakeAccountEmailService{}, OAuth: &fakeOAuthService{},
		OAuthMetadata: &api.OAuthMetadata{Issuer: issuerDeTeste}, Probe: &fakeProbe{},
		Tokens:  &fakeTokenValidator{claims: auth.Claims{Subject: "u1", Scope: scope}},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: "owner"}},
	})
	req := httptest.NewRequest(route.method, route.path, nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertRoutesPass(t *testing.T, routes []scopedRoute, scope string) {
	t.Helper()
	for _, route := range routes {
		rec := serveScoped(route, scope)

		assert.NotContains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, rec.Code, "%s %s com scope %q", route.method, route.path, scope)
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
		for _, route := range loginOnlyRoutes {
			rec := serveScoped(route, scope)

			assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s com scope %q", route.method, route.path, scope)
			assert.Contains(t, rec.Body.String(), `"error":"first_party_only"`, route.path)
			assert.Empty(t, rec.Header().Get("WWW-Authenticate"), route.path)
		}
	}
}
