package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

type fakeAuthService struct {
	loginPair    auth.TokenPair
	loginErr     error
	refreshPair  auth.TokenPair
	refreshErr   error
	revokeErr    error
	accountUser  auth.User
	accountErr   error
	loginCalls   int
	refreshCalls int
	revokeCalls  int
	accountCalls int
}

func (f *fakeAuthService) LoginLocal(ctx context.Context, email, password string) (auth.TokenPair, error) {
	f.loginCalls++
	return f.loginPair, f.loginErr
}

func (f *fakeAuthService) RefreshToken(ctx context.Context, rawRefreshToken string) (auth.TokenPair, error) {
	f.refreshCalls++
	return f.refreshPair, f.refreshErr
}

func (f *fakeAuthService) RevokeSession(ctx context.Context, rawRefreshToken string) error {
	f.revokeCalls++
	return f.revokeErr
}

func (f *fakeAuthService) GetAccount(ctx context.Context, userID string) (auth.User, error) {
	f.accountCalls++
	return f.accountUser, f.accountErr
}

func setupAuthRouter(svc api.AuthService, validator api.TokenValidator) http.Handler {
	return api.NewRouterWithAuth(&fakeService{}, svc, validator, &fakeProbe{}, nil)
}

func TestAuthHandler_Login_Sucesso(t *testing.T) {
	svc := &fakeAuthService{
		loginPair: auth.TokenPair{
			AccessToken:  "access-token-jwt",
			RefreshToken: "refresh-token-xyz",
			ExpiresIn:    900,
		},
	}
	router := setupAuthRouter(svc, nil)

	body := `{"email":"admin@markupp.dev","password":"senhaSegura123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, svc.loginCalls)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "access-token-jwt", resp["access_token"])
	assert.Equal(t, "refresh-token-xyz", resp["refresh_token"])
	assert.Equal(t, float64(900), resp["expires_in"])
}

func TestAuthHandler_Login_JSONInvalido_Retorna400(t *testing.T) {
	router := setupAuthRouter(&fakeAuthService{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString("{corrompido"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}

func TestAuthHandler_Login_CredenciaisInvalidas_Retorna401(t *testing.T) {
	svc := &fakeAuthService{loginErr: auth.ErrInvalidCredentials}
	router := setupAuthRouter(svc, nil)

	body := `{"email":"admin@markupp.dev","password":"senhaErrada"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_credentials")
}

func TestAuthHandler_Login_AutoRegistroDesativado_Retorna403(t *testing.T) {
	svc := &fakeAuthService{loginErr: auth.ErrRegistrationDisabled}
	router := setupAuthRouter(svc, nil)

	body := `{"email":"novo@markupp.dev","password":"senhaQualquer"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "registration_disabled")
}

func TestAuthHandler_Refresh_Sucesso(t *testing.T) {
	svc := &fakeAuthService{
		refreshPair: auth.TokenPair{
			AccessToken:  "novo-access-token",
			RefreshToken: "novo-refresh-token",
			ExpiresIn:    900,
		},
	}
	router := setupAuthRouter(svc, nil)

	body := `{"refresh_token":"token-antigo"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, svc.refreshCalls)
}

func TestAuthHandler_Refresh_TokenRevogado_Retorna401(t *testing.T) {
	svc := &fakeAuthService{refreshErr: auth.ErrRefreshTokenRevoked}
	router := setupAuthRouter(svc, nil)

	body := `{"refresh_token":"token-revogado"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_refresh_token")
}

func TestAuthHandler_Logout_Sucesso(t *testing.T) {
	svc := &fakeAuthService{}
	router := setupAuthRouter(svc, nil)

	body := `{"refresh_token":"token-para-revogar"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, svc.revokeCalls)
}

func TestAuthHandler_Me_Sucesso(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	svc := &fakeAuthService{
		accountUser: auth.User{
			ID:        "user-id-42",
			TenantID:  "tenant-default",
			Email:     "admin@markupp.dev",
			Role:      "admin",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	validator := &fakeTokenValidator{
		claims: auth.Claims{
			Subject:  "user-id-42",
			TenantID: "tenant-default",
			Role:     "admin",
		},
	}
	router := setupAuthRouter(svc, validator)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, svc.accountCalls)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "user-id-42", resp["id"])
	assert.Equal(t, "admin@markupp.dev", resp["email"])
	assert.Equal(t, "admin", resp["role"])
}

func TestAuthHandler_Login_EmailOuSenhaVazios_Retorna400(t *testing.T) {
	svc := &fakeAuthService{loginErr: auth.ErrEmptyEmail}
	router := setupAuthRouter(svc, nil)

	body := `{"email":"","password":"senha"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "email obrigatorio")
}

func TestAuthHandler_Refresh_JSONInvalido_Retorna400(t *testing.T) {
	router := setupAuthRouter(&fakeAuthService{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString("{corrompido"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}

func TestAuthHandler_Logout_JSONInvalido_Retorna400(t *testing.T) {
	router := setupAuthRouter(&fakeAuthService{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString("{corrompido"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}

func TestAuthHandler_Me_UsuarioNaoEncontrado_Retorna404(t *testing.T) {
	svc := &fakeAuthService{accountErr: auth.ErrUserNotFound}
	validator := &fakeTokenValidator{
		claims: auth.Claims{
			Subject:  "usuario-deletado",
			TenantID: "tenant-default",
			Role:     "member",
		},
	}
	router := setupAuthRouter(svc, validator)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}
