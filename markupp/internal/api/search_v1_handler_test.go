package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

var resultadoDeTeste = notes.SearchResult{ID: "n1", VaultID: "va", Path: "pasta/kubernetes.md", UpdatedAt: momentoDeTeste, Snippet: "o \u0002kubernetes\u0003", Rank: 0.75}

func TestV1SearchNotes_DevolveANotaARelevanciaEOTrecho(t *testing.T) {
	svc := &fakeService{searchResult: []notes.SearchResult{resultadoDeTeste}}

	rec := requisicaoV1(svc, nil, "viewer", http.MethodGet, "/v1/search/notes?q=kubernetes&vault=va&offset=1&limit=5", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"note":{"id":"n1","title":"kubernetes","path":"pasta/kubernetes.md"},"vault_id":"va","rank":0.75,
		"snippet":"o \u0002kubernetes\u0003"}]`, rec.Body.String())
	assert.Equal(t, []any{"kubernetes", "va", 1, 5}, []any{svc.searchArgs.query, svc.searchArgs.vaultID, svc.searchArgs.offset, svc.searchArgs.limit})
}

func TestV1SearchNotes_SemCofre_BuscaEmTodosESemResultadoDevolveArray(t *testing.T) {
	svc := &fakeService{}

	rec := requisicaoV1(svc, nil, "member", http.MethodGet, "/v1/search/notes?q=go", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[]`, rec.Body.String())
	assert.Empty(t, svc.searchArgs.vaultID)
}

func TestV1SearchNotes_SemQ_Retorna400(t *testing.T) {
	svc := &fakeService{}

	rec := requisicaoV1(svc, nil, "member", http.MethodGet, "/v1/search/notes?query=go", "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var corpo map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&corpo))
	assert.Equal(t, "invalid_request", corpo["error"])
	assert.Contains(t, corpo["message"], "q")
	assert.False(t, svc.searchArgs.called)
}

func TestV1SearchNotes_CofreSemAcesso_Retorna403(t *testing.T) {
	rec := requisicaoV1(&fakeService{searchErr: notes.ErrVaultForbidden}, nil, "member", http.MethodGet, "/v1/search/notes?q=go&vault=x", "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSearchNotesSemPrefixo_MantemARespostaSemRelevanciaEBuscaEmTodosOsCofres(t *testing.T) {
	svc := &fakeService{searchResult: []notes.SearchResult{resultadoDeTeste}}

	rec := requisicaoV1(svc, nil, "member", http.MethodGet, "/notes/search?query=kubernetes&vault=va", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"id":"n1","vault_id":"va","path":"pasta/kubernetes.md","updated_at":"2026-10-16T12:00:00Z",
		"snippet":"o \u0002kubernetes\u0003"}]`, rec.Body.String())
	assert.Empty(t, svc.searchArgs.vaultID, "a rota sem prefixo nao filtra por cofre")
}
