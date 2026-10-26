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

// fakeVaultPublisher guarda o pedido de publicação e devolve o erro configurado.
type fakeVaultPublisher struct {
	err     error
	actor   identity.Actor
	vaultID string
	slug    string
	title   string
	removed string
}

var publicacaoDeTeste = publication.Publication{
	VaultID: "va", Slug: "docs", Title: "Docs", PublishedBy: "usuario-teste",
	PublishedAt: time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
}

const publicacaoDeTesteJSON = `{"vault_id":"va","slug":"docs","title":"Docs","published_by":"usuario-teste","published_at":"2026-10-25T09:00:00Z"}`

func (f *fakeVaultPublisher) Publish(_ context.Context, actor identity.Actor, vaultID, slug, title string) (publication.Publication, error) {
	f.actor, f.vaultID, f.slug, f.title = actor, vaultID, slug, title
	return publicacaoDeTeste, f.err
}

func (f *fakeVaultPublisher) Unpublish(_ context.Context, actor identity.Actor, vaultID string) error {
	f.actor, f.removed = actor, vaultID
	return f.err
}

func (f *fakeVaultPublisher) Publication(_ context.Context, vaultID string) (publication.Publication, error) {
	f.vaultID = vaultID
	return publicacaoDeTeste, f.err
}

// pedePublicacao faz method em /v1/vaults/va/publication com o papel dado no tenant.
func pedePublicacao(t *testing.T, svc api.VaultPublisher, role, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := api.NewRouter(api.RouterDeps{
		Publications: svc, Probe: &fakeProbe{},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: role}},
		Tokens:  &fakeTokenValidator{claims: auth.Claims{Subject: "usuario-teste"}},
	})
	req := httptest.NewRequest(method, "/v1/vaults/va/publication", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPublicacao_AdminPublica_DevolveAPublicacao(t *testing.T) {
	for _, papel := range []string{"owner", "admin"} {
		svc := &fakeVaultPublisher{}

		rec := pedePublicacao(t, svc, papel, http.MethodPut, `{"slug":"docs","title":"Docs"}`)

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, publicacaoDeTesteJSON, rec.Body.String())
		assert.Equal(t, identity.Actor{UserID: "usuario-teste", TenantID: "t1", Role: papel}, svc.actor)
		assert.Equal(t, []string{"va", "docs", "Docs"}, []string{svc.vaultID, svc.slug, svc.title})
	}
}

func TestPublicacao_MembroOuViewer_Responde403SemChegarAoServico(t *testing.T) {
	for _, papel := range []string{"member", "viewer"} {
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			svc := &fakeVaultPublisher{}

			rec := pedePublicacao(t, svc, papel, method, `{"slug":"docs"}`)

			assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s", papel, method)
			assert.Empty(t, svc.vaultID+svc.removed, "%s %s nao chega ao servico", papel, method)
		}
	}
}

func TestPublicacao_SlugRepetido_Responde409(t *testing.T) {
	svc := &fakeVaultPublisher{err: fmt.Errorf("%w: %q", publication.ErrSlugTaken, "docs")}

	rec := pedePublicacao(t, svc, "admin", http.MethodPut, `{"slug":"docs"}`)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"slug_taken"`)
}

func TestPublicacao_ErrosDoServico_ViramStatus(t *testing.T) {
	casos := map[error]int{
		fmt.Errorf("%w: %q", publication.ErrInvalidSlug, "A"): http.StatusBadRequest,
		publication.ErrInvalidTitle:                           http.StatusBadRequest,
		notes.ErrVaultForbidden:                               http.StatusForbidden,
		publication.ErrNotAdmin:                               http.StatusForbidden,
		fmt.Errorf("banco fora"):                              http.StatusInternalServerError,
	}
	for err, status := range casos {
		rec := pedePublicacao(t, &fakeVaultPublisher{err: err}, "admin", http.MethodPut, `{"slug":"docs"}`)

		assert.Equal(t, status, rec.Code, err.Error())
	}
}

func TestPublicacao_CorpoInvalido_Responde400(t *testing.T) {
	rec := pedePublicacao(t, &fakeVaultPublisher{}, "admin", http.MethodPut, `{`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPublicacao_AdminDespublica_Responde204(t *testing.T) {
	svc := &fakeVaultPublisher{}

	rec := pedePublicacao(t, svc, "admin", http.MethodDelete, "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "va", svc.removed)
}

func TestPublicacao_DespublicarNaoPublicado_Responde404(t *testing.T) {
	svc := &fakeVaultPublisher{err: publication.ErrNotPublished}

	rec := pedePublicacao(t, svc, "admin", http.MethodDelete, "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"not_published"`)
}

func TestPublicacao_DespublicarCofreForaDoAlcance_Responde403(t *testing.T) {
	rec := pedePublicacao(t, &fakeVaultPublisher{err: notes.ErrVaultForbidden}, "owner", http.MethodDelete, "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"vault_forbidden"`)
}

func TestPublicacao_LerCofreForaDoAlcance_Responde403(t *testing.T) {
	rec := pedePublicacao(t, &fakeVaultPublisher{err: notes.ErrVaultForbidden}, "admin", http.MethodGet, "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"vault_forbidden"`)
}

func TestPublicacao_ViewerLe_DevolveAPublicacao(t *testing.T) {
	svc := &fakeVaultPublisher{}

	rec := pedePublicacao(t, svc, "viewer", http.MethodGet, "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, publicacaoDeTesteJSON, rec.Body.String())
	assert.Equal(t, "va", svc.vaultID)
}

func TestPublicacao_LerNaoPublicado_Responde404(t *testing.T) {
	rec := pedePublicacao(t, &fakeVaultPublisher{err: publication.ErrNotPublished}, "member", http.MethodGet, "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPublicacao_SemToken_Responde401(t *testing.T) {
	router := api.NewRouter(api.RouterDeps{
		Publications: &fakeVaultPublisher{}, Tenants: &fakeTenantResolver{},
		Tokens: &fakeTokenValidator{err: auth.ErrInvalidToken},
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/vaults/va/publication", strings.NewReader(`{"slug":"docs"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPublicacao_SoSobV1(t *testing.T) {
	router := api.NewRouter(api.RouterDeps{
		Publications: &fakeVaultPublisher{}, Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: "admin"}},
		Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: "usuario-teste"}},
	})
	req := httptest.NewRequest(http.MethodGet, "/vaults/va/publication", nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
