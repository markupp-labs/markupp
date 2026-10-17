package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	acceptPair   auth.TokenPair
	acceptErr    error
	acceptToken  string
	tenants      []auth.Membership

	pending            []auth.PendingInvite
	acceptedMembership auth.Membership
	answerErr          error
	answeredInvite     string

	createdTenant   auth.Membership
	createTenantErr error
	tenantCreator   string
	tenantName      string
}

func (f *fakeAuthService) ListMyInvites(ctx context.Context, userID string) ([]auth.PendingInvite, error) {
	return f.pending, nil
}

func (f *fakeAuthService) AcceptMyInvite(ctx context.Context, userID, inviteID string) (auth.Membership, error) {
	f.answeredInvite = inviteID
	return f.acceptedMembership, f.answerErr
}

func (f *fakeAuthService) DeclineMyInvite(ctx context.Context, userID, inviteID string) error {
	f.answeredInvite = inviteID
	return f.answerErr
}

func (f *fakeAuthService) CreateTenant(ctx context.Context, userID, name string) (auth.Membership, error) {
	f.tenantCreator, f.tenantName = userID, name
	return f.createdTenant, f.createTenantErr
}

func (f *fakeAuthService) AcceptInvite(ctx context.Context, token, password string) (auth.TokenPair, error) {
	f.acceptToken = token
	return f.acceptPair, f.acceptErr
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

func (f *fakeAuthService) ListTenants(ctx context.Context, userID string) ([]auth.Membership, error) {
	return f.tenants, nil
}

func (f *fakeAuthService) GetAccount(ctx context.Context, userID string) (auth.User, error) {
	f.accountCalls++
	return f.accountUser, f.accountErr
}

func setupAuthRouter(svc api.AuthService, validator api.TokenValidator) http.Handler {
	if validator == nil {
		validator = &fakeTokenValidator{}
	}
	return api.NewRouter(api.RouterDeps{Notes: &fakeService{}, Tenants: tenantDeTeste, Auth: svc, Tokens: validator, Probe: &fakeProbe{}})
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

func TestAuthHandler_Login_AutenticacaoLocalDesativada_Retorna403(t *testing.T) {
	svc := &fakeAuthService{loginErr: auth.ErrLocalAuthDisabled}
	router := setupAuthRouter(svc, nil)

	body := `{"email":"novo@markupp.dev","password":"senhaQualquer"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "local_auth_disabled")
}

func TestAuthHandler_Login_SenhaMuitoLonga_Retorna400(t *testing.T) {
	svc := &fakeAuthService{loginErr: auth.ErrPasswordTooLong}
	router := setupAuthRouter(svc, nil)

	body := `{"email":"admin@markupp.dev","password":"senha"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}

func TestAuthHandler_Login_CorpoMaiorQue4KiB_Retorna400(t *testing.T) {
	router := setupAuthRouter(&fakeAuthService{}, nil)

	body := fmt.Sprintf(`{"email":"admin@markupp.dev","password":"%s"}`, strings.Repeat("a", 5000))
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
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

func TestAuthHandler_Refresh_CorpoMaiorQue4KiB_Retorna400(t *testing.T) {
	router := setupAuthRouter(&fakeAuthService{}, nil)

	body := fmt.Sprintf(`{"refresh_token":"%s"}`, strings.Repeat("x", 5000))
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
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

func TestAuthHandler_Logout_FalhaNaRevogacao_RegistraLogERetorna204(t *testing.T) {
	svc := &fakeAuthService{revokeErr: errors.New("erro ao persistir revogacao")}
	router := setupAuthRouter(svc, nil)

	body := `{"refresh_token":"token-para-revogar"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, svc.revokeCalls)
}

func TestAuthHandler_Logout_CorpoMaiorQue4KiB_Retorna400(t *testing.T) {
	router := setupAuthRouter(&fakeAuthService{}, nil)

	body := fmt.Sprintf(`{"refresh_token":"%s"}`, strings.Repeat("x", 5000))
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}

func TestAuthHandler_Me_Sucesso(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	svc := &fakeAuthService{
		accountUser: auth.User{
			ID:        "user-id-42",
			Email:     "admin@markupp.dev",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	validator := &fakeTokenValidator{
		claims: auth.Claims{
			Subject: "user-id-42",
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
	assert.NotContains(t, resp, "tenant_id", "a conta e global")
	assert.Equal(t, false, resp["email_verified"], "conta sem email verificado")
}

func TestAuthHandler_Me_EmailVerificado_DevolveTrue(t *testing.T) {
	svc := &fakeAuthService{accountUser: auth.User{ID: "user-id-42", Email: "admin@markupp.dev", EmailVerified: true}}
	router := setupAuthRouter(svc, &fakeTokenValidator{claims: auth.Claims{Subject: "user-id-42"}})

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["email_verified"])
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
			Subject: "usuario-deletado",
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

func TestAuthHandler_Me_ErroInterno_Retorna500(t *testing.T) {
	svc := &fakeAuthService{accountErr: errors.New("erro inesperado de conexao")}
	validator := &fakeTokenValidator{
		claims: auth.Claims{
			Subject: "user-id-42",
		},
	}
	router := setupAuthRouter(svc, validator)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal")
}

func postAccept(router http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/invites/accept", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAuthHandler_AcceptInvite_Sucesso_DevolveSessao(t *testing.T) {
	svc := &fakeAuthService{acceptPair: auth.TokenPair{AccessToken: "jwt-novo", RefreshToken: "r", ExpiresIn: 900}}

	rec := postAccept(setupAuthRouter(svc, nil), `{"token":"tok-1","password":"senhaNova"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "tok-1", svc.acceptToken)
	assert.Contains(t, rec.Body.String(), "jwt-novo")
}

func TestAuthHandler_AcceptInvite_ConviteInvalido_Retorna400(t *testing.T) {
	svc := &fakeAuthService{acceptErr: auth.ErrInviteInvalid}

	rec := postAccept(setupAuthRouter(svc, nil), `{"token":"velho","password":"senhaNova"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_invite")
}

func TestAuthHandler_AcceptInvite_EmailJaTemConta_Retorna409(t *testing.T) {
	svc := &fakeAuthService{acceptErr: auth.ErrUserAlreadyExists}

	rec := postAccept(setupAuthRouter(svc, nil), `{"token":"tok-1","password":"senhaNova"}`)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "already_exists")
}

func TestAuthHandler_MeusTenants_ListaVinculos(t *testing.T) {
	svc := &fakeAuthService{tenants: []auth.Membership{
		{TenantID: "t1", TenantName: "Acme", Role: "owner"},
		{TenantID: "t2", TenantName: "Globex", Role: "viewer"},
	}}
	router := setupAuthRouter(svc, &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}})
	req := httptest.NewRequest(http.MethodGet, "/me/tenants", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, map[string]any{"id": "t2", "name": "Globex", "role": "viewer"}, resp[1])
}

func postTenant(svc *fakeAuthService, body string) *httptest.ResponseRecorder {
	router := setupAuthRouter(svc, &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}})
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAuthHandler_CriarTenant_Retorna201ComPapelOwner(t *testing.T) {
	svc := &fakeAuthService{createdTenant: auth.Membership{TenantID: "t-novo", TenantName: "Acme", Role: "owner"}}

	rec := postTenant(svc, `{"name":"Acme"}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "u1", svc.tenantCreator)
	assert.Equal(t, "Acme", svc.tenantName)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, map[string]any{"id": "t-novo", "name": "Acme", "role": "owner"}, resp)
}

func TestAuthHandler_CriarTenant_NomeInvalido_Retorna400(t *testing.T) {
	rec := postTenant(&fakeAuthService{createTenantErr: auth.ErrInvalidTenantName}, `{"name":""}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}
