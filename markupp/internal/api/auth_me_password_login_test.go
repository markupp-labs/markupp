package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func meResponseFor(t *testing.T, user auth.User) map[string]any {
	t.Helper()
	svc := &fakeAuthService{accountUser: user}
	router := setupAuthRouter(svc, &fakeTokenValidator{claims: auth.Claims{Subject: user.ID}})
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp
}

func TestAuthHandler_Me_PasswordLoginLigado_DevolveTrue(t *testing.T) {
	resp := meResponseFor(t, auth.User{ID: "user-id-42", Email: "admin@markupp.dev", PasswordLogin: true})
	assert.Equal(t, true, resp["password_login"])
}

func TestAuthHandler_Me_PasswordLoginDesligado_DevolveFalse(t *testing.T) {
	resp := meResponseFor(t, auth.User{ID: "user-id-42", Email: "admin@markupp.dev"})
	assert.Equal(t, false, resp["password_login"])
}
