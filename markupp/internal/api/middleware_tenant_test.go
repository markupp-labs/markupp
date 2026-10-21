package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

type fakeTenantResolver struct {
	membership auth.Membership
	err        error
	requested  string
}

func (f *fakeTenantResolver) ResolveTenant(ctx context.Context, userID, requested string) (auth.Membership, error) {
	f.requested = requested
	return f.membership, f.err
}

var tenantDeTeste = &fakeTenantResolver{membership: auth.Membership{TenantID: "tenant-default", Role: "member"}}

func serveWithTenant(resolver api.TenantResolver, header string) (*httptest.ResponseRecorder, identity.Actor) {
	var ator identity.Actor
	handler := api.RequireTenant(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ator, _ = identity.ActorFrom(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req = req.WithContext(identity.WithActor(req.Context(), identity.Actor{UserID: "u1"}))
	if header != "" {
		req.Header.Set("X-Markupp-Tenant", header)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec, ator
}

func TestRequireTenant_CabecalhoComVinculo_AtorGanhaTenantEPapelDoBanco(t *testing.T) {
	resolver := &fakeTenantResolver{membership: auth.Membership{TenantID: "t2", Role: "viewer"}}

	rec, ator := serveWithTenant(resolver, "t2")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "t2", resolver.requested)
	assert.Equal(t, identity.Actor{UserID: "u1", TenantID: "t2", Role: "viewer"}, ator)
}

func TestRequireTenant_ErrosDeVinculo(t *testing.T) {
	casos := map[error]struct {
		status int
		codigo string
	}{
		auth.ErrMembershipNotFound: {http.StatusForbidden, "tenant_forbidden"},
		auth.ErrTenantRequired:     {http.StatusBadRequest, "tenant_required"},
		auth.ErrNoTenant:           {http.StatusForbidden, "no_tenant"},
	}
	for errVinculo, esperado := range casos {
		rec, _ := serveWithTenant(&fakeTenantResolver{err: errVinculo}, "")

		assert.Equal(t, esperado.status, rec.Code, errVinculo.Error())
		assert.Contains(t, rec.Body.String(), esperado.codigo)
	}
}
