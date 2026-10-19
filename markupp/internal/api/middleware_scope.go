package api

import (
	"net/http"
	"slices"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// requireOAuthScope barra o token OAuth sem o escopo exigido, com 403 e o desafio da
// RFC 6750. Token de login passa, e notas:escrita também satisfaz notas:leitura.
func requireOAuthScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, _ := identity.ActorFrom(r.Context())
			if actor.FromOAuth && !grantsScope(actor.OAuthScopes, scope) {
				w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope", scope="`+scope+`"`)
				writeError(w, "insufficient_scope", "o token nao tem o escopo "+scope, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func grantsScope(granted []string, scope string) bool {
	if scope == oauth.ScopeNotesRead && slices.Contains(granted, oauth.ScopeNotesWrite) {
		return true
	}
	return slices.Contains(granted, scope)
}

// rejectOAuthToken reserva a rota ao token de login: consentimento, autorizações e
// email da conta nunca ficam ao alcance de um agente, qualquer que seja o escopo.
func rejectOAuthToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, _ := identity.ActorFrom(r.Context())
		if actor.FromOAuth {
			writeError(w, "first_party_only", "rota so aceita o token de login", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
