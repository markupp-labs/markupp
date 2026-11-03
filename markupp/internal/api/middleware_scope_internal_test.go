package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// serveWithAcceptedScopes passa um token OAuth com o scope dado por uma rota que aceita
// os escopos accepted.
func serveWithAcceptedScopes(scope string, accepted ...string) *httptest.ResponseRecorder {
	validator := fakeScopedClaimsValidator{claims: auth.Claims{Subject: "user-123", Scope: scope}}
	handler := acceptOAuthScope(accepted...)(RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	req.Header.Set("Authorization", "Bearer token-oauth")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestAcceptOAuthScope_ConjuntoDeEscopos_PassaComQualquerUm(t *testing.T) {
	for _, scope := range []string{"notas:leitura", "notas:escrita", "administracao"} {
		rec := serveWithAcceptedScopes(scope, oauth.ScopeNotesRead, oauth.ScopeAdmin)

		assert.Equal(t, http.StatusOK, rec.Code, scope)
	}
}

func TestAcceptOAuthScope_ConjuntoDeEscopos_RecusaEscopoForaDele(t *testing.T) {
	rec := serveWithAcceptedScopes("notas:leitura", oauth.ScopeNotesWrite, oauth.ScopeAdmin)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"insufficient_scope","message":"o token nao tem o escopo notas:escrita ou administracao"}`, rec.Body.String())
	assert.Equal(t, `Bearer error="insufficient_scope", scope="notas:escrita administracao"`, rec.Header().Get("WWW-Authenticate"))
}

func TestAcceptOAuthScope_SemEscopo_RecusaTokenOAuth(t *testing.T) {
	rec := serveWithAcceptedScopes("notas:leitura administracao")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"first_party_only","message":"rota so aceita o token de login"}`, rec.Body.String())
}
