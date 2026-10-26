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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

// fakePublicVaultReader serve um cofre publicado, "docs", e guarda o que a rota pediu.
type fakePublicVaultReader struct {
	vault   publication.PublicVault
	list    []notes.Note
	note    notes.Note
	hits    []notes.SearchResult
	links   []notes.Link
	drawn   notes.Graph
	err     error
	slug    string
	query   string
	noteID  string
	limit   int
	graphed bool
}

func (f *fakePublicVaultReader) check(slug string) error {
	f.slug = slug
	if f.err != nil {
		return f.err
	}
	if slug != "docs" {
		return fmt.Errorf("%w: slug %q", publication.ErrNotPublished, slug)
	}
	return nil
}

func (f *fakePublicVaultReader) PublicVault(_ context.Context, slug string) (publication.PublicVault, error) {
	return f.vault, f.check(slug)
}

func (f *fakePublicVaultReader) PublicNotes(_ context.Context, slug string, q notes.VaultNotesQuery) ([]notes.Note, error) {
	f.limit = q.Limit
	return f.list, f.check(slug)
}

func (f *fakePublicVaultReader) PublicNote(_ context.Context, slug, noteID string) (notes.Note, error) {
	f.noteID = noteID
	return f.note, f.check(slug)
}

func (f *fakePublicVaultReader) SearchPublicNotes(_ context.Context, slug, query string, _, limit int) ([]notes.SearchResult, error) {
	f.query, f.limit = query, limit
	return f.hits, f.check(slug)
}

func (f *fakePublicVaultReader) PublicGraphVersion(_ context.Context, slug string) (int64, error) {
	return f.vault.GraphVersion, f.check(slug)
}

func (f *fakePublicVaultReader) PublicGraph(_ context.Context, slug string, _ notes.GraphFilter) (notes.Graph, error) {
	f.graphed = true
	return f.drawn, f.check(slug)
}

func (f *fakePublicVaultReader) PublicLinks(_ context.Context, slug string, q notes.VaultLinkQuery) ([]notes.Link, error) {
	f.limit = q.Limit
	return f.links, f.check(slug)
}

var publicadoEm = time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

func cofrePublicoDeTeste() *fakePublicVaultReader {
	return &fakePublicVaultReader{
		vault: publication.PublicVault{VaultID: "interno", Slug: "docs", Title: "Docs", PublishedAt: publicadoEm, GraphVersion: 7},
		note:  notes.Note{ID: "n1", VaultID: "interno", Path: "guia/inicio.md", Content: "# oi", ContentSHA256: "abc", CreatedAt: publicadoEm, UpdatedAt: publicadoEm},
		list:  []notes.Note{{ID: "n1", VaultID: "interno", Path: "guia/inicio.md", ContentSHA256: "abc", UpdatedAt: publicadoEm}},
		hits:  []notes.SearchResult{{ID: "n1", VaultID: "interno", Path: "guia/inicio.md", Snippet: "oi", Rank: 0.5}},
		drawn: grafoDeTeste,
	}
}

// routerPublico monta o roteador só com a leitura pública e o limite dado; o validador
// recusa qualquer token, para provar que a rota não pede conta.
func routerPublico(reader api.PublicVaultReader, limiter *api.AccountRateLimiter) http.Handler {
	return api.NewRouter(api.RouterDeps{
		Tokens:       &fakeTokenValidator{err: auth.ErrInvalidToken},
		PublicVaults: reader, PublicRateLimiter: limiter,
	})
}

func getPublico(router http.Handler, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.RemoteAddr = "203.0.113.9:4000"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPublico_Cofre_SemContaDevolveSlugTituloEVersao(t *testing.T) {
	rec := getPublico(routerPublico(cofrePublicoDeTeste(), nil), "/v1/public/vaults/docs", map[string]string{"Origin": "https://site.example"})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"slug":"docs","title":"Docs","published_at":"2026-10-25T09:00:00Z","graph_version":7}`, rec.Body.String())
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "public, no-cache", rec.Header().Get("Cache-Control"))
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	assert.Contains(t, rec.Header().Get("Access-Control-Expose-Headers"), "ETag")
}

func TestPublico_CofreNaoPublicado_Responde404(t *testing.T) {
	for _, rota := range []string{"", "/notes", "/notes/n1", "/search?q=oi", "/graph", "/links"} {
		rec := getPublico(routerPublico(cofrePublicoDeTeste(), nil), "/v1/public/vaults/privado"+rota, nil)

		assert.Equal(t, http.StatusNotFound, rec.Code, rota)
		assert.Contains(t, rec.Body.String(), `"not_found"`, rota)
	}
}

func TestPublico_MesmoETag_Responde304SemCorpo(t *testing.T) {
	router := routerPublico(cofrePublicoDeTeste(), nil)
	primeira := getPublico(router, "/v1/public/vaults/docs/notes/n1", nil)
	require.Equal(t, http.StatusOK, primeira.Code)

	rec := getPublico(router, "/v1/public/vaults/docs/notes/n1", map[string]string{"If-None-Match": primeira.Header().Get("ETag")})

	assert.Equal(t, http.StatusNotModified, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, primeira.Header().Get("ETag"), rec.Header().Get("ETag"))
}

func TestPublico_Nota_SemOCofreInterno(t *testing.T) {
	reader := cofrePublicoDeTeste()

	rec := getPublico(routerPublico(reader, nil), "/v1/public/vaults/docs/notes/n1", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "n1", reader.noteID)
	assert.JSONEq(t, `{"id":"n1","title":"inicio","path":"guia/inicio.md","content":"# oi","content_sha256":"abc",
		"created_at":"2026-10-25T09:00:00Z","updated_at":"2026-10-25T09:00:00Z","degree":null,"pagerank":null,"community":null}`, rec.Body.String())
}

func TestPublico_Notas_ListaSemConteudoComPagina(t *testing.T) {
	reader := cofrePublicoDeTeste()

	rec := getPublico(routerPublico(reader, nil), "/v1/public/vaults/docs/notes?limit=5", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 5, reader.limit)
	assert.JSONEq(t, `[{"id":"n1","title":"inicio","path":"guia/inicio.md","updated_at":"2026-10-25T09:00:00Z",
		"content_sha256":"abc","degree":null,"pagerank":null,"community":null}]`, rec.Body.String())
}

func TestPublico_Busca_SemOCofreInterno(t *testing.T) {
	reader := cofrePublicoDeTeste()

	rec := getPublico(routerPublico(reader, nil), "/v1/public/vaults/docs/search?q=oi&limit=3", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "oi", reader.query)
	assert.Equal(t, 3, reader.limit)
	assert.JSONEq(t, `[{"note":{"id":"n1","title":"inicio","path":"guia/inicio.md"},"rank":0.5,"snippet":"oi"}]`, rec.Body.String())
}

func TestPublico_BuscaSemQOuLongaDemais_Responde400(t *testing.T) {
	for _, alvo := range []string{"/search", "/search?q=" + strings.Repeat("a", 257)} {
		reader := cofrePublicoDeTeste()

		rec := getPublico(routerPublico(reader, nil), "/v1/public/vaults/docs"+alvo, nil)

		assert.Equal(t, http.StatusBadRequest, rec.Code, alvo)
		assert.Empty(t, reader.query, "consulta invalida nao chega ao banco")
	}
}

func TestPublico_Ligacoes_ListaVaziaComoArray(t *testing.T) {
	rec := getPublico(routerPublico(cofrePublicoDeTeste(), nil), "/v1/public/vaults/docs/links", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestPublico_Grafo_ETagDaVersaoECachePublico(t *testing.T) {
	reader := cofrePublicoDeTeste()
	router := routerPublico(reader, nil)

	rec := getPublico(router, "/v1/public/vaults/docs/graph", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, grafoDeTesteJSON, rec.Body.String())
	assert.Equal(t, `W/"7"`, rec.Header().Get("ETag"))
	assert.Equal(t, "public, no-cache", rec.Header().Get("Cache-Control"))

	reader.graphed = false
	rec = getPublico(router, "/v1/public/vaults/docs/graph", map[string]string{"If-None-Match": `W/"7"`})
	assert.Equal(t, http.StatusNotModified, rec.Code)
	assert.False(t, reader.graphed, "com a versao atual o grafo nem e lido")
}

func TestPublico_Preflight_LiberaGETEIfNoneMatch(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/v1/public/vaults/docs/notes", nil)
	req.Header.Set("Origin", "https://site.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	routerPublico(cofrePublicoDeTeste(), nil).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), http.MethodGet)
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "If-None-Match")
}

func TestPublico_AcimaDoLimitePorIP_Responde429EmTodasAsRotas(t *testing.T) {
	clock := &fakeRateClock{current: time.Unix(1_000_000, 0)}
	router := routerPublico(cofrePublicoDeTeste(), api.NewAccountRateLimiter(2, time.Minute, clock.Now, nil))
	require.Equal(t, http.StatusOK, getPublico(router, "/v1/public/vaults/docs", nil).Code)
	require.Equal(t, http.StatusOK, getPublico(router, "/v1/public/vaults/docs/notes", nil).Code)

	rec := getPublico(router, "/v1/public/vaults/outro-slug/notes/n9", nil)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "slug e nota novos nao abrem limite novo")
	assert.Equal(t, "60", rec.Header().Get("Retry-After"))
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"), "o 429 tambem e legivel por outra origem")
}

func TestPublico_LimiteVenceuAJanela_VoltaAResponder(t *testing.T) {
	clock := &fakeRateClock{current: time.Unix(1_000_000, 0)}
	router := routerPublico(cofrePublicoDeTeste(), api.NewAccountRateLimiter(1, time.Minute, clock.Now, nil))
	require.Equal(t, http.StatusOK, getPublico(router, "/v1/public/vaults/docs", nil).Code)
	require.Equal(t, http.StatusTooManyRequests, getPublico(router, "/v1/public/vaults/docs", nil).Code)

	clock.current = clock.current.Add(time.Minute)

	assert.Equal(t, http.StatusOK, getPublico(router, "/v1/public/vaults/docs", nil).Code)
}

func TestPublico_SemLeituraPublica_RotaNaoExiste(t *testing.T) {
	router := api.NewRouter(api.RouterDeps{Tokens: &fakeTokenValidator{}})

	rec := getPublico(router, "/v1/public/vaults/docs", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
