package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

// fakeAccountEmailService faz o papel do serviço de email da conta: guarda o que
// recebeu e devolve err.
type fakeAccountEmailService struct {
	err error

	verificationFor string
	confirmedToken  string
	resetEmail      string
	resetToken      string
	resetPassword   string
}

func (f *fakeAccountEmailService) RequestEmailVerification(_ context.Context, userID string) error {
	f.verificationFor = userID
	return f.err
}

func (f *fakeAccountEmailService) ConfirmEmail(_ context.Context, token string) error {
	f.confirmedToken = token
	return f.err
}

func (f *fakeAccountEmailService) RequestPasswordReset(_ context.Context, email string) error {
	f.resetEmail = email
	return f.err
}

func (f *fakeAccountEmailService) ResetPassword(_ context.Context, token, password string) error {
	f.resetToken, f.resetPassword = token, password
	return f.err
}

func accountEmailRequest(svc api.AccountEmailService, method, path, body string, logged bool) *httptest.ResponseRecorder {
	validator := &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}}
	router := api.NewRouter(api.RouterDeps{AccountEmail: svc, Tokens: validator})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if logged {
		req.Header.Set("Authorization", "Bearer token-valido")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func codigoDoErro(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	code, _ := resp["error"].(string)
	return code
}

func TestAccountEmailRoutes_PedirVerificacao_ExigeLoginEPedeParaAConta(t *testing.T) {
	svc := &fakeAccountEmailService{}

	assert.Equal(t, http.StatusUnauthorized, accountEmailRequest(svc, http.MethodPost, "/auth/email/verification", "", false).Code)
	rec := accountEmailRequest(svc, http.MethodPost, "/auth/email/verification", "", true)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "u1", svc.verificationFor)
}

func TestAccountEmailRoutes_ErrosDeDominio(t *testing.T) {
	casos := []struct {
		err    error
		path   string
		body   string
		status int
		code   string
	}{
		{auth.ErrEmailAlreadyVerified, "/auth/email/verification", "", http.StatusConflict, "email_already_verified"},
		{auth.ErrEmailUnavailable, "/auth/email/verification", "", http.StatusServiceUnavailable, "email_unavailable"},
		{auth.ErrInvalidToken, "/auth/email/verify", `{"token":"x"}`, http.StatusBadRequest, "invalid_token"},
		{auth.ErrEmailUnavailable, "/auth/password/forgot", `{"email":"a@markupp.dev"}`, http.StatusServiceUnavailable, "email_unavailable"},
		{auth.ErrEmptyEmail, "/auth/password/forgot", `{"email":""}`, http.StatusBadRequest, "invalid_request"},
		{auth.ErrInvalidToken, "/auth/password/reset", `{"token":"x","password":"senhaNova123"}`, http.StatusBadRequest, "invalid_token"},
		{auth.ErrEmptyPassword, "/auth/password/reset", `{"token":"x","password":""}`, http.StatusBadRequest, "invalid_request"},
	}
	for _, c := range casos {
		rec := accountEmailRequest(&fakeAccountEmailService{err: c.err}, http.MethodPost, c.path, c.body, true)

		assert.Equal(t, c.status, rec.Code, "%s com %v", c.path, c.err)
		assert.Equal(t, c.code, codigoDoErro(t, rec), "%s com %v", c.path, c.err)
	}
}

func TestAccountEmailRoutes_ConfirmarEmail_SemLogin(t *testing.T) {
	svc := &fakeAccountEmailService{}

	rec := accountEmailRequest(svc, http.MethodPost, "/auth/email/verify", `{"token":"tok-v"}`, false)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "tok-v", svc.confirmedToken)
}

func TestAccountEmailRoutes_PedirTroca_SempreAceitaSemDizerSeAContaExiste(t *testing.T) {
	svc := &fakeAccountEmailService{}

	rec := accountEmailRequest(svc, http.MethodPost, "/auth/password/forgot", `{"email":"qualquer@markupp.dev"}`, false)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "qualquer@markupp.dev", svc.resetEmail)
}

func TestAccountEmailRoutes_TrocarSenha_SemLogin(t *testing.T) {
	svc := &fakeAccountEmailService{}

	rec := accountEmailRequest(svc, http.MethodPost, "/auth/password/reset", `{"token":"tok-r","password":"senhaNova123"}`, false)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "tok-r", svc.resetToken)
	assert.Equal(t, "senhaNova123", svc.resetPassword)
}
