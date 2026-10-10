package api

import (
	"net/http"
	"strings"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/tenant"
)

const (
	headerTenantID = "X-Tenant-ID"
	headerUserID   = "X-User-ID"
	headerUserRole = "X-User-Role"
)

// TenantMiddleware extrai as informações de tenant e usuário dos cabeçalhos HTTP
// e as injeta no contexto da requisição. Se omitido, adota defaultTenantID.
func TenantMiddleware(defaultTenantID string) func(http.Handler) http.Handler {
	if defaultTenantID == "" {
		defaultTenantID = "default"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := strings.TrimSpace(r.Header.Get(headerTenantID))
			if tenantID == "" {
				tenantID = defaultTenantID
			}

			userID := strings.TrimSpace(r.Header.Get(headerUserID))
			role := strings.TrimSpace(r.Header.Get(headerUserRole))
			if role == "" {
				role = "member"
			}

			ctx := tenant.WithContext(r.Context(), tenant.Context{
				TenantID: tenantID,
				UserID:   userID,
				Role:     role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin restringe o endpoint apenas para requisições com perfil de administrador.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := tenant.FromContext(r.Context())
		if !ok || tc.Role != "admin" {
			writeError(w, "forbidden", "acesso restrito a administradores", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
