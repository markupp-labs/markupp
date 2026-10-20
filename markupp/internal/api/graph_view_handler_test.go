package api_test

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

func (f *fakeNoteGraph) VaultGraph(_ context.Context, vaultID string, filter notes.GraphFilter) (notes.Graph, error) {
	f.record("vault_graph", vaultID, 0, 0)
	f.query = filter
	return f.drawn, f.err
}

func (f *fakeNoteGraph) LocalGraph(_ context.Context, noteID string, depth int, filter notes.GraphFilter) (notes.Graph, error) {
	f.record("local_graph", noteID, depth, 0)
	f.query = filter
	return f.drawn, f.err
}

func (f *fakeNoteGraph) VaultGraphVersion(_ context.Context, vaultID string) (int64, error) {
	f.record("vault_graph_version", vaultID, 0, 0)
	return f.version, f.err
}

func (f *fakeNoteGraph) NoteGraphVersion(_ context.Context, noteID string) (int64, error) {
	f.record("note_graph_version", noteID, 0, 0)
	return f.version, f.err
}

// getGrafoDesenhado faz o GET em target como viewer, com os cabeçalhos dados.
func getGrafoDesenhado(t *testing.T, graph api.NoteGraph, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	router := api.NewRouter(api.RouterDeps{
		Notes: &fakeService{}, Graph: graph, Probe: &fakeProbe{},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: "viewer"}},
		Tokens:  &fakeTokenValidator{claims: auth.Claims{Subject: "usuario-teste"}},
	})
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

var grafoDeTeste = notes.Graph{
	Nodes: []notes.GraphNode{
		{ID: ptr("n1"), Title: "a", Path: ptr("a.md"), Degree: ptr(1), PageRank: ptr(0.5), Community: ptr(int64(7))},
		{Title: "Foto.png", Unresolved: true},
	},
	Links: []notes.GraphLink{{Source: 0, Target: 1, Type: notes.LinkEmbed}},
}

const grafoDeTesteJSON = `{"nodes":[
	{"id":"n1","title":"a","path":"a.md","degree":1,"pagerank":0.5,"community":7,"unresolved":false},
	{"id":null,"title":"Foto.png","path":null,"degree":null,"pagerank":null,"community":null,"unresolved":true}],
	"links":[{"source":0,"target":1,"type":"embed","predicate":null}]}`

func TestV1GetVaultGraph_NosEParesDeIndicesComFiltrosEETag(t *testing.T) {
	graph := &fakeNoteGraph{drawn: grafoDeTeste, version: 42}

	rec := getGrafoDesenhado(t, graph, "/v1/vaults/va/graph?orphans=false&unresolved=false&inferred=false&community=7", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, grafoDeTesteJSON, rec.Body.String())
	assert.Equal(t, `W/"42"`, rec.Header().Get("ETag"))
	assert.Equal(t, []any{"vault_graph", "va"}, []any{graph.method, graph.id})
	assert.Equal(t, notes.GraphFilter{HideOrphans: true, HideUnresolved: true, HideInferred: true, Community: ptr(int64(7))}, graph.query)
}

func TestV1GetVaultGraph_FiltrosVerdadeirosOuAusentesNaoEscondem(t *testing.T) {
	graph := &fakeNoteGraph{}

	rec := getGrafoDesenhado(t, graph, "/v1/vaults/va/graph?orphans=true&inferred=true", nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, notes.GraphFilter{}, graph.query)
	assert.Empty(t, rec.Header().Get("ETag"), "sem versao antes do primeiro segundo estagio")
}

func TestV1Graph_IfNoneMatchDaVersao_Retorna304SemLerOGrafo(t *testing.T) {
	for _, alvo := range []string{"/v1/vaults/va/graph", "/v1/notes/n1/graph?depth=2"} {
		for _, pedido := range []string{`W/"42"`, `"42"`, `"7", W/"42"`, `*`} {
			graph := &fakeNoteGraph{version: 42}

			rec := getGrafoDesenhado(t, graph, alvo, map[string]string{"If-None-Match": pedido})

			assert.Equal(t, http.StatusNotModified, rec.Code, "%s com %s", alvo, pedido)
			assert.Empty(t, rec.Body.String())
			assert.Equal(t, `W/"42"`, rec.Header().Get("ETag"))
			assert.Contains(t, graph.method, "graph_version", "%s com %s nao le o grafo", alvo, pedido)
		}
	}
}

func TestV1Graph_IfNoneMatchDeOutraVersao_Retorna200(t *testing.T) {
	graph := &fakeNoteGraph{drawn: grafoDeTeste, version: 43}

	rec := getGrafoDesenhado(t, graph, "/v1/vaults/va/graph", map[string]string{"If-None-Match": `W/"42"`})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, `W/"43"`, rec.Header().Get("ETag"))
}

func TestV1Graph_CacheControlPrivadoQueRevalida(t *testing.T) {
	for _, alvo := range []string{"/v1/vaults/va/graph", "/v1/notes/n1/graph"} {
		for _, pedido := range []string{"", `W/"42"`} {
			graph := &fakeNoteGraph{drawn: grafoDeTeste, version: 42}

			rec := getGrafoDesenhado(t, graph, alvo, map[string]string{"If-None-Match": pedido})

			assert.Equal(t, "private, no-cache", rec.Header().Get("Cache-Control"),
				"%s com If-None-Match %q: so o navegador do usuario guarda, e revalida pelo ETag", alvo, pedido)
		}
	}
}

func TestV1Graph_ComGzip_RespondeComprimido(t *testing.T) {
	graph := &fakeNoteGraph{drawn: grafoDeTeste, version: 42}

	rec := getGrafoDesenhado(t, graph, "/v1/vaults/va/graph", map[string]string{"Accept-Encoding": "gzip"})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	leitor, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	corpo, err := io.ReadAll(leitor)
	require.NoError(t, err)
	assert.JSONEq(t, grafoDeTesteJSON, string(corpo))
}

func TestV1GetNoteGraph_ProfundidadePadraoEUmEVaiAteTres(t *testing.T) {
	for alvo, profundidade := range map[string]int{"/v1/notes/n1/graph": 1, "/v1/notes/n1/graph?depth=3&unresolved=false": 3} {
		graph := &fakeNoteGraph{drawn: grafoDeTeste}

		rec := getGrafoDesenhado(t, graph, alvo, nil)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, grafoDeTesteJSON, rec.Body.String())
		assert.Equal(t, []any{"local_graph", "n1", profundidade}, []any{graph.method, graph.id, graph.offset}, alvo)
	}
}

func TestV1Graph_ParametroInvalido_Retorna400ComOValorEOFormato(t *testing.T) {
	casos := map[string]string{
		"/v1/notes/n1/graph?depth=0":           `depth "0", esperado inteiro de 1 a 3`,
		"/v1/notes/n1/graph?depth=4":           `depth "4", esperado inteiro de 1 a 3`,
		"/v1/notes/n1/graph?depth=dois":        `depth "dois", esperado inteiro de 1 a 3`,
		"/v1/vaults/va/graph?orphans=talvez":   `orphans "talvez", esperado true ou false`,
		"/v1/vaults/va/graph?community=grande": `community "grande", esperado inteiro`,
	}
	for alvo, mensagem := range casos {
		rec := getGrafoDesenhado(t, &fakeNoteGraph{}, alvo, nil)

		assert.Equal(t, http.StatusBadRequest, rec.Code, alvo)
		var corpo map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&corpo), alvo)
		assert.Contains(t, corpo["message"], mensagem, alvo)
	}
}

func TestV1Graph_ErrosDeDominio_ViramStatus(t *testing.T) {
	casos := map[string]struct {
		err    error
		status int
	}{
		"/v1/notes/x/graph":  {notes.ErrNotFound, http.StatusNotFound},
		"/v1/vaults/x/graph": {notes.ErrVaultForbidden, http.StatusForbidden},
	}
	for alvo, caso := range casos {
		rec := getGrafoDesenhado(t, &fakeNoteGraph{err: caso.err}, alvo, nil)

		assert.Equal(t, caso.status, rec.Code, "%s: %s", alvo, rec.Body.String())
	}
}
