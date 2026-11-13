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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

// fakePublicationService guarda o que a rota pediu e devolve o erro configurado.
type fakePublicationService struct {
	err       error
	actor     identity.Actor
	vaultID   string
	target    publication.Target
	request   publication.Request
	id        string
	indexable *bool
}

var publicacaoDaNota = publication.Publication{
	ID: "p1", VaultID: "va", VaultName: "Docs", Kind: "note", NoteID: "n1", TargetName: "Guia de Deploy",
	NoteCount: 1, PublishedByID: "usuario-teste", PublishedByName: "eu@markupp.dev",
	PublishedAt: time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC), PublicID: "Ab3dEf6hIj9k",
}

const publicacaoDaNotaJSON = `{"id":"p1","vault_id":"va","vault_name":"Docs","kind":"note","note_id":"n1","folder_path":null,
	"target_name":"Guia de Deploy","note_count":1,"indexable":false,"published_by":{"id":"usuario-teste","name":"eu@markupp.dev"},
	"published_at":"2026-11-12T09:00:00Z","public_id":"Ab3dEf6hIj9k","title_slug":"guia-de-deploy"}`

func (f *fakePublicationService) Preview(_ context.Context, vaultID string, t publication.Target) (publication.Preview, error) {
	f.vaultID, f.target = vaultID, t
	return publication.Preview{Kind: t.Kind, TargetName: "docs", NoteCount: 4, IncludesFuture: true}, f.err
}

func (f *fakePublicationService) Publish(_ context.Context, actor identity.Actor, vaultID string, req publication.Request) (publication.Publication, error) {
	f.actor, f.vaultID, f.request = actor, vaultID, req
	return publicacaoDaNota, f.err
}

func (f *fakePublicationService) List(_ context.Context, vaultID string) ([]publication.Publication, error) {
	f.vaultID = vaultID
	return []publication.Publication{publicacaoDaNota}, f.err
}

func (f *fakePublicationService) SetIndexable(_ context.Context, actor identity.Actor, id string, indexable bool) (publication.Publication, error) {
	f.actor, f.id, f.indexable = actor, id, &indexable
	return publicacaoDaNota, f.err
}

func (f *fakePublicationService) Unpublish(_ context.Context, actor identity.Actor, id string) error {
	f.actor, f.id = actor, id
	return f.err
}

// pedePublicacoes faz method em target com o papel dado no tenant t1.
func pedePublicacoes(svc api.PublicationService, role, method, target, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Publications: svc, Probe: &fakeProbe{},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: role}},
		Tokens:  &fakeTokenValidator{claims: auth.Claims{Subject: "usuario-teste"}},
	})
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPublicar_Nota_Devolve201ComIDPublicoESlug(t *testing.T) {
	svc := &fakePublicationService{}

	rec := pedePublicacoes(svc, "member", http.MethodPost, "/v1/vaults/va/publications",
		`{"target":{"kind":"note","note_id":"n1"},"confirm":"Guia de Deploy","indexable":true}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.JSONEq(t, publicacaoDaNotaJSON, rec.Body.String())
	assert.Equal(t, "va", svc.vaultID)
	assert.Equal(t, publication.Request{Target: publication.Target{Kind: "note", NoteID: "n1"}, Confirm: "Guia de Deploy", Indexable: true}, svc.request)
	assert.Equal(t, identity.Actor{UserID: "usuario-teste", TenantID: "t1", Role: "member"}, svc.actor)
}

func TestPublicar_Pasta_LeOCaminhoDoAlvo(t *testing.T) {
	svc := &fakePublicationService{}

	rec := pedePublicacoes(svc, "admin", http.MethodPost, "/v1/vaults/va/publications", `{"target":{"kind":"folder","path":"docs/deploy"},"confirm":"docs/deploy"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, publication.Target{Kind: "folder", FolderPath: "docs/deploy"}, svc.request.Target)
}

func TestPublicar_ErrosDoServico_StatusECodigoDoContrato(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: kind %q", publication.ErrInvalidTarget, "x"), http.StatusBadRequest, "invalid_request"},
		{publication.ErrConfirmationMismatch, http.StatusUnprocessableEntity, "confirmation_mismatch"},
		{notes.ErrVaultForbidden, http.StatusForbidden, "vault_forbidden"},
		{publication.ErrPublishForbidden, http.StatusForbidden, "publish_forbidden"},
		{publication.ErrNoteNotFound, http.StatusNotFound, "note_not_found"},
		{publication.ErrFolderNotFound, http.StatusNotFound, "folder_not_found"},
	}
	for _, c := range casos {
		rec := pedePublicacoes(&fakePublicationService{err: c.err}, "owner", http.MethodPost, "/v1/vaults/va/publications", `{"target":{"kind":"vault"},"confirm":"Docs"}`)

		assert.Equal(t, c.status, rec.Code, c.code)
		assert.Contains(t, rec.Body.String(), `"error":"`+c.code+`"`)
	}
}

func TestPublicar_JaPublicado_409ComOIDDaPublicacaoExistente(t *testing.T) {
	svc := &fakePublicationService{err: &publication.AlreadyPublishedError{PublicationID: "p-velha"}}

	rec := pedePublicacoes(svc, "owner", http.MethodPost, "/v1/vaults/va/publications", `{"target":{"kind":"vault"},"confirm":"Docs"}`)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"already_published"`)
	assert.Contains(t, rec.Body.String(), `"publication_id":"p-velha"`)
}

func TestPublicar_CorpoInvalido_400(t *testing.T) {
	rec := pedePublicacoes(&fakePublicationService{}, "owner", http.MethodPost, "/v1/vaults/va/publications", `{"target":`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPrevia_LeOAlvoDaQueryString(t *testing.T) {
	svc := &fakePublicationService{}

	rec := pedePublicacoes(svc, "viewer", http.MethodGet, "/v1/vaults/va/publications/preview?kind=folder&path=docs", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"kind":"folder","target_name":"docs","note_count":4,"includes_future":true}`, rec.Body.String())
	assert.Equal(t, publication.Target{Kind: "folder", FolderPath: "docs"}, svc.target)
}

func TestListarPublicacoes_ItensETotalFiltradosPeloCofre(t *testing.T) {
	svc := &fakePublicationService{}

	rec := pedePublicacoes(svc, "viewer", http.MethodGet, "/v1/publications?vault_id=va", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"items":[`+publicacaoDaNotaJSON+`],"total":1}`, rec.Body.String())
	assert.Equal(t, "va", svc.vaultID)
}

func TestIndexacao_PatchTrocaODaPublicacao(t *testing.T) {
	svc := &fakePublicationService{}

	rec := pedePublicacoes(svc, "owner", http.MethodPatch, "/v1/publications/p1", `{"indexable":true}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "p1", svc.id)
	require.NotNil(t, svc.indexable)
	assert.True(t, *svc.indexable)
}

func TestIndexacao_SemOCampo_400ComOFormatoEsperado(t *testing.T) {
	rec := pedePublicacoes(&fakePublicationService{}, "owner", http.MethodPatch, "/v1/publications/p1", `{}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "indexable")
}

func TestDespublicar_204EErrosDoContrato(t *testing.T) {
	svc := &fakePublicationService{}
	rec := pedePublicacoes(svc, "member", http.MethodDelete, "/v1/publications/p1", "")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "p1", svc.id)

	rec = pedePublicacoes(&fakePublicationService{err: publication.ErrPublicationNotFound}, "member", http.MethodDelete, "/v1/publications/p9", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"publication_not_found"`)

	rec = pedePublicacoes(&fakePublicationService{err: publication.ErrPublishForbidden}, "member", http.MethodDelete, "/v1/publications/p1", "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPublicacoes_SoSobV1(t *testing.T) {
	rec := pedePublicacoes(&fakePublicationService{}, "owner", http.MethodGet, "/publications", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
