package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
)

type fakeRetention struct {
	view     instance.RetentionView
	err      error
	gotActor string
	gotDays  int
}

func (f *fakeRetention) Settings(context.Context) (instance.RetentionView, error) {
	return f.view, f.err
}

func (f *fakeRetention) Update(_ context.Context, actorID string, days int) (instance.RetentionView, error) {
	f.gotActor, f.gotDays = actorID, days
	return f.view, f.err
}

func retentionRequest(svc *fakeRetention, method, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Auth: &fakeAuthService{accountUser: administradorDaInstancia}, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		InstanceAdmins: instance.Admins{emailDoAdministrador}, Retention: svc,
	})
	req := httptest.NewRequest(method, "/v1/instance/retention", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRetention_Ler(t *testing.T) {
	rec := retentionRequest(&fakeRetention{view: instance.RetentionView{Days: 365}}, http.MethodGet, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"days":365,"host_locked":false}`, rec.Body.String())
}

func TestRetention_Trocar(t *testing.T) {
	svc := &fakeRetention{view: instance.RetentionView{Days: 30}}

	rec := retentionRequest(svc, http.MethodPut, `{"days":30}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "u1", svc.gotActor)
	assert.Equal(t, 30, svc.gotDays)
}

func TestRetention_Erros(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{instance.ErrHostLocked, http.StatusConflict, "host_locked"},
		{instance.ErrInvalidRetention, http.StatusBadRequest, "invalid_retention"},
	}
	for _, c := range casos {
		rec := retentionRequest(&fakeRetention{err: c.err}, http.MethodPut, `{"days":30}`)

		assert.Equal(t, c.status, rec.Code, c.code)
		assert.Contains(t, rec.Body.String(), `"error":"`+c.code+`"`)
	}
}

func TestRetention_SemDias_400(t *testing.T) {
	svc := &fakeRetention{}

	rec := retentionRequest(svc, http.MethodPut, `{}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, svc.gotDays)
}
