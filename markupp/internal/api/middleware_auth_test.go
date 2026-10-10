package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

type fakeTokenValidator struct {
	claims auth.Claims
	err    error
}

func (f *fakeTokenValidator) ValidateToken(tokenString string) (auth.Claims, error) {
	if f.err != nil {
		return auth.Claims{}, f.err
	}
	return f.claims, nil
}

func TestRequireAuth_TokenValido_InjetaContextoEProssegue(t *testing.T) {
	validator := &fakeTokenValidator{
		claims: auth.Claims{
			Subject:  "user-123",
			TenantID: "tenant-456",
			Role:     "admin",
		},
	}

	var reached bool
	var capturedUser api.UserContext

	handler := api.RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		u, ok := api.UserFromContext(r.Context())
		require.True(t, ok)
		capturedUser = u
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	req.Header.Set("Authorization", "Bearer token-valido-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, reached)
	assert.Equal(t, "user-123", capturedUser.UserID)
	assert.Equal(t, "tenant-456", capturedUser.TenantID)
	assert.Equal(t, "admin", capturedUser.Role)
}

func TestRequireAuth_SemHeaderAuthorization_Retorna401(t *testing.T) {
	validator := &fakeTokenValidator{}
	handler := api.RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestRequireAuth_FormatoNaoBearer_Retorna401(t *testing.T) {
	validator := &fakeTokenValidator{}
	handler := api.RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpzZW5oYQ==")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestRequireAuth_TokenInvalidoOuExpirado_Retorna401(t *testing.T) {
	validator := &fakeTokenValidator{err: errors.New("token adulterado")}
	handler := api.RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	req.Header.Set("Authorization", "Bearer token-invalido")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}
