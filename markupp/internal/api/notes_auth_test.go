package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

func TestNotesRoutes_SemToken_Retorna401(t *testing.T) {
	svc := &fakeService{listResult: nil}
	validator := &fakeTokenValidator{}
	router := api.NewRouter(api.RouterDeps{Notes: svc, Tokens: validator, Probe: &fakeProbe{}})

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestNotesRoutes_TokenInvalido_Retorna401(t *testing.T) {
	svc := &fakeService{listResult: nil}
	validator := &fakeTokenValidator{err: errors.New("token invalido")}
	router := api.NewRouter(api.RouterDeps{Notes: svc, Tokens: validator, Probe: &fakeProbe{}})

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("Authorization", "Bearer token-falso")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestNotesRoutes_TokenValido_AcessaNotasComSucesso(t *testing.T) {
	svc := &fakeService{listResult: nil}
	validator := &fakeTokenValidator{
		claims: auth.Claims{
			Subject:  "usuario-1",
			TenantID: "tenant-default",
			Role:     "member",
		},
	}
	router := api.NewRouter(api.RouterDeps{Notes: svc, Tokens: validator, Probe: &fakeProbe{}})

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
