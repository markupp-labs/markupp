package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// fakeScopedClaimsValidator aceita todo token e devolve as claims guardadas.
type fakeScopedClaimsValidator struct{ claims auth.Claims }

func (f fakeScopedClaimsValidator) ValidateToken(string) (auth.Claims, error) {
	return f.claims, nil
}

func TestRequireAuth_TokenComScopeNaRotaQueAceita_MarcaOAtorComoOAuth(t *testing.T) {
	validator := fakeScopedClaimsValidator{claims: auth.Claims{Subject: "user-123", Scope: "notas:leitura administracao"}}
	var ator identity.Actor
	handler := acceptOAuthScope(anyOAuthScope)(RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ator, _ = identity.ActorFrom(r.Context())
	})))
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	req.Header.Set("Authorization", "Bearer token-oauth")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, identity.Actor{UserID: "user-123", FromOAuth: true, OAuthScopes: []string{"notas:leitura", "administracao"}}, ator)
}

func TestRequireAuth_DeclaracaoDepoisDoRequireAuth_RecusaTokenOAuth(t *testing.T) {
	validator := fakeScopedClaimsValidator{claims: auth.Claims{Subject: "user-123", Scope: "notas:leitura"}}
	handler := RequireAuth(validator)(acceptOAuthScope(anyOAuthScope)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	req.Header.Set("Authorization", "Bearer token-oauth")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.JSONEq(t, `{"error":"first_party_only","message":"rota so aceita o token de login"}`, rec.Body.String())
}
