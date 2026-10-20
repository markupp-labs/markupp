package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// TenantHeader é onde o cliente diz em qual dos seus tenants a requisição age.
const TenantHeader = "X-Markupp-Tenant"

// TenantResolver confere o vínculo do usuário com o tenant pedido.
type TenantResolver interface {
	ResolveTenant(ctx context.Context, userID, requested string) (auth.Membership, error)
}

// RequireTenant completa o ator autenticado com o tenant e o papel que o banco
// registra para ele. O cabeçalho só escolhe entre os tenants da pessoa.
func RequireTenant(resolver TenantResolver) func(http.Handler) http.Handler {
	return resolveTenantBy(resolver, tenantFromHeader, writeTenantError)
}

func tenantFromHeader(r *http.Request) string {
	return r.Header.Get(TenantHeader)
}

// resolveTenantBy completa o ator com o vínculo do tenant que requested lê da
// requisição, e responde com writeErr quando o vínculo falta.
func resolveTenantBy(resolver TenantResolver, requested func(*http.Request) string, writeErr func(http.ResponseWriter, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, _ := identity.ActorFrom(r.Context())
			m, err := resolver.ResolveTenant(r.Context(), actor.UserID, requested(r))
			if err != nil {
				writeErr(w, err)
				return
			}
			actor.TenantID, actor.Role = m.TenantID, m.Role
			next.ServeHTTP(w, r.WithContext(identity.WithActor(r.Context(), actor)))
		})
	}
}

func writeTenantError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrMembershipNotFound):
		writeError(w, "tenant_forbidden", "sem acesso a esse tenant", http.StatusForbidden)
	case errors.Is(err, auth.ErrTenantRequired):
		writeError(w, "tenant_required", "informe o tenant em "+TenantHeader, http.StatusBadRequest)
	case errors.Is(err, auth.ErrNoTenant):
		writeError(w, "no_tenant", "usuario sem nenhum tenant", http.StatusForbidden)
	default:
		slog.Error("falha ao resolver tenant", "error", err)
		writeError(w, "internal", "erro interno", http.StatusInternalServerError)
	}
}

// requireWriter barra o viewer, que só lê no tenant.
func requireWriter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, _ := identity.ActorFrom(r.Context())
		if actor.Role == identity.RoleViewer {
			writeError(w, "read_only", "o papel viewer so le as notas", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
