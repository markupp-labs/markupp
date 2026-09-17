package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
)

const origemPermitida = "https://painel.markupp.dev"

func corsRequest(t *testing.T, origins []string, method, origin string, preflight bool) *httptest.ResponseRecorder {
	t.Helper()
	router := api.NewRouter(&fakeService{listResult: nil}, &fakeProbe{}, origins)
	req := httptest.NewRequest(method, "/notes", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if preflight {
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCORS_SemOrigemConfigurada_NaoMandaCabecalho(t *testing.T) {
	rec := corsRequest(t, nil, http.MethodGet, origemPermitida, false)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_OrigemPermitida_EcoaAOrigem(t *testing.T) {
	rec := corsRequest(t, []string{origemPermitida}, http.MethodGet, origemPermitida, false)

	assert.Equal(t, origemPermitida, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", rec.Header().Get("Vary"))
}

func TestCORS_OrigemNaoPermitida_NaoMandaCabecalho(t *testing.T) {
	rec := corsRequest(t, []string{origemPermitida}, http.MethodGet, "https://invasor.example", false)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, http.StatusOK, rec.Code, "a requisicao segue, o navegador é quem barra")
}

func TestCORS_RequisicaoSemOrigem_NaoMandaCabecalho(t *testing.T) {
	rec := corsRequest(t, []string{origemPermitida}, http.MethodGet, "", false)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightDeOrigemPermitida_Retorna204(t *testing.T) {
	rec := corsRequest(t, []string{origemPermitida}, http.MethodOptions, origemPermitida, true)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, origemPermitida, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), http.MethodPut)
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
}

func TestCORS_PreflightDeOrigemNaoPermitida_NaoResponde204(t *testing.T) {
	rec := corsRequest(t, []string{origemPermitida}, http.MethodOptions, "https://invasor.example", true)

	assert.NotEqual(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}
