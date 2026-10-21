package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

func (f *fakeAdminService) DeleteTenant(ctx context.Context, actor identity.Actor) error {
	f.tenantDeletedBy = &actor
	return f.deleteTenantErr
}

// deleteTenant manda DELETE /tenants/{tenantID} com outro tenant no cabeçalho, para
// provar que vale o do caminho.
func deleteTenant(svc api.AdminService, resolver *fakeTenantResolver, tenantID string) *httptest.ResponseRecorder {
	validator := &fakeTokenValidator{claims: auth.Claims{Subject: "u1"}}
	router := api.NewRouter(api.RouterDeps{Admin: svc, Tokens: validator, Tenants: resolver})
	req := httptest.NewRequest(http.MethodDelete, "/tenants/"+tenantID, nil)
	req.Header.Set("Authorization", "Bearer token-valido")
	req.Header.Set(api.TenantHeader, "tenant-do-cabecalho")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestDeleteTenant_Owner_Retorna204NoTenantDoCaminho(t *testing.T) {
	svc := &fakeAdminService{}
	resolver := &fakeTenantResolver{membership: auth.Membership{TenantID: "t9", Role: "owner"}}

	rec := deleteTenant(svc, resolver, "t9")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "t9", resolver.requested)
	require.NotNil(t, svc.tenantDeletedBy)
	assert.Equal(t, identity.Actor{UserID: "u1", TenantID: "t9", Role: "owner"}, *svc.tenantDeletedBy)
}

func TestDeleteTenant_PapelAbaixoDeOwner_Retorna403(t *testing.T) {
	for _, papel := range []string{"admin", "member", "viewer"} {
		svc := &fakeAdminService{deleteTenantErr: auth.ErrNotOwner}
		resolver := &fakeTenantResolver{membership: auth.Membership{TenantID: "t9", Role: papel}}

		rec := deleteTenant(svc, resolver, "t9")

		assert.Equal(t, http.StatusForbidden, rec.Code, papel)
		assert.Contains(t, rec.Body.String(), `"forbidden"`, papel)
	}
}

func TestDeleteTenant_SemVinculo_Retorna404SemRevelarOTenant(t *testing.T) {
	svc := &fakeAdminService{}
	resolver := &fakeTenantResolver{err: auth.ErrMembershipNotFound}

	rec := deleteTenant(svc, resolver, "t-alheio")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"tenant_not_found"`)
	assert.NotContains(t, rec.Body.String(), "t-alheio")
	assert.Nil(t, svc.tenantDeletedBy)
}

func TestDeleteTenant_ApagadoNoMeioDoCaminho_Retorna404(t *testing.T) {
	svc := &fakeAdminService{deleteTenantErr: auth.ErrMembershipNotFound}
	resolver := &fakeTenantResolver{membership: auth.Membership{TenantID: "t9", Role: "owner"}}

	rec := deleteTenant(svc, resolver, "t9")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"tenant_not_found"`)
}
