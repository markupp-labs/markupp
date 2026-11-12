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

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func (f *fakeAuthService) Register(ctx context.Context, email, password string) (auth.TokenPair, error) {
	f.registerEmail = email
	return f.registerPair, f.registerErr
}

func postAccount(router http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAccounts_Create_Sucesso_Devolve201ComSessao(t *testing.T) {
	svc := &fakeAuthService{registerPair: auth.TokenPair{AccessToken: "jwt-novo", RefreshToken: "r-novo", ExpiresIn: 900}}

	rec := postAccount(setupAuthRouter(svc, nil), `{"email":"nova@markupp.dev","password":"senhaNova123-de-teste"}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "nova@markupp.dev", svc.registerEmail)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "jwt-novo", resp["access_token"])
	assert.Equal(t, "r-novo", resp["refresh_token"])
	assert.Equal(t, float64(900), resp["expires_in"])
}

func TestAccounts_Create_ErrosDeDominio_CodigoEStatus(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{auth.ErrRegistrationDisabled, http.StatusForbidden, "registration_disabled"},
		{auth.ErrUserAlreadyExists, http.StatusConflict, "already_exists"},
		{auth.ErrEmptyPassword, http.StatusBadRequest, "invalid_request"},
		{auth.ErrPasswordTooShort, http.StatusBadRequest, "password_too_short"},
		{auth.ErrCommonPassword, http.StatusBadRequest, "common_password"},
		{auth.ErrLocalAuthDisabled, http.StatusForbidden, "local_auth_disabled"},
	}
	for _, c := range casos {
		rec := postAccount(setupAuthRouter(&fakeAuthService{registerErr: c.err}, nil), `{"email":"nova@markupp.dev","password":"x"}`)

		assert.Equal(t, c.status, rec.Code, c.code)
		assert.Contains(t, rec.Body.String(), `"`+c.code+`"`, c.code)
	}
}

func TestAccounts_Create_SoNaVersaoAtual(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBufferString(`{"email":"nova@markupp.dev","password":"x"}`))
	rec := httptest.NewRecorder()

	setupAuthRouter(&fakeAuthService{}, nil).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code, "rota nova nasce so em /v1")
}
