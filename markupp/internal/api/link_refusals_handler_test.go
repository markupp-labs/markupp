package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/linkrefusals"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// fakeLinkRefusalService registra quem pediu o quê e devolve err em toda chamada.
type fakeLinkRefusalService struct {
	actor identity.Actor
	args  []string
	page  notes.Page
	err   error
}

var recusaDeTeste = linkrefusals.Refusal{
	ID:        "r1",
	Notes:     [2]notes.NoteSummary{{ID: "n1", Title: "a", Path: "a.md"}, {ID: "n2", Title: "b", Path: "b.md"}},
	CreatedAt: momentoDeTeste,
}

const recusaDeTesteJSON = `{"id":"r1","notes":[{"id":"n1","title":"a","path":"a.md"},{"id":"n2","title":"b","path":"b.md"}],
	"created_at":"2026-10-16T12:00:00Z"}`

func (f *fakeLinkRefusalService) Refuse(_ context.Context, actor identity.Actor, vaultID, noteA, noteB string) (linkrefusals.Refusal, error) {
	f.actor, f.args = actor, []string{vaultID, noteA, noteB}
	return recusaDeTeste, f.err
}

func (f *fakeLinkRefusalService) List(_ context.Context, vaultID string, p notes.Page) ([]linkrefusals.Refusal, error) {
	f.args, f.page = []string{vaultID}, p
	return []linkrefusals.Refusal{recusaDeTeste}, f.err
}

func (f *fakeLinkRefusalService) Undo(_ context.Context, actor identity.Actor, vaultID, id string) error {
	f.actor, f.args = actor, []string{vaultID, id}
	return f.err
}

func recusasRequest(svc api.LinkRefusalService, method, path, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		LinkRefusals: svc, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: "member"}},
	})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestV1PostLinkRefusals_RecusaOParComOAtor(t *testing.T) {
	svc := &fakeLinkRefusalService{}

	rec := recusasRequest(svc, http.MethodPost, "/v1/vaults/v1/link-refusals", `{"note_ids":["n2","n1"]}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.JSONEq(t, recusaDeTesteJSON, rec.Body.String())
	assert.Equal(t, []string{"v1", "n2", "n1"}, svc.args)
	assert.Equal(t, identity.Actor{UserID: "u1", TenantID: "t1", Role: "member"}, svc.actor)
}

func TestV1PostLinkRefusals_CorpoSemDuasNotas_Retorna400(t *testing.T) {
	for _, corpo := range []string{`{"note_ids":["n1"]}`, `{"note_ids":["n1","n2","n3"]}`, `{}`, `nao e json`} {
		rec := recusasRequest(&fakeLinkRefusalService{}, http.MethodPost, "/v1/vaults/v1/link-refusals", corpo)

		assert.Equal(t, http.StatusBadRequest, rec.Code, corpo)
		assert.Contains(t, rec.Body.String(), "invalid_request", corpo)
	}
}

func TestV1GetLinkRefusals_ListaPaginada(t *testing.T) {
	svc := &fakeLinkRefusalService{}

	rec := recusasRequest(svc, http.MethodGet, "/v1/vaults/v1/link-refusals?offset=2&limit=5", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, "["+recusaDeTesteJSON+"]", rec.Body.String())
	assert.Equal(t, []string{"v1"}, svc.args)
	assert.Equal(t, notes.Page{Offset: 2, Limit: 5}, svc.page)
}

func TestV1DeleteLinkRefusal_Desfaz(t *testing.T) {
	svc := &fakeLinkRefusalService{}

	rec := recusasRequest(svc, http.MethodDelete, "/v1/vaults/v1/link-refusals/r1", "")

	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"v1", "r1"}, svc.args)
}

func TestLinkRefusals_ErrosDoServicoViramStatus(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{linkrefusals.ErrReadOnly, http.StatusForbidden, "read_only"},
		{linkrefusals.ErrSameNote, http.StatusBadRequest, "same_note"},
		{linkrefusals.ErrNoteOutsideVault, http.StatusUnprocessableEntity, "note_outside_vault"},
		{linkrefusals.ErrRefusalNotFound, http.StatusNotFound, "link_refusal_not_found"},
		{notes.ErrVaultForbidden, http.StatusForbidden, "vault_forbidden"},
	}
	for _, c := range casos {
		svc := &fakeLinkRefusalService{err: fmt.Errorf("embrulhado: %w", c.err)}

		rec := recusasRequest(svc, http.MethodDelete, "/v1/vaults/v1/link-refusals/r1", "")

		assert.Equal(t, c.status, rec.Code, c.code)
		assert.Contains(t, rec.Body.String(), `"`+c.code+`"`)
	}
}

func TestLinkRefusals_SoSobV1(t *testing.T) {
	rec := recusasRequest(&fakeLinkRefusalService{}, http.MethodGet, "/vaults/v1/link-refusals", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
