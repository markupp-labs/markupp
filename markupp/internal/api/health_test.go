package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
)

// fakeProbe responde a sonda de armazenamento com o erro configurado no teste.
type fakeProbe struct {
	err    error
	called bool
}

func (f *fakeProbe) PingContext(ctx context.Context) error {
	f.called = true
	return f.err
}

func healthRequest(t *testing.T, probe api.StorageProbe) *httptest.ResponseRecorder {
	t.Helper()
	router := api.NewRouter(&fakeService{}, probe, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	return rec
}

func TestHealthz_ArmazenamentoResponde_Retorna200(t *testing.T) {
	probe := &fakeProbe{}

	rec := healthRequest(t, probe)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, probe.called, "a sonda deve consultar o armazenamento")

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestHealthz_ArmazenamentoFalha_Retorna503(t *testing.T) {
	rec := healthRequest(t, &fakeProbe{err: errors.New("conexao fechada")})

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "unavailable", body["status"])
}
