package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/api"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/tenant"
)

func TestTenantMiddleware_InjetaCabecalhosNoContexto(t *testing.T) {
	var captured tenant.Context
	handler := api.TenantMiddleware("default-tenant")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := tenant.FromContext(r.Context())
		require.True(t, ok)
		captured = tc
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req.Header.Set("X-Tenant-ID", "custom-tenant")
	req.Header.Set("X-User-ID", "user-42")
	req.Header.Set("X-User-Role", "admin")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "custom-tenant", captured.TenantID)
	assert.Equal(t, "user-42", captured.UserID)
	assert.Equal(t, "admin", captured.Role)
}

func TestTenantMiddleware_SemCabecalho_UsaFallback(t *testing.T) {
	var captured tenant.Context
	handler := api.TenantMiddleware("meu-default")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := tenant.FromContext(r.Context())
		require.True(t, ok)
		captured = tc
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "meu-default", captured.TenantID)
	assert.Equal(t, "member", captured.Role)
}

func TestRequireAdmin_AdminPermite_MembroBloqueia(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	protected := api.RequireAdmin(nextHandler)

	// Admin tem sucesso
	reqAdmin := httptest.NewRequest(http.MethodGet, "/users", nil)
	ctxAdmin := tenant.WithContext(reqAdmin.Context(), tenant.Context{Role: "admin"})
	recAdmin := httptest.NewRecorder()
	protected.ServeHTTP(recAdmin, reqAdmin.WithContext(ctxAdmin))
	assert.Equal(t, http.StatusOK, recAdmin.Code)

	// Membro toma 403 Forbidden
	reqMember := httptest.NewRequest(http.MethodGet, "/users", nil)
	ctxMember := tenant.WithContext(reqMember.Context(), tenant.Context{Role: "member"})
	recMember := httptest.NewRecorder()
	protected.ServeHTTP(recMember, reqMember.WithContext(ctxMember))
	assert.Equal(t, http.StatusForbidden, recMember.Code)
}
