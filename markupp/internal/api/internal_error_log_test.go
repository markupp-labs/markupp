package api_test

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// logCapturado troca o log padrão por um que escreve no buffer até o fim do teste.
func logCapturado(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	return &buf
}

func TestLogin_ErroInesperado_RegistraOErroNoLog(t *testing.T) {
	buf := logCapturado(t)
	router := setupAuthRouter(&fakeAuthService{loginErr: errors.New("banco fora")}, nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"a@markupp.dev","password":"senhaSegura123"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, buf.String(), "banco fora", "o 500 sem log esconde a causa de quem opera")
}

func TestOAuth_ErroInesperado_RegistraComMensagemDoOAuth(t *testing.T) {
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/oauth/authorize?client_id=c1", nil),
		httptest.NewRequest(http.MethodGet, "/v1/oauth/requests/p1", nil),
	} {
		buf := logCapturado(t)
		req.Header.Set("Authorization", "Bearer token-valido")

		rec := serveOAuth(&fakeOAuthService{err: errors.New("banco fora")}, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code, req.URL.Path)
		assert.Contains(t, buf.String(), `"msg":"falha no oauth"`, req.URL.Path)
		assert.Contains(t, buf.String(), "banco fora", req.URL.Path)
	}
}

func TestAccountEmail_ErroInesperado_RegistraOErroNoLog(t *testing.T) {
	buf := logCapturado(t)

	rec := accountEmailRequest(&fakeAccountEmailService{err: errors.New("banco fora")}, http.MethodPost, "/auth/password/reset", `{"token":"x","password":"senhaNova123"}`, false)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, buf.String(), "banco fora")
}
