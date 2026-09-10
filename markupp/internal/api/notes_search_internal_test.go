package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

type fakeBuscaService struct {
	resultado []notes.SearchResult
	erro      error
	offset    int
	limit     int
	chamado   bool
}

func (f *fakeBuscaService) Create(ctx context.Context, path, content string) (notes.Note, error) {
	return notes.Note{}, nil
}

func (f *fakeBuscaService) Update(ctx context.Context, id, path, content string, lastModifiedAt time.Time, force bool) (notes.Note, error) {
	return notes.Note{}, nil
}

func (f *fakeBuscaService) Delete(ctx context.Context, id string) error {
	return nil
}

func (f *fakeBuscaService) GetNoteById(ctx context.Context, id string) (notes.Note, error) {
	return notes.Note{}, nil
}

func (f *fakeBuscaService) ListNotes(ctx context.Context) ([]notes.Note, error) {
	return nil, nil
}

func (f *fakeBuscaService) SearchNotes(ctx context.Context, query string, offset, limit int) ([]notes.SearchResult, error) {
	f.chamado = true
	f.offset = offset
	f.limit = limit
	return f.resultado, f.erro
}

type casoClamp struct {
	nome     string
	valor    int
	minimo   int
	maximo   int
	esperado int
}

type casoParseQueryInt struct {
	nome        string
	valor       string
	padrao      int
	esperado    int
	esperaFalha bool
}

func TestClamp_ValoresNasFronteirasEForaDaFaixa_RetornaValorLimitado(t *testing.T) {
	casos := []casoClamp{
		{"abaixo do minimo", -7, 0, maxOffset, 0},
		{"exatamente no minimo", minLimit, minLimit, maxLimit, minLimit},
		{"dentro da faixa", 42, 0, maxOffset, 42},
		{"exatamente no maximo", maxLimit, minLimit, maxLimit, maxLimit},
		{"acima do maximo", maxOffset + 1, 0, maxOffset, maxOffset},
	}
	for _, caso := range casos {
		conferirClamp(t, caso)
	}
}

func conferirClamp(t *testing.T, caso casoClamp) {
	t.Helper()
	t.Run(caso.nome, func(t *testing.T) {
		obtido := clamp(caso.valor, caso.minimo, caso.maximo)
		assert.Equal(t, caso.esperado, obtido,
			"clamp(%d, %d, %d) devolveu %d, esperado %d",
			caso.valor, caso.minimo, caso.maximo, obtido, caso.esperado)
	})
}

func TestParseQueryInt_EntradasValidasEInvalidas_RetornaInteiroOuErro(t *testing.T) {
	casos := []casoParseQueryInt{
		{"entrada vazia usa o padrao", "", 10, 10, false},
		{"entrada vazia com padrao zero", "", 0, 0, false},
		{"inteiro positivo", "25", 10, 25, false},
		{"inteiro negativo", "-3", 10, -3, false},
		{"zero explicito", "0", 10, 0, false},
		{"texto nao numerico", "abc", 10, 0, true},
		{"decimal nao aceito", "12.5", 10, 0, true},
		{"apenas espaco", " ", 10, 0, true},
	}
	for _, caso := range casos {
		conferirParseQueryInt(t, caso)
	}
}

func conferirParseQueryInt(t *testing.T, caso casoParseQueryInt) {
	t.Helper()
	t.Run(caso.nome, func(t *testing.T) {
		obtido, err := parseQueryInt(caso.valor, caso.padrao)
		conferirFalhaParseQueryInt(t, caso, err)
		assert.Equal(t, caso.esperado, obtido,
			"parseQueryInt(%q, %d) devolveu %d, esperado %d",
			caso.valor, caso.padrao, obtido, caso.esperado)
	})
}

func conferirFalhaParseQueryInt(t *testing.T, caso casoParseQueryInt, err error) {
	t.Helper()
	if caso.esperaFalha {
		require.Error(t, err, "parseQueryInt(%q, %d) deveria falhar por nao ser um inteiro", caso.valor, caso.padrao)
		return
	}
	require.NoError(t, err, "parseQueryInt(%q, %d) nao deveria falhar", caso.valor, caso.padrao)
}

func buscarNotas(t *testing.T, svc NoteService, queryString string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/notes/search?"+queryString, nil)
	rec := httptest.NewRecorder()
	NewRouter(svc).ServeHTTP(rec, req)
	return rec
}

func codigoDoErro(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var corpo errorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&corpo),
		"corpo da resposta deveria ser um errorResponse JSON, recebido %q", rec.Body.String())
	return corpo.Error
}

func TestSearch_LimitNaoInteiro_Retorna400InvalidRequest(t *testing.T) {
	svc := &fakeBuscaService{}

	rec := buscarNotas(t, svc, "query=go&limit=abc")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "invalid_request", codigoDoErro(t, rec))
	assert.False(t, svc.chamado, "SearchNotes nao deveria ser chamado com limit invalido")
}

func TestSearch_ServicoRetornaErroDesconhecido_Retorna500Internal(t *testing.T) {
	svc := &fakeBuscaService{erro: errors.New("indice de busca indisponivel")}

	rec := buscarNotas(t, svc, "query=go")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "internal", codigoDoErro(t, rec))
}

func TestSearch_ServicoRetornaErrInvalidId_Retorna400InvalidId(t *testing.T) {
	svc := &fakeBuscaService{erro: notes.ErrInvalidId}

	rec := buscarNotas(t, svc, "query=go")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "invalid_id", codigoDoErro(t, rec))
}

func TestSearch_SemOffsetELimit_UsaValoresPadrao(t *testing.T) {
	svc := &fakeBuscaService{}

	rec := buscarNotas(t, svc, "query=go")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, svc.offset)
	assert.Equal(t, 10, svc.limit)
}

func TestSearch_OffsetELimitAcimaDoMaximo_LimitaAoTeto(t *testing.T) {
	svc := &fakeBuscaService{}

	rec := buscarNotas(t, svc, "query=go&offset=999999&limit=5000")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, maxOffset, svc.offset)
	assert.Equal(t, maxLimit, svc.limit)
}

func TestSearch_OffsetELimitAbaixoDoMinimo_LimitaAoPiso(t *testing.T) {
	svc := &fakeBuscaService{}

	rec := buscarNotas(t, svc, "query=go&offset=-40&limit=0")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, svc.offset)
	assert.Equal(t, minLimit, svc.limit)
}
