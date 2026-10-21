package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

func (f *fakeAdminRepository) DeleteTenantTx(ctx context.Context, event auth.AuditEventRecord) error {
	f.tenantDeleted, f.event = true, event
	return nil
}

func TestDeleteTenant_Owner_ApagaEAuditaQuemEQualTenant(t *testing.T) {
	repo := &fakeAdminRepository{}
	clock := defaultClock()
	dona := identity.Actor{UserID: "dona", TenantID: "t1", Role: identity.RoleOwner}

	require.NoError(t, auth.NewAdminService(repo, clock, false).DeleteTenant(context.Background(), dona))

	assert.True(t, repo.tenantDeleted)
	assert.Equal(t, auth.AuditEventRecord{
		ID: repo.event.ID, TenantID: "t1", ActorID: "dona", Action: "tenants.delete",
		Target: "t1", Result: "success", OccurredAt: clock(),
	}, repo.event)
	assert.NotEmpty(t, repo.event.ID)
}

func TestDeleteTenant_PapelAbaixoDeOwner_RecusaSemApagar(t *testing.T) {
	for _, papel := range []string{identity.RoleAdmin, identity.RoleMember, identity.RoleViewer} {
		repo := &fakeAdminRepository{}
		ator := identity.Actor{UserID: "u1", TenantID: "t1", Role: papel}

		err := auth.NewAdminService(repo, defaultClock(), false).DeleteTenant(context.Background(), ator)

		require.ErrorIs(t, err, auth.ErrNotOwner, papel)
		assert.Contains(t, err.Error(), `"`+papel+`"`)
		assert.False(t, repo.tenantDeleted, papel)
	}
}
