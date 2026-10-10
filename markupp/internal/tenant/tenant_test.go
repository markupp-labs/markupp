package tenant_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/tenant"
)

func TestTenantContext_WithEFromContext_Sucesso(t *testing.T) {
	ctx := context.Background()
	tc := tenant.Context{
		TenantID: "tenant-abc",
		UserID:   "user-123",
		Role:     "admin",
	}

	ctxComTenant := tenant.WithContext(ctx, tc)
	recuperado, ok := tenant.FromContext(ctxComTenant)

	assert.True(t, ok)
	assert.Equal(t, "tenant-abc", recuperado.TenantID)
	assert.Equal(t, "user-123", recuperado.UserID)
	assert.Equal(t, "admin", recuperado.Role)
}

func TestTenantContext_FromContext_VazioRetornaFalso(t *testing.T) {
	ctx := context.Background()
	_, ok := tenant.FromContext(ctx)
	assert.False(t, ok)
}

func TestTenantContext_IDFromContext_UsaFallbackQuandoAusente(t *testing.T) {
	ctx := context.Background()
	tid := tenant.IDFromContext(ctx, "default")
	assert.Equal(t, "default", tid)

	ctxComTenant := tenant.WithContext(ctx, tenant.Context{TenantID: "custom-tenant"})
	tid = tenant.IDFromContext(ctxComTenant, "default")
	assert.Equal(t, "custom-tenant", tid)
}
