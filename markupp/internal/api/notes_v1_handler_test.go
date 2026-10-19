package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// requisicaoV1 manda method em target para o roteador com svc e graph, no papel role.
func requisicaoV1(svc api.NoteService, graph api.NoteGraph, role, method, target, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Notes: svc, Graph: graph, Probe: &fakeProbe{},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: role}},
		Tokens:  &fakeTokenValidator{claims: auth.Claims{Subject: "usuario-teste"}},
	})
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

var momentoDeTeste = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func notaRanqueada() notes.Note {
	grau, pagerank, comunidade := 3, 0.25, int64(7)
	return notes.Note{
		ID: "n1", VaultID: "va", Path: "a.md", Content: "x", ContentSHA256: "abc",
		CreatedAt: momentoDeTeste, UpdatedAt: momentoDeTeste,
		Rank: notes.NoteRank{Degree: &grau, PageRank: &pagerank, Community: &comunidade},
	}
}

const notaRanqueadaJSON = `{"id":"n1","vault_id":"va","path":"a.md","content":"x","content_sha256":"abc",
	"created_at":"2026-10-16T12:00:00Z","updated_at":"2026-10-16T12:00:00Z","degree":3,"pagerank":0.25,"community":7}`

func TestV1GetNote_TrazAOrganizacaoDerivada(t *testing.T) {
	rec := requisicaoV1(&fakeService{note: notaRanqueada()}, nil, "viewer", http.MethodGet, "/v1/notes/n1", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, notaRanqueadaJSON, rec.Body.String())
}

func TestV1GetNote_AntesDoSegundoEstagio_CamposDerivadosNulos(t *testing.T) {
	nota := notes.Note{ID: "n1", VaultID: "va", Path: "a.md", CreatedAt: momentoDeTeste, UpdatedAt: momentoDeTeste}

	rec := requisicaoV1(&fakeService{note: nota}, nil, "member", http.MethodGet, "/v1/notes/n1", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"id":"n1","vault_id":"va","path":"a.md","content":"","content_sha256":"",
		"created_at":"2026-10-16T12:00:00Z","updated_at":"2026-10-16T12:00:00Z","degree":null,"pagerank":null,"community":null}`, rec.Body.String())
}

func TestV1PutNote_AtualizaEDevolveARepresentacaoV1(t *testing.T) {
	svc := &fakeService{updateNote: notaRanqueada()}

	rec := requisicaoV1(svc, nil, "member", http.MethodPut, "/v1/notes/n1", `{"path":"a.md","content":"x","force":true}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, notaRanqueadaJSON, rec.Body.String())
	assert.Equal(t, []any{"n1", "a.md", "x", true}, []any{svc.updateArgs.id, svc.updateArgs.path, svc.updateArgs.content, svc.updateArgs.force})
}

func TestV1DeleteNote_Retorna204(t *testing.T) {
	svc := &fakeService{}

	rec := requisicaoV1(svc, nil, "member", http.MethodDelete, "/v1/notes/n1", "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "n1", svc.deletedID)
}

func TestV1PostNote_CriaNoCofreDoCaminho(t *testing.T) {
	svc := &fakeService{note: notes.Note{ID: "n1", VaultID: "va", Path: "a.md", CreatedAt: momentoDeTeste, UpdatedAt: momentoDeTeste}}

	rec := requisicaoV1(svc, nil, "member", http.MethodPost, "/v1/vaults/va/notes", `{"vault_id":"outro","path":"a.md","content":""}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, "va", svc.createdVaultID, "o cofre vem do caminho, nao do corpo")
	assert.Contains(t, rec.Body.String(), `"degree":null`)
}

func TestV1ListVaultNotes_RepassaFiltroOrdemEPagina(t *testing.T) {
	svc := &fakeService{listResult: []notes.Note{notaRanqueada()}}

	rec := requisicaoV1(svc, nil, "viewer", http.MethodGet, "/v1/vaults/va/notes?orphan=true&community=7&sort=pagerank&direction=asc&offset=2&limit=5", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, "["+notaRanqueadaJSON+"]", rec.Body.String())
	orfa, comunidade := true, int64(7)
	assert.Equal(t, "va", svc.vaultListArgs.vaultID)
	assert.Equal(t, notes.VaultNotesQuery{Orphan: &orfa, Community: &comunidade, Sort: notes.SortByPageRank, Offset: 2, Limit: 5}, svc.vaultListArgs.query)
}

func TestV1ListVaultNotes_SemParametros_PorCaminhoNaPaginaPadrao(t *testing.T) {
	svc := &fakeService{}

	rec := requisicaoV1(svc, nil, "member", http.MethodGet, "/v1/vaults/va/notes", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[]`, rec.Body.String(), "lista vazia sai como array")
	assert.Equal(t, notes.VaultNotesQuery{Descending: true, Limit: 10}, svc.vaultListArgs.query, "direction padrao e desc")
}

func TestV1ListVaultNotes_ParametroInvalido_Retorna400ComOValor(t *testing.T) {
	casos := map[string]string{
		"orphan":    "orphan=talvez",
		"community": "community=sete",
		"sort":      "sort=titulo",
		"direction": "direction=cima",
		"limit":     "limit=abc",
	}
	for nome, consulta := range casos {
		rec := requisicaoV1(&fakeService{}, nil, "member", http.MethodGet, "/v1/vaults/va/notes?"+consulta, "")

		assert.Equal(t, http.StatusBadRequest, rec.Code, nome)
		var corpo map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&corpo), nome)
		assert.Equal(t, "invalid_request", corpo["error"], nome)
		assert.Contains(t, corpo["message"], nome, "a mensagem diz o parametro")
	}
}

func TestV1ListVaultNotes_CofreSemAcesso_Retorna403(t *testing.T) {
	rec := requisicaoV1(&fakeService{listErr: notes.ErrVaultForbidden}, nil, "member", http.MethodGet, "/v1/vaults/va/notes", "")

	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestV1Notes_Viewer_EscritaRetorna403(t *testing.T) {
	escritas := [][3]string{
		{http.MethodPost, "/v1/vaults/va/notes", `{"path":"x.md","content":"y"}`},
		{http.MethodPut, "/v1/notes/id-x", `{"path":"x.md","content":"y","force":true}`},
		{http.MethodDelete, "/v1/notes/id-x", ""},
	}
	for _, e := range escritas {
		rec := requisicaoV1(&fakeService{}, nil, "viewer", e[0], e[1], e[2])

		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s", e[0], e[1])
		assert.Contains(t, rec.Body.String(), "read_only")
	}
}

func TestRotasDoGrafoSubstituidasPelaListaDoCofre_SaemSemVersaoObsoleta(t *testing.T) {
	for _, rota := range []string{"/vaults/va/orphans", "/vaults/va/hubs"} {
		rec := requisicaoV1(&fakeService{}, &fakeNoteGraph{}, "member", http.MethodGet, rota, "")

		assert.Equal(t, http.StatusNotFound, rec.Code, rota)
	}
}
