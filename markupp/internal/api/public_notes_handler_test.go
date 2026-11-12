package api_test

import (
	"context"
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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

const idPublicoDeTeste = "Ab3dEf6hIj9k"

// fakePublicNoteReader serve só a nota idPublicoDeTeste e guarda o que a rota pediu.
type fakePublicNoteReader struct {
	tag            string
	query          string
	offset, limit  int
	graphed        bool
	requestedFirst string
}

func (f *fakePublicNoteReader) check(publicID string) error {
	if f.requestedFirst == "" {
		f.requestedFirst = publicID
	}
	if publicID != idPublicoDeTeste {
		return fmt.Errorf("%w: %q", publication.ErrPublicNoteNotFound, publicID)
	}
	return nil
}

func (f *fakePublicNoteReader) PublicNote(_ context.Context, publicID string) (publication.PublicNote, error) {
	passo := "Zz9yXx8wVv7u"
	return publication.PublicNote{
		PublicID: publicID, Title: "Guia de Deploy", Content: "leia o [[Passo]]", UpdatedAt: time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC),
		Publication: publication.PublicScope{Kind: "folder", Title: "docs"},
		Links:       []publication.PublicLink{{CitedTitle: "Passo", Kind: "explicit", PublicID: &passo}, {CitedTitle: "Segredo", Kind: "explicit"}},
	}, f.check(publicID)
}

func (f *fakePublicNoteReader) PublicTree(_ context.Context, publicID string, offset, limit int) ([]publication.PublicTreeItem, int, error) {
	f.offset, f.limit = offset, limit
	return []publication.PublicTreeItem{{PublicID: publicID, Title: "Guia de Deploy", Path: "Guia de Deploy.md"}}, 5, f.check(publicID)
}

func (f *fakePublicNoteReader) SearchPublic(_ context.Context, publicID, query string, offset, limit int) ([]publication.PublicSearchHit, int, error) {
	f.query, f.offset, f.limit = query, offset, limit
	return []publication.PublicSearchHit{{PublicID: publicID, Title: "Guia", Snippet: "o \x02deploy\x03", Rank: 0.5}}, 1, f.check(publicID)
}

func (f *fakePublicNoteReader) PublicGraphTag(_ context.Context, publicID string) (string, error) {
	return f.tag, f.check(publicID)
}

func (f *fakePublicNoteReader) PublicGraph(_ context.Context, publicID string) (publication.PublicGraph, error) {
	f.graphed = true
	grau := 1
	return publication.PublicGraph{
		Nodes: []publication.PublicGraphNode{{PublicID: publicID, Title: "Guia", Degree: &grau}, {PublicID: "Zz9yXx8wVv7u", Title: "Passo", Degree: &grau}},
		Links: []publication.PublicGraphLink{{Source: 0, Target: 1, Type: "explicit"}},
	}, f.check(publicID)
}

// routerPublico monta só a leitura pública, sem token válido nenhum.
func routerPublico(reader api.PublicNoteReader, limiter *api.AccountRateLimiter) http.Handler {
	return api.NewRouter(api.RouterDeps{
		Tokens:      &fakeTokenValidator{err: auth.ErrInvalidToken},
		PublicNotes: reader, PublicRateLimiter: limiter,
	})
}

func getPublico(router http.Handler, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

const notaPublicaPath = "/v1/public/notes/" + idPublicoDeTeste

func TestNotaPublica_SemConta_DevolveANotaComPublicacaoELigacoes(t *testing.T) {
	rec := getPublico(routerPublico(&fakePublicNoteReader{}, nil), notaPublicaPath, nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"public_id":"Ab3dEf6hIj9k","title":"Guia de Deploy","title_slug":"guia-de-deploy","content":"leia o [[Passo]]",
		"updated_at":"2026-11-12T09:00:00Z","indexable":false,"publication":{"kind":"folder","title":"docs","root_public_id":null},
		"links":[{"cited_title":"Passo","kind":"explicit","public_id":"Zz9yXx8wVv7u","title_slug":"passo"},
		         {"cited_title":"Segredo","kind":"explicit","public_id":null,"title_slug":null}]}`, rec.Body.String())
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, rec.Header().Get("ETag"))
}

func TestNotaPublica_InexistenteOuNaoPublicada_MesmoNotFound(t *testing.T) {
	router := routerPublico(&fakePublicNoteReader{}, nil)

	inexistente := getPublico(router, "/v1/public/notes/AAAAAAAAAAAA", nil)
	foraDoFormato := getPublico(router, "/v1/public/notes/..%2Fx", nil)

	require.Equal(t, http.StatusNotFound, inexistente.Code)
	assert.JSONEq(t, `{"error":"not_found","message":"note not found"}`, inexistente.Body.String())
	assert.Equal(t, http.StatusNotFound, foraDoFormato.Code)
	assert.JSONEq(t, inexistente.Body.String(), foraDoFormato.Body.String())
}

func TestNotaPublica_ETagIgual_Responde304(t *testing.T) {
	router := routerPublico(&fakePublicNoteReader{}, nil)
	tag := getPublico(router, notaPublicaPath, nil).Header().Get("ETag")

	rec := getPublico(router, notaPublicaPath, map[string]string{"If-None-Match": tag})

	assert.Equal(t, http.StatusNotModified, rec.Code)
}

func TestArvorePublica_ItensETotalComPagina(t *testing.T) {
	reader := &fakePublicNoteReader{}

	rec := getPublico(routerPublico(reader, nil), notaPublicaPath+"/tree?offset=10&limit=5", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"items":[{"public_id":"Ab3dEf6hIj9k","title":"Guia de Deploy","title_slug":"guia-de-deploy","path":"Guia de Deploy.md"}],"total":5}`, rec.Body.String())
	assert.Equal(t, []int{10, 5}, []int{reader.offset, reader.limit})
}

func TestBuscaPublica_ItensComTrechoETotal(t *testing.T) {
	reader := &fakePublicNoteReader{}

	rec := getPublico(routerPublico(reader, nil), notaPublicaPath+"/search?q=deploy", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"items":[{"public_id":"Ab3dEf6hIj9k","title":"Guia","title_slug":"guia","snippet":"o \u0002deploy\u0003","rank":0.5}],"total":1}`, rec.Body.String())
	assert.Equal(t, "deploy", reader.query)
}

func TestBuscaPublica_SemConsultaOuLonga_400(t *testing.T) {
	router := routerPublico(&fakePublicNoteReader{}, nil)

	assert.Equal(t, http.StatusBadRequest, getPublico(router, notaPublicaPath+"/search", nil).Code)
	rec := getPublico(router, notaPublicaPath+"/search?q="+strings.Repeat("a", 257), nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "257")
}

func TestGrafoPublico_NosPeloIDPublicoComETagDaVersao(t *testing.T) {
	reader := &fakePublicNoteReader{tag: "7-abc"}
	router := routerPublico(reader, nil)

	rec := getPublico(router, notaPublicaPath+"/graph", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"nodes":[{"public_id":"Ab3dEf6hIj9k","title":"Guia","title_slug":"guia","degree":1},
		{"public_id":"Zz9yXx8wVv7u","title":"Passo","title_slug":"passo","degree":1}],
		"links":[{"source":0,"target":1,"type":"explicit"}]}`, rec.Body.String())
	assert.Equal(t, `W/"7-abc"`, rec.Header().Get("ETag"))
	reader.graphed = false
	rec = getPublico(router, notaPublicaPath+"/graph", map[string]string{"If-None-Match": `W/"7-abc"`})
	assert.Equal(t, http.StatusNotModified, rec.Code)
	assert.False(t, reader.graphed, "304 nao monta o grafo")
}

func TestLeituraPublica_LimitePorIPSomaTodasAsRotas(t *testing.T) {
	clock := &fakeRateClock{current: time.Unix(1_000_000, 0)}
	router := routerPublico(&fakePublicNoteReader{}, api.NewAccountRateLimiter(2, time.Minute, clock.Now, nil))
	require.Equal(t, http.StatusOK, getPublico(router, notaPublicaPath, nil).Code)
	require.Equal(t, http.StatusOK, getPublico(router, notaPublicaPath+"/tree", nil).Code)

	rec := getPublico(router, "/v1/public/notes/AAAAAAAAAAAA", nil)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "id novo nao abre limite novo")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestLeituraPublica_PreflightSemLimiteParaQualquerOrigem(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, notaPublicaPath, nil)
	req.Header.Set("Origin", "https://outro.exemplo.dev")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	routerPublico(&fakePublicNoteReader{}, nil).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}
