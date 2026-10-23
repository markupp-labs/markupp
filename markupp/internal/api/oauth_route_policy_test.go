package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oauthRoutePolicy é o que a rota aceita: public não passa pelo RequireAuth, anyScope
// aceita token OAuth com qualquer escopo e scope exige aquele escopo. Rota fora de
// declaredOAuthPolicies tem de recusar todo token OAuth com first_party_only.
type oauthRoutePolicy struct {
	public, anyScope bool
	scope            string
}

var (
	publicRoute   = oauthRoutePolicy{public: true}
	anyScopeRoute = oauthRoutePolicy{anyScope: true}
	notesRead     = oauthRoutePolicy{scope: "notas:leitura"}
	notesWrite    = oauthRoutePolicy{scope: "notas:escrita"}
	adminScope    = oauthRoutePolicy{scope: "administracao"}
)

// declaredOAuthPolicies lista, por método e padrão do chi, as rotas que não são só de
// login. Rota nova fica fora daqui e quebra o teste se aceitar token OAuth.
var declaredOAuthPolicies = map[string]oauthRoutePolicy{
	"GET /.well-known/oauth-authorization-server/": publicRoute, "OPTIONS /.well-known/oauth-authorization-server/": publicRoute,
	"GET /oauth/authorize/": publicRoute, "POST /oauth/token": publicRoute, "OPTIONS /oauth/token": publicRoute,
	"POST /oauth/revoke": publicRoute, "OPTIONS /oauth/revoke": publicRoute,
	"GET /v1/healthz": publicRoute, "POST /v1/auth/login": publicRoute, "POST /v1/auth/refresh": publicRoute,
	"POST /v1/auth/logout": publicRoute, "POST /v1/auth/invites/accept": publicRoute,
	"POST /v1/auth/password/forgot": publicRoute, "POST /v1/auth/password/reset": publicRoute,
	"GET /v1/auth/me": anyScopeRoute, "GET /v1/me/tenants": anyScopeRoute,
	"GET /v1/notes": notesRead, "GET /v1/notes/search": notesRead, "GET /v1/notes/{id}": notesRead,
	"POST /v1/notes": notesWrite, "PUT /v1/notes/{id}": notesWrite, "DELETE /v1/notes/{id}": notesWrite,
	"GET /v1/vaults/{id}/notes": notesRead, "POST /v1/vaults/{id}/notes": notesWrite, "GET /v1/search/notes": notesRead,
	"GET /v1/notes/{id}/links": notesRead, "GET /v1/notes/{id}/backlinks": notesRead, "GET /v1/vaults/{id}/links": notesRead,
	"GET /v1/notes/{id}/mentions": notesRead, "GET /v1/notes/{id}/backmentions": notesRead,
	"GET /v1/notes/{id}/suggestions": notesRead, "GET /v1/vaults/{id}/communities": notesRead,
	"GET /v1/vaults/{id}/communities/{cid}": notesRead, "GET /v1/vaults/{id}/graph": notesRead,
	"GET /v1/notes/{id}/graph": notesRead,
	"GET /v1/vaults":           notesRead, "POST /v1/vaults": notesWrite, "PATCH /v1/vaults/{id}": notesWrite,
	"DELETE /v1/vaults/{id}": notesWrite, "GET /v1/vaults/{id}/members": notesRead,
	"POST /v1/vaults/{id}/members": notesWrite, "DELETE /v1/vaults/{id}/members/{user_id}": notesWrite,
	"GET /v1/users": adminScope, "GET /v1/users/invites": adminScope, "POST /v1/users/invites": adminScope,
	"PATCH /v1/users/{id}": adminScope, "DELETE /v1/users/{id}": adminScope, "DELETE /v1/users/invites/{id}": adminScope,
	"POST /v1/tenants": adminScope, "GET /v1/me/invites": adminScope,
	"POST /v1/me/invites/{id}/accept": adminScope, "POST /v1/me/invites/{id}/decline": adminScope,
}

// v1OnlyPrefixes são as rotas sem o alias obsoleto sem prefixo, já com os parâmetros
// trocados por x1: as do OAuth e as da API orientada a recursos (ADR-0040).
var v1OnlyPrefixes = []string{"/v1/me/authorizations", "/v1/oauth/requests", "/v1/vaults", "/v1/search/", "/v1/notes/x1/"}

const allOAuthScopes = "notas:leitura notas:escrita administracao"

var routeParam = regexp.MustCompile(`\{[^}]+\}`)

func TestOAuthRoutePolicy_TodaRotaRecusaTokenOAuthOuDeclaraOQueAceita(t *testing.T) {
	routes := walkedRoutes(t)
	require.NotEmpty(t, routes)
	for _, route := range routes {
		if route.path == "/*" {
			continue // alias obsoleto sem prefixo, conferido pelas rotas de /v1
		}
		policy := declaredOAuthPolicies[route.method+" "+route.path]
		for _, path := range pathsWithAlias(routeParam.ReplaceAllString(route.path, "x1")) {
			assertOAuthRoutePolicy(t, scopedRoute{route.method, path}, policy)
		}
	}
}

func TestOAuthRoutePolicy_ListaSoTemRotasRegistradas(t *testing.T) {
	registered := map[string]bool{}
	for _, route := range walkedRoutes(t) {
		registered[route.method+" "+route.path] = true
	}
	for key := range declaredOAuthPolicies {
		assert.True(t, registered[key], "rota declarada sem registro no roteador: %s", key)
	}
}

func walkedRoutes(t *testing.T) []scopedRoute {
	t.Helper()
	var routes []scopedRoute
	router := routerWithAllServices(&fakeTokenValidator{}, &fakeTenantResolver{})
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, scopedRoute{method, route})
		return nil
	})
	require.NoError(t, err)
	return routes
}

// pathsWithAlias junta ao caminho de /v1 o alias obsoleto sem prefixo, quando existe.
func pathsWithAlias(path string) []string {
	alias, versioned := strings.CutPrefix(path, "/v1")
	v1Only := slices.ContainsFunc(v1OnlyPrefixes, func(prefix string) bool { return strings.HasPrefix(path, prefix) })
	if !versioned || v1Only {
		return []string{path}
	}
	return []string{path, alias}
}

func assertOAuthRoutePolicy(t *testing.T, route scopedRoute, policy oauthRoutePolicy) {
	t.Helper()
	switch {
	case policy.public:
		assertPublicRoute(t, route)
	case policy.anyScope:
		assertAcceptsEachScope(t, route)
	case policy.scope != "":
		assertNeedsScope(t, route, policy.scope)
	default:
		assertFirstPartyOnly(t, []scopedRoute{route}, allOAuthScopes)
	}
}

// assertPublicRoute prova que a rota não passa pelo RequireAuth: com um validador que
// recusa todo token, a resposta não é o 401 dele.
func assertPublicRoute(t *testing.T, route scopedRoute) {
	t.Helper()
	router := routerWithAllServices(&fakeTokenValidator{err: errors.New("token recusado")}, &fakeTenantResolver{})
	req := httptest.NewRequest(route.method, route.path, nil)
	req.Header.Set("Authorization", "Bearer token-oauth")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusUnauthorized, rec.Code, "%s %s deveria ser publica", route.method, route.path)
}

func assertAcceptsEachScope(t *testing.T, route scopedRoute) {
	t.Helper()
	for _, scope := range strings.Fields(allOAuthScopes) {
		assertNotRefused(t, route, scope)
	}
}

// assertNeedsScope prova que a rota passa com o escopo exigido e recusa os outros.
func assertNeedsScope(t *testing.T, route scopedRoute, required string) {
	t.Helper()
	assertNotRefused(t, route, required)
	assertRoutesNeedScope(t, []scopedRoute{route}, otherScopes(required), required)
}

func assertNotRefused(t *testing.T, route scopedRoute, scope string) {
	t.Helper()
	rec := serveScoped(route, scope)

	assert.NotContains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, rec.Code, "%s %s com scope %q", route.method, route.path, scope)
}

// otherScopes são os escopos que não satisfazem required; notas:escrita satisfaz notas:leitura.
func otherScopes(required string) string {
	others := slices.DeleteFunc(strings.Fields(allOAuthScopes), func(s string) bool {
		return s == required || (required == "notas:leitura" && s == "notas:escrita")
	})
	return strings.Join(others, " ")
}
