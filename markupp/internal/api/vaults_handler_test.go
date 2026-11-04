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
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/vaults"
)

// fakeVaultService registra quem pediu o quê e devolve err em toda chamada.
type fakeVaultService struct {
	actor identity.Actor
	args  []string
	err   error
}

var (
	cofreDeTeste  = vaults.Vault{ID: "v1", Name: "Time", CreatedAt: momentoDeTeste}
	membroDeTeste = vaults.Member{UserID: "u2", Email: "b@markupp.dev", Role: "member", JoinedAt: momentoDeTeste}
)

const (
	cofreDeTesteJSON  = `{"id":"v1","name":"Time","created_at":"2026-10-16T12:00:00Z"}`
	membroDeTesteJSON = `{"id":"u2","email":"b@markupp.dev","role":"member","joined_at":"2026-10-16T12:00:00Z"}`
)

func (f *fakeVaultService) List(ctx context.Context) ([]vaults.Vault, error) {
	return []vaults.Vault{cofreDeTeste}, f.err
}

func (f *fakeVaultService) Create(ctx context.Context, actor identity.Actor, name string) (vaults.Vault, error) {
	f.actor, f.args = actor, []string{name}
	return cofreDeTeste, f.err
}

func (f *fakeVaultService) Rename(ctx context.Context, actor identity.Actor, id, name string) (vaults.Vault, error) {
	f.actor, f.args = actor, []string{id, name}
	return cofreDeTeste, f.err
}

func (f *fakeVaultService) Delete(ctx context.Context, actor identity.Actor, id string) error {
	f.actor, f.args = actor, []string{id}
	return f.err
}

func (f *fakeVaultService) ListMembers(ctx context.Context, vaultID string) ([]vaults.Member, error) {
	f.args = []string{vaultID}
	return []vaults.Member{membroDeTeste}, f.err
}

func (f *fakeVaultService) AddMember(ctx context.Context, actor identity.Actor, vaultID, userID string) (vaults.Member, error) {
	f.actor, f.args = actor, []string{vaultID, userID}
	return membroDeTeste, f.err
}

func (f *fakeVaultService) RemoveMember(ctx context.Context, actor identity.Actor, vaultID, userID string) error {
	f.actor, f.args = actor, []string{vaultID, userID}
	return f.err
}

func vaultsRequest(svc api.VaultService, method, path, body string) *httptest.ResponseRecorder {
	router := api.NewRouter(api.RouterDeps{
		Vaults: svc, Tokens: &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}},
		Tenants: &fakeTenantResolver{membership: auth.Membership{TenantID: "t1", Role: "member"}},
	})
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer token-valido")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

var atorDeTeste = identity.Actor{UserID: "u1", TenantID: "t1", Role: "member"}

func TestVaultsRoutes_List_DevolveArrayNu(t *testing.T) {
	rec := vaultsRequest(&fakeVaultService{}, http.MethodGet, "/v1/vaults", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[`+cofreDeTesteJSON+`]`, rec.Body.String())
}

func TestVaultsRoutes_Create_Devolve201ComOCofre(t *testing.T) {
	svc := &fakeVaultService{}

	rec := vaultsRequest(svc, http.MethodPost, "/v1/vaults", `{"name":"Time"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.JSONEq(t, cofreDeTesteJSON, rec.Body.String())
	assert.Equal(t, atorDeTeste, svc.actor)
	assert.Equal(t, []string{"Time"}, svc.args)
}

func TestVaultsRoutes_Rename_Devolve200ComOCofre(t *testing.T) {
	svc := &fakeVaultService{}

	rec := vaultsRequest(svc, http.MethodPatch, "/v1/vaults/v1", `{"name":"Time"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, cofreDeTesteJSON, rec.Body.String())
	assert.Equal(t, []string{"v1", "Time"}, svc.args)
}

func TestVaultsRoutes_Delete_Devolve204(t *testing.T) {
	svc := &fakeVaultService{}

	rec := vaultsRequest(svc, http.MethodDelete, "/v1/vaults/v1", "")

	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"v1"}, svc.args)
}

func TestVaultsRoutes_ListMembers_DevolveArrayNu(t *testing.T) {
	svc := &fakeVaultService{}

	rec := vaultsRequest(svc, http.MethodGet, "/v1/vaults/v1/members", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[`+membroDeTesteJSON+`]`, rec.Body.String())
	assert.Equal(t, []string{"v1"}, svc.args)
}

func TestVaultsRoutes_AddMember_Devolve201ComOMembro(t *testing.T) {
	svc := &fakeVaultService{}

	rec := vaultsRequest(svc, http.MethodPost, "/v1/vaults/v1/members", `{"user_id":"u2"}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.JSONEq(t, membroDeTesteJSON, rec.Body.String())
	assert.Equal(t, atorDeTeste, svc.actor)
	assert.Equal(t, []string{"v1", "u2"}, svc.args)
}

func TestVaultsRoutes_RemoveMember_Devolve204(t *testing.T) {
	svc := &fakeVaultService{}

	rec := vaultsRequest(svc, http.MethodDelete, "/v1/vaults/v1/members/u2", "")

	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"v1", "u2"}, svc.args)
}

func TestVaultsRoutes_CorpoInvalido_Retorna400(t *testing.T) {
	rec := vaultsRequest(&fakeVaultService{}, http.MethodPost, "/v1/vaults", `{`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_request"`)
}

func TestVaultsRoutes_ErrosDeDominio_SeguemOContrato(t *testing.T) {
	casos := []struct {
		err    error
		status int
		code   string
	}{
		{vaults.ErrInvalidName, http.StatusBadRequest, "invalid_name"},
		{vaults.ErrReadOnly, http.StatusForbidden, "read_only"},
		{vaults.ErrVaultNotFound, http.StatusForbidden, "vault_forbidden"},
		{vaults.ErrNotTenantMember, http.StatusUnprocessableEntity, "not_tenant_member"},
		{vaults.ErrAlreadyVaultMember, http.StatusConflict, "already_vault_member"},
		{vaults.ErrMemberNotFound, http.StatusNotFound, "vault_member_not_found"},
		{fmt.Errorf("banco fora"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range casos {
		svc := &fakeVaultService{err: fmt.Errorf("embrulhado: %w", c.err)}

		rec := vaultsRequest(svc, http.MethodPost, "/v1/vaults/v1/members", `{"user_id":"u2"}`)

		assert.Equal(t, c.status, rec.Code, c.err.Error())
		assert.Contains(t, rec.Body.String(), `"error":"`+c.code+`"`, c.err.Error())
	}
}

func TestVaultsRoutes_ErroNaLeitura_TambemSegueOContrato(t *testing.T) {
	rec := vaultsRequest(&fakeVaultService{err: vaults.ErrVaultNotFound}, http.MethodGet, "/v1/vaults/v9/members", "")

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"vault_forbidden"`)
}

func TestVaultsRoutes_SoExistemSobV1(t *testing.T) {
	rec := vaultsRequest(&fakeVaultService{}, http.MethodGet, "/vaults", "")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
