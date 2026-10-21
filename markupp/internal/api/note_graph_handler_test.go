package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// fakeNoteGraph devolve listas fixas e guarda o último pedido recebido.
type fakeNoteGraph struct {
	links     []notes.Link
	mentions  []notes.Mention
	suggested []notes.LinkSuggestion
	groups    []notes.Community
	drawn     notes.Graph
	version   int64
	err       error

	method string
	id     string
	offset int
	limit  int
	query  any
}

func (f *fakeNoteGraph) record(method, id string, offset, limit int) {
	f.method, f.id, f.offset, f.limit = method, id, offset, limit
}

func (f *fakeNoteGraph) Links(_ context.Context, noteID string, q notes.LinkFilter) ([]notes.Link, error) {
	f.record("links", noteID, q.Offset, q.Limit)
	f.query = q
	return f.links, f.err
}

func (f *fakeNoteGraph) Backlinks(_ context.Context, noteID string, q notes.LinkFilter) ([]notes.Link, error) {
	f.record("backlinks", noteID, q.Offset, q.Limit)
	f.query = q
	return f.links, f.err
}

func (f *fakeNoteGraph) VaultLinks(_ context.Context, vaultID string, q notes.VaultLinkQuery) ([]notes.Link, error) {
	f.record("vault_links", vaultID, q.Offset, q.Limit)
	f.query = q
	return f.links, f.err
}

func (f *fakeNoteGraph) Mentions(_ context.Context, noteID string, p notes.Page) ([]notes.Mention, error) {
	f.record("mentions", noteID, p.Offset, p.Limit)
	return f.mentions, f.err
}

func (f *fakeNoteGraph) Backmentions(_ context.Context, noteID string, p notes.Page) ([]notes.Mention, error) {
	f.record("backmentions", noteID, p.Offset, p.Limit)
	return f.mentions, f.err
}

func (f *fakeNoteGraph) Suggestions(_ context.Context, noteID string, p notes.Page) ([]notes.LinkSuggestion, error) {
	f.record("suggestions", noteID, p.Offset, p.Limit)
	return f.suggested, f.err
}

func (f *fakeNoteGraph) Communities(_ context.Context, vaultID string, p notes.Page) ([]notes.Community, error) {
	f.record("communities", vaultID, p.Offset, p.Limit)
	return f.groups, f.err
}

func (f *fakeNoteGraph) Community(_ context.Context, vaultID string, communityID int64) (notes.Community, error) {
	f.record("community", vaultID, 0, 0)
	f.query = communityID
	if len(f.groups) == 0 {
		return notes.Community{}, f.err
	}
	return f.groups[0], f.err
}

func getGrafo(t *testing.T, graph api.NoteGraph, role, target string) *httptest.ResponseRecorder {
	t.Helper()
	router := api.NewRouter(api.RouterDeps{
		Notes: &fakeService{}, Graph: graph, Probe: &fakeProbe{},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: role}},
		Tokens:  &fakeTokenValidator{claims: auth.Claims{Subject: "usuario-teste"}},
	})
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

var ligacaoDeTeste = notes.Link{
	Source:     notes.NoteSummary{ID: "n1", Title: "a", Path: "a.md"},
	Target:     &notes.NoteSummary{ID: "n2", Title: "b", Path: "pasta/b.md"},
	CitedTitle: "B", Type: "typed", Predicate: ptr("autor"),
}

const ligacaoDeTesteJSON = `{"source":{"id":"n1","title":"a","path":"a.md"},"target":{"id":"n2","title":"b","path":"pasta/b.md"},
	"cited_title":"B","type":"typed","predicate":"autor","inferred_target":null}`

func ptr[T any](v T) *T { return &v }

func TestV1GetLinksEBacklinks_UmItemPorLigacaoComFiltroDeTipo(t *testing.T) {
	for _, sentido := range []string{"links", "backlinks"} {
		graph := &fakeNoteGraph{links: []notes.Link{ligacaoDeTeste}}

		rec := getGrafo(t, graph, "viewer", "/v1/notes/n1/"+sentido+"?type=typed&offset=3&limit=500")

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, "["+ligacaoDeTesteJSON+"]", rec.Body.String())
		assert.Equal(t, []any{sentido, "n1", 3, 100}, []any{graph.method, graph.id, graph.offset, graph.limit}, "limit acima do teto vira 100")
		assert.Equal(t, notes.LinkFilter{Type: "typed", Page: notes.Page{Offset: 3, Limit: 100}}, graph.query)
	}
}

func TestV1GetLinks_NaoResolvidaComPalpite(t *testing.T) {
	palpite := notes.NoteSummary{ID: "k", Title: "Kubernetes", Path: "Kubernetes.md"}
	graph := &fakeNoteGraph{links: []notes.Link{{Source: ligacaoDeTeste.Source, CitedTitle: "Kubernets", Type: "explicit", InferredTarget: &palpite}}}

	rec := getGrafo(t, graph, "member", "/v1/notes/n1/links")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"source":{"id":"n1","title":"a","path":"a.md"},"target":null,"cited_title":"Kubernets","type":"explicit",
		"predicate":null,"inferred_target":{"id":"k","title":"Kubernetes","path":"Kubernetes.md"}}]`, rec.Body.String())
	assert.Equal(t, notes.LinkFilter{Page: notes.Page{Limit: 10}}, graph.query, "sem filtro, na pagina padrao")
}

func TestV1GetVaultLinks_FiltraPorStatusETipo(t *testing.T) {
	graph := &fakeNoteGraph{}

	rec := getGrafo(t, graph, "viewer", "/v1/vaults/va/links?status=unresolved&type=embed")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[]`, rec.Body.String(), "lista vazia sai como array")
	assert.Equal(t, "va", graph.id)
	assert.Equal(t, notes.VaultLinkQuery{Status: notes.LinkUnresolved, LinkFilter: notes.LinkFilter{Type: "embed", Page: notes.Page{Limit: 10}}}, graph.query)
}

func TestV1Ligacoes_FiltroInvalido_Retorna400ComOValor(t *testing.T) {
	for _, alvo := range []string{"/v1/notes/n1/links?type=forte", "/v1/notes/n1/backlinks?type=forte", "/v1/vaults/va/links?status=talvez", "/v1/vaults/va/links?type=forte"} {
		rec := getGrafo(t, &fakeNoteGraph{}, "member", alvo)

		assert.Equal(t, http.StatusBadRequest, rec.Code, alvo)
		var corpo map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&corpo), alvo)
		assert.Equal(t, "invalid_request", corpo["error"], alvo)
		assert.Regexp(t, "forte|talvez", corpo["message"], alvo)
	}
}

func TestRotasDasLigacoesSemVersao_SaemSemVersaoObsoleta(t *testing.T) {
	for _, rota := range []string{"/notes/n1/backlinks", "/notes/n1/neighbors", "/v1/notes/n1/neighbors", "/vaults/va/unresolved-links"} {
		rec := getGrafo(t, &fakeNoteGraph{}, "member", rota)

		assert.Equal(t, http.StatusNotFound, rec.Code, rota)
	}
}

func TestV1GetMentionsEBackmentions_DevolvemANotaOTituloEAProbabilidade(t *testing.T) {
	for _, sentido := range []string{"mentions", "backmentions"} {
		graph := &fakeNoteGraph{mentions: []notes.Mention{
			{Note: notes.NoteSummary{ID: "n2", Title: "b", Path: "pasta/b.md"}, CitedTitle: "kubernetes", Prior: 0.5},
		}}

		rec := getGrafo(t, graph, "viewer", "/v1/notes/n1/"+sentido+"?limit=5")

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, `[{"note":{"id":"n2","title":"b","path":"pasta/b.md"},"cited_title":"kubernetes","prior":0.5}]`, rec.Body.String())
		assert.Equal(t, []any{sentido, "n1", 0, 5}, []any{graph.method, graph.id, graph.offset, graph.limit})
	}
}

func TestRotaDeMencoesSemVersao_SaiSemVersaoObsoleta(t *testing.T) {
	rec := getGrafo(t, &fakeNoteGraph{}, "member", "/notes/n1/mentions")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestV1GetSuggestions_DevolveANotaAPontuacaoFundidaEOsDoisSinais(t *testing.T) {
	graph := &fakeNoteGraph{suggested: []notes.LinkSuggestion{
		{Note: notes.NoteSummary{ID: "n2", Title: "b", Path: "pasta/b.md"}, Score: 0.03, ResourceAllocation: 0.5, Cosine: 0.25},
	}}

	rec := getGrafo(t, graph, "viewer", "/v1/notes/n1/suggestions")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"note":{"id":"n2","title":"b","path":"pasta/b.md"},"score":0.03,"resource_allocation":0.5,"cosine":0.25}]`, rec.Body.String())
	assert.Equal(t, []any{"suggestions", "n1", 0, 10}, []any{graph.method, graph.id, graph.offset, graph.limit}, "limite padrao do repo")
	assert.Equal(t, http.StatusNotFound, getGrafo(t, &fakeNoteGraph{}, "member", "/notes/n1/suggestions").Code, "a rota sem versao sai")
}

var comunidadeDeTeste = notes.Community{ID: 7, Size: 12, Hub: notes.NoteSummary{ID: "n2", Title: "b", Path: "pasta/b.md"}}

const comunidadeDeTesteJSON = `{"id":7,"size":12,"hub":{"id":"n2","title":"b","path":"pasta/b.md"}}`

func TestV1GetCommunities_DevolveTamanhoENotaCentral(t *testing.T) {
	graph := &fakeNoteGraph{groups: []notes.Community{comunidadeDeTeste}}

	rec := getGrafo(t, graph, "viewer", "/v1/vaults/va/communities?offset=2")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, "["+comunidadeDeTesteJSON+"]", rec.Body.String())
	assert.Equal(t, []any{"communities", "va", 2, 10}, []any{graph.method, graph.id, graph.offset, graph.limit})
	assert.Equal(t, http.StatusNotFound, getGrafo(t, &fakeNoteGraph{}, "member", "/vaults/va/clusters").Code, "a rota sem versao sai")
}

func TestV1GetCommunity_LeUmaPeloId(t *testing.T) {
	graph := &fakeNoteGraph{groups: []notes.Community{comunidadeDeTeste}}

	rec := getGrafo(t, graph, "member", "/v1/vaults/va/communities/7")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, comunidadeDeTesteJSON, rec.Body.String())
	assert.Equal(t, []any{"community", "va", int64(7)}, []any{graph.method, graph.id, graph.query})
}

func TestV1GetCommunity_IdNaoInteiro_Retorna400ComOValor(t *testing.T) {
	rec := getGrafo(t, &fakeNoteGraph{}, "member", "/v1/vaults/va/communities/sete")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "sete")
}

func TestGrafo_ErrosDeDominio_ViramStatus(t *testing.T) {
	casos := map[string]struct {
		err    error
		target string
		status int
	}{
		"nota inexistente":       {notes.ErrNotFound, "/v1/notes/x/backlinks", http.StatusNotFound},
		"saida sem nota":         {notes.ErrNotFound, "/v1/notes/x/links", http.StatusNotFound},
		"mencao sem nota":        {notes.ErrNotFound, "/v1/notes/x/mentions", http.StatusNotFound},
		"quem menciona sem nota": {notes.ErrNotFound, "/v1/notes/x/backmentions", http.StatusNotFound},
		"sugestao sem nota":      {notes.ErrNotFound, "/v1/notes/x/suggestions", http.StatusNotFound},
		"cofre sem acesso":       {notes.ErrVaultForbidden, "/v1/vaults/x/links", http.StatusForbidden},
		"comunidade sem acesso":  {notes.ErrVaultForbidden, "/v1/vaults/x/communities", http.StatusForbidden},
		"comunidade inexistente": {notes.ErrCommunityNotFound, "/v1/vaults/x/communities/3", http.StatusNotFound},
		"uma sem acesso":         {notes.ErrVaultForbidden, "/v1/vaults/x/communities/3", http.StatusForbidden},
		"limit nao inteiro":      {nil, "/v1/vaults/x/links?limit=abc", http.StatusBadRequest},
		"offset nao inteiro":     {nil, "/v1/notes/x/links?offset=abc", http.StatusBadRequest},
	}
	for nome, caso := range casos {
		rec := getGrafo(t, &fakeNoteGraph{err: caso.err}, "member", caso.target)

		assert.Equal(t, caso.status, rec.Code, "%s: %s", nome, rec.Body.String())
		var corpo map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&corpo), nome)
		assert.NotEmpty(t, corpo["error"], nome)
	}
}
