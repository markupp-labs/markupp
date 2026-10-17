package api_test

import (
	"context"
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

// fakeNoteGraph devolve listas fixas e guarda o último pedido recebido.
type fakeNoteGraph struct {
	linked     []notes.LinkedNote
	neighbors  []notes.Neighbor
	unresolved []notes.UnresolvedLink
	err        error

	method string
	id     string
	offset int
	limit  int
}

func (f *fakeNoteGraph) record(method, id string, offset, limit int) {
	f.method, f.id, f.offset, f.limit = method, id, offset, limit
}

func (f *fakeNoteGraph) Backlinks(_ context.Context, noteID string, offset, limit int) ([]notes.LinkedNote, error) {
	f.record("backlinks", noteID, offset, limit)
	return f.linked, f.err
}

func (f *fakeNoteGraph) Neighbors(_ context.Context, noteID string, offset, limit int) ([]notes.Neighbor, error) {
	f.record("neighbors", noteID, offset, limit)
	return f.neighbors, f.err
}

func (f *fakeNoteGraph) Orphans(_ context.Context, vaultID string, offset, limit int) ([]notes.LinkedNote, error) {
	f.record("orphans", vaultID, offset, limit)
	return f.linked, f.err
}

func (f *fakeNoteGraph) UnresolvedLinks(_ context.Context, vaultID string, offset, limit int) ([]notes.UnresolvedLink, error) {
	f.record("unresolved", vaultID, offset, limit)
	return f.unresolved, f.err
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

var notaLigada = notes.LinkedNote{ID: "n2", VaultID: "va", Path: "b.md", UpdatedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)}

func TestGetBacklinks_DevolveAsNotasEPaginaNoPadrao(t *testing.T) {
	graph := &fakeNoteGraph{linked: []notes.LinkedNote{notaLigada}}

	rec := getGrafo(t, graph, "viewer", "/notes/n1/backlinks?offset=3&limit=500")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"id":"n2","vault_id":"va","path":"b.md","updated_at":"2026-10-16T12:00:00Z"}]`, rec.Body.String())
	assert.Equal(t, []any{"backlinks", "n1", 3, 100}, []any{graph.method, graph.id, graph.offset, graph.limit}, "limit acima do teto vira 100")
}

func TestGetNeighbors_DizOSentidoDeCadaVizinho(t *testing.T) {
	graph := &fakeNoteGraph{neighbors: []notes.Neighbor{{LinkedNote: notaLigada, Incoming: true, Outgoing: false}}}

	rec := getGrafo(t, graph, "member", "/notes/n1/neighbors")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"id":"n2","vault_id":"va","path":"b.md","updated_at":"2026-10-16T12:00:00Z","incoming":true,"outgoing":false}]`, rec.Body.String())
	assert.Equal(t, []any{"neighbors", "n1", 0, 10}, []any{graph.method, graph.id, graph.offset, graph.limit})
}

func TestGetOrphans_LeOCofreDaRota(t *testing.T) {
	graph := &fakeNoteGraph{}

	rec := getGrafo(t, graph, "viewer", "/vaults/va/orphans")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[]`, rec.Body.String(), "lista vazia sai como array")
	assert.Equal(t, []any{"orphans", "va"}, []any{graph.method, graph.id})
}

func TestGetUnresolvedLinks_DevolveOTituloCitadoEOTipo(t *testing.T) {
	graph := &fakeNoteGraph{unresolved: []notes.UnresolvedLink{{NoteID: "n1", Path: "a.md", Title: "Inexistente", Kind: "explicit"}}}

	rec := getGrafo(t, graph, "member", "/vaults/va/unresolved-links")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[{"note_id":"n1","path":"a.md","title":"Inexistente","kind":"explicit"}]`, rec.Body.String())
	assert.Equal(t, []any{"unresolved", "va"}, []any{graph.method, graph.id})
}

func TestGrafo_ErrosDeDominio_ViramStatus(t *testing.T) {
	casos := map[string]struct {
		err    error
		target string
		status int
	}{
		"nota inexistente":   {notes.ErrNotFound, "/notes/x/backlinks", http.StatusNotFound},
		"cofre sem acesso":   {notes.ErrVaultForbidden, "/vaults/x/orphans", http.StatusForbidden},
		"limit nao inteiro":  {nil, "/vaults/x/unresolved-links?limit=abc", http.StatusBadRequest},
		"offset nao inteiro": {nil, "/notes/x/neighbors?offset=abc", http.StatusBadRequest},
	}
	for nome, caso := range casos {
		rec := getGrafo(t, &fakeNoteGraph{err: caso.err}, "member", caso.target)

		assert.Equal(t, caso.status, rec.Code, "%s: %s", nome, rec.Body.String())
		var corpo map[string]any
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&corpo), nome)
		assert.NotEmpty(t, corpo["error"], nome)
	}
}
